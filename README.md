# process_handler

Windows 终端 TUI 工具，查找并安全终止占用 TCP 监听端口的进程。

## 功能特性

- 扫描所有 IPv4 TCP 监听端口及对应的进程信息（名称、路径、命令行、父进程、用户）
- 按进程关键程度分级保护，防止误杀系统关键进程
- 交互式 TUI：浏览、筛选、确认终止、帮助面板
- 自动定时刷新端口状态

## 前置要求

- **仅限 Windows**（依赖 `iphlpapi.dll`、WMI、`OpenProcess` 等 Windows API）
- Go 1.25+

## 安装与构建

```bash
go build -o process_handler.exe .
```

或直接运行：

```bash
go run .
```

## 使用方式

启动后进入浏览模式，默认每 3 秒刷新一次端口列表。

### 键盘操作

| 按键 | 说明 |
|------|------|
| `↑` `↓` / `j` `k` | 上下移动光标 |
| `Home` / `End` / `g` / `G` | 跳到首行/末行 |
| `PgUp` / `PgDn` | 翻页 |
| `Enter` / `Space` | 确认终止当前选中进程 |
| `/` | 进入筛选模式（按 PID/名称/端口/路径/命令行筛选） |
| `Esc` | 退出筛选模式 / 取消确认 |
| `?` | 显示帮助面板 |
| `r` | 切换自动刷新（开/关） |
| `q` / `Ctrl+C` | 退出程序 |
| `y` / `n` | 确认/取消终止进程 |

### 筛选模式

输入关键词实时筛选，支持匹配 PID、进程名、端口、可执行文件路径、命令行。大小写不敏感。按 `Esc` 退出筛选。

## 安全分级机制

为防止误杀，每个进程按关键程度分为三级：

| 级别 | 颜色 | 说明 | 终止行为 |
|------|------|------|----------|
| **CritBlocked**（阻止） | 红色 | System、smss.exe、csrss.exe、lsass.exe 等系统核心进程 | **硬阻止**，无法进入确认流程 |
| **CritWarn**（警告） | 黄色 | SYSTEM 用户进程、svchost.exe、explorer.exe 等 | 需输入完整 PID 号才能确认终止 |
| **CritNone**（安全） | 默认 | 普通用户进程 | 直接确认即可终止 |

## 项目结构

```
process_handler/
├── main.go                      # 入口，启动 TUI
├── internal/
│   ├── portscan/                # 数据采集层
│   │   ├── collect.go           # Collect() 并发编排
│   │   ├── ports.go             # GetExtendedTcpTable 系统调用
│   │   ├── process.go           # WMI 查询 + 用户名获取
│   │   └── join.go              # 端口 x 进程按 PID 关联
│   ├── procinfo/                # 数据模型
│   │   ├── types.go             # PortEntry / ProcessInfo / Row / CriticalLevel
│   │   └── critical.go          # Classify() 安全分级
│   ├── kill/                    # 进程终止
│   │   └── kill.go              # Terminate() 通过 OpenProcess + TerminateProcess
│   └── tui/                     # Bubble Tea 界面层
│       ├── model.go             # Model / Init / collectCmd
│       ├── update.go            # Update() 状态机
│       ├── view.go              # View() 布局组装
│       ├── table.go             # 进程表格渲染
│       ├── detail.go            # 详情面板渲染
│       ├── keys.go              # 帮助文本
│       └── styles.go            # lipgloss 主题样式
└── docs/                        # 设计文档和实现计划
```

## 开发命令

```bash
go build ./...    # 构建所有包
go vet ./...      # 静态检查
go test ./...     # 运行所有测试（需要 Windows 环境）
```

测试分为两类：`procinfo` 包为纯函数表驱动测试，其余为 Windows 集成测试（会实际打开端口、启动进程并终止）。
