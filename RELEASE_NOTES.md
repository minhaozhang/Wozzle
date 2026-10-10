# Wozzle v0.2.2

Dozzle-style monitoring for **WSL 3.0 native containers** (`wslc`), in a single binary. 本版修复 exec 断连、托盘菜单与图标问题，UI 升级为 Dozzle 风格仪表盘。

## What's New / 修复与改进

- **FIX: exec 断连** — `wslc exec -i` 在无真实控制台的调用方（服务/隐藏窗口进程）下，超过 ~1 秒即以 `ERROR_INVALID_HANDLE` 崩溃并卡死容器 exec 通道。一次性执行（含 MCP `exec_run`）不再传 `-i`，长命令稳定；交互式网页终端保留 `-i -t`（有真实 stdin 管道）
  - Fix: `wslc exec -i` died with `ERROR_INVALID_HANDLE` after ~1s when the caller had no real console, wedging the container's exec channel. One-shot exec (incl. MCP `exec_run`) no longer passes `-i`
- **FIX: 托盘右键菜单不显示** — systray 改为主线程 Register + 窗口间消息泵，菜单稳定弹出
- **FIX: exe / 任务栏图标缺失** — 内嵌图标资源（syso）+ WM_SETICON
- **NEW: Dozzle 风格仪表盘** — 表格视图 + 侧边导航，信息密度更高
- **NEW: WSL 宿主指标** — 宿主 CPU / 内存 / 网络一并展示（`internal/wslc/host.go`）
- **FIX: 已退出容器的日志** — 读取 wslc stderr 日志，WebSocket 以 1000 正常关闭
- **Docs: `docs/ref/`** — 收录 microsoft/mxc 官方 wslc 参考文档（入门 / 状态机 / 注册表策略 / SDK 绑定 / 路线图），附实测网络姿态结论：bridge 模式仅公网出站，容器→宿主/局域网 TCP 不通（会话 VM 拦截）

## Requirements

- Windows 10/11 with WSL 3.0+ (`wslc.exe` on PATH)
- WebView2 runtime (preinstalled on Windows 10/11)

## Downloads

- `wozzle.exe` — 桌面版（托盘 + 窗口，无控制台）
- `wozzle-console.exe` — 控制台 / 服务版
- `wozzle-mcp.exe` — MCP server（给 AI agent 用，stdio）
