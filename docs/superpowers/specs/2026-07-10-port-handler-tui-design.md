# Port Handler TUI — 设计文档

- **日期**: 2026-07-10
- **状态**: 已确认
- **平台**: 仅 Windows
- **技术栈**: Go 1.25 + Bubble Tea (`bubbletea`/`bubbles`/`lipgloss`)

## 1. 目标

本地开发时，常有进程在后台监听端口却不被察觉，导致端口被占用、新服务起不来，甚至误杀关键进程。本工具提供一个终端 TUI，用于：

1. 查看当前所有监听 TCP 端口及其占用进程。
2. 展示足够的进程身份信息（路径、命令行、父进程、用户名），让使用者判断"这是不是自己起的开发服务"。
3. 直接在 TUI 内终止进程，并对关键系统进程做分级保护，避免误杀。

## 2. 范围

- **仅 Windows** 平台。
- **仅监听状态的 TCP 端口**（IPv4）。IPv6、UDP、非监听连接不在本期范围。
- 单进程终止（非批量）。
- 非后台守护，交互式 TUI 一次会话使用。

## 3. 技术方案

**方案 A（已选）**：Windows 原生 API 查端口映射 + WMI 查进程详情。

- 端口映射：`GetExtendedTcpTable`（`TCP_TABLE_OWNER_PID_LISTENER`）→ 监听端口 + 占用 PID，原生可靠，无文本解析。
- 进程详情：WQL 查询 `Win32_Process` 一次拿到 `ProcessId / Name / ExecutablePath / CommandLine / ParentProcessId`。
- 用户名：`OpenProcess + GetTokenInformation(TokenUser) + LookupAccountSid`，纯 API，绕开 WMI 方法调用的兼容坑。
- 两者在内存按 PID join。

权衡：WMI 查全量进程约几百毫秒，但只查一次建索引，配合原生 API 的端口表，单次采集 < 1s，满足 3s 自动刷新。

## 4. 包结构

```
process_handler/
  main.go              # 入口，启动 Bubble Tea
  internal/
    portscan/          # 数据采集层
      ports.go         # GetExtendedTcpTable → 监听端口+PID
      process.go       # WMI Win32_Process → 进程详情；OpenProcess+Token → 用户名
      join.go          # 端口×进程 join → []Row
      collect.go       # Collect() 唯一对外入口，并发跑两路
    procinfo/          # 数据结构与判定
      types.go         # PortEntry, ProcessInfo, Row
      critical.go      # 关键进程识别（名单/用户名判定）
    kill/              # 终止进程
      kill.go          # OpenProcess + TerminateProcess
    tui/               # Bubble Tea 界面
      model.go         # 主 Model
      update.go        # Update 逻辑
      view.go          # View 渲染与布局
      table.go         # 表格区渲染
      detail.go        # 详情面板渲染
      keys.go          # 按键绑定
      styles.go        # lipgloss 样式
```

设计原则：采集逻辑全部封装在 `portscan`，TUI 只消费 `[]procinfo.Row`。各包可独立测试。

## 5. 数据模型

```go
// procinfo/types.go

type PortEntry struct {
    LocalAddr net.IP
    LocalPort uint16
    PID       uint32
}

type ProcessInfo struct {
    PID        uint32
    Name       string
    ExePath    string
    CmdLine    string
    ParentPID  uint32
    ParentName string
    Username   string   // "DOMAIN\user" 形式
}

// Row 不直接嵌入两者（PortEntry 与 ProcessInfo 都有 PID 字段，会冲突）。
// PID 统一取自 PortEntry；Proc.PID 与之一致（join 保证）。
type Row struct {
    PortEntry              // 提供 LocalAddr / LocalPort / PID
    Proc      ProcessInfo  // 命名字段，避免 PID 歧义
    Critical  CriticalLevel  // None / Warn / Blocked
}

type CriticalLevel int
const (
    CritNone    CriticalLevel = iota
    CritWarn                      // 强警告，可 kill
    CritBlocked                   // 硬拒绝，不可 kill
)
```

## 6. 数据采集细节

### 6.1 端口映射 (`ports.go`)

- `GetExtendedTcpTable` 传 `TCP_TABLE_OWNER_PID_LISTENER`，返回 `MIB_TCPROW_OWNER_PID` 数组。
- 每行含 `dwLocalAddr`、`dwLocalPort`（网络字节序）、`dwOwningPid`。
- 输出 `[]PortEntry`。一个 PID 可对应多个端口（多个监听）。
- 预留扩展位：未来 IPv6/UDP 在本文件内加函数，不改外部接口。

### 6.2 进程详情 (`process.go`)

WQL 查询：
```sql
SELECT ProcessId, Name, ExecutablePath, CommandLine, ParentProcessId
FROM Win32_Process
```
- 用 `github.com/yusufpapurcu/wmi`（轻量纯 Go，无 cgo）跑 WQL。
- 结果建 `map[uint32]ProcessInfo`。
- `ParentName`：从同一 map 按 `ParentProcessId` 查（找不到则留空）。
- `ExecutablePath` / `CommandLine` 可能为空（系统进程无路径），留空字符串。

### 6.3 用户名 (`process.go`)

纯 API 路径：
```
OpenProcess(PROCESS_QUERY_INFORMATION, pid)
  → GetTokenInformation(TokenUser) → TOKEN_USER.User.Sid
  → LookupAccountSid → "DOMAIN\user"
```
- 失败（无权限/进程已退出）→ `Username = ""`，不崩。

### 6.4 join (`join.go`)

- 端口表按 PID 与进程 map join。
- PID 在进程表缺失（已退出）→ `ProcessInfo` 置零值，`Name = "Unknown"`。
- 输出 `[]Row`，每端口一行。

### 6.5 并发采集 (`collect.go`)

```go
func Collect() ([]procinfo.Row, error)
```
内部并发跑端口查询和 WMI 进程查询，再 join，返回 `[]Row`。TUI 通过 `tea.Cmd` 异步调用，结果以 message 回主循环，界面不卡顿。

### 6.6 关键进程识别 (`procinfo/critical.go`)

- **硬拒绝名单**（`CritBlocked`，不可 kill）：`System`(PID 4)、`Idle`(PID 0)、`Registry`、`smss`、`csrss`、`wininit`、`services`、`lsass`、`winlogon`。
- **强警告**（`CritWarn`，可 kill 但强警示）：用户名为 `SYSTEM`/`NETWORK SERVICE`/`LOCAL SERVICE` 的进程，或名为 `svchost`、`dwm`、`explorer`。
- 其余 `CritNone`。
- 判定为纯函数，表驱动测试。

## 7. TUI 设计

### 7.1 布局（主从详情 master-detail）

```
┌─ Process Handler ──────────────────────────────────────────────┐
│ PID   NAME              PORT    PARENT          USER           │ 表头
│ ──────────────────────────────────────────────────────────── │
│ 4521  node.exe         :3000   8821 (code)      admin          │ 选中行高亮
│ 7833  python.exe       :8000   8821 (code)      admin          │
│ 4     System           :135    —                SYSTEM      ⚠  │ 关键进程标记
│ ...                                                              │
├─ Detail ───────────────────────────────────────────────────────┤
│ PID:        4521                                                │
│ Name:       node.exe                                            │
│ Port:       127.0.0.1:3000                                      │
│ ExePath:    D:\nodejs\node.exe                                  │
│ CmdLine:    "D:\nodejs\node.exe" server.js --port 3000          │
│ Parent:     8821 (Code.exe)                                     │
│ User:       DESKTOP\admin                                       │
│ Critical:   no                                                  │
├────────────────────────────────────────────────────────────────┤
│ r refresh  R auto-refresh  /filter  enter kill  q quit  ↑↓ move│ 帮助栏
└────────────────────────────────────────────────────────────────┘
```

- **表格区（上）**：列 `PID | NAME | PORT | PARENT(PID+名) | USER` + 关键进程 `⚠` 标记。长名字截断，宽度随终端自适应。
- **详情区（下）**：选中行完整信息，`ExePath`/`CmdLine` 不截断，超宽换行。
- **帮助栏（底）**：常驻显示主按键；进入过滤/确认等模式时替换为该模式提示。
- 终端过窄（< 60 列）显示提示放大窗口；高度自适应分配表格区与详情区。

### 7.2 状态机

| 状态 | 说明 | 可用按键 |
|------|------|----------|
| `browse` | 默认浏览 | `↑↓`/`j k` 移动、`r` 刷新、`R` 切自动刷新、`/` 过滤、`Enter`/`k` kill 确认、`q` 退出、`?` 全屏帮助 |
| `filter` | 输入过滤 | 输入关键字即时匹配 PID/端口/进程名/路径/命令行，`Esc` 返回 browse |
| `confirm` | kill 确认弹窗 | 普通进程：`y` 执行、`n`/`Esc` 取消；`CritWarn` 需先键入完整 PID 再 `y` |
| `killing` | 执行中 | 自动返回，显示结果 toast |

### 7.3 消息

- `collectResultMsg{rows, err}`：采集完成。
- `tickMsg`：自动刷新定时器触发采集。
- `killResultMsg{pid, err}`：终止完成。

## 8. Kill 与关键进程保护

### 8.1 终止实现 (`kill/kill.go`)

```go
func Terminate(pid uint32) error
```
`OpenProcess(PROCESS_TERMINATE, false, pid)` → `TerminateProcess(handle, 1)`。纯 API，无子进程。同用户进程无需提权；系统/他人进程返回 `ERROR_ACCESS_DENIED`——天然保护。

### 8.2 kill 流程

1. `browse` 选中行按 `Enter`/`k` → `confirm`。
2. 确认弹窗：
   - 正文：`将终止 PID 4521 (node.exe)，持有端口 127.0.0.1:3000`
   - `CritWarn`：追加红底警示 `⚠ 关键系统进程！终止可能导致系统不稳定`，并进入 PID 输入子模式——必须键入完整 PID 数字才能确认（防误操作），`Esc` 取消。
   - `CritBlocked`：**直接拒绝**，弹窗仅显示"该进程不可终止"，无 `y` 选项。
3. `y` → `killing`，调 `kill.Terminate(pid)`。
4. 结果：成功 → toast `已终止 PID 4521` + 触发刷新；失败 → toast 显示 Windows 错误信息（拒绝访问/进程已不存在等）。
5. `n`/`Esc` → 回 `browse`。

## 9. 错误处理与边界

- `Collect()` 返回 `(rows, error)`；降级策略：端口查询失败 → 错误条 + 空列表；WMI 失败 → 仍显示端口+PID，进程字段标 `unknown`（用户名走纯 API 不受影响）。
- kill 时进程已退出（`ERROR_INVALID_PARAMETER`/不存在）→ toast"进程已不存在"，刷新后消失。
- 权限不足（`ERROR_ACCESS_DENIED`）→ toast"拒绝访问（权限不足或系统进程）"。
- 终端过窄 → 提示放大窗口。
- 同 PID 多端口 → 多行展示。
- 未知 PID（端口表有、进程表无）→ 标 `Unknown` 不崩。
- 过滤为空 → 显示全部。
- 自动刷新开启时仍可过滤/kill，kill 后立即刷新。

## 10. 依赖

- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`
- `golang.org/x/sys/windows`
- `github.com/yusufpapurcu/wmi`

## 11. 测试策略

- **`procinfo.Critical`**：纯函数，表驱动测试覆盖三类名单与用户名判定。
- **`portscan` 集成测试**：起 `net.Listen("127.0.0.1:0")` 的 goroutine，断言 `Collect()` 结果里能按端口找到该 PID；kill 后断言该行消失。
- **`kill`**：起 dummy 进程（`exec.Command("ping","-t","127.0.0.1")`），测能终止；对 PID 4 测返回拒绝。
- **`tui` Update**：纯函数测试——构造 model、发 message、断言状态变迁，不渲染真终端。

## 12. 非目标 / 未来扩展

- IPv6 / UDP / 非监听连接。
- 跨平台（Linux/macOS）。
- 批量多选 kill。
- 优雅终止（先 `Ctrl+C` 再强杀）——本期直接 `TerminateProcess`。
- 持久化配置（默认刷新间隔等）。
