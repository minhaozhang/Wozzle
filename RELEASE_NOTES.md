# Wozzle v0.2.1

Dozzle-style monitoring for **WSL 3.0 native containers** (`wslc`), in a single binary. 新增 MCP server，让各类 AI agent（Claude / opencode / Cursor…）直接管理 WSL 容器。

## Highlights

- **Live log streaming** — tail/backfill, regex & text filtering, pause/resume, download, auto-reconnect across container restarts
- **Resource monitoring** — CPU / memory / network / block I/O / PIDs with rolling charts (2 s refresh)
- **Event-driven** container list refresh
- **Web terminal** — xterm.js wired to `wslc exec`
- **Container control** — start / stop / kill / restart / remove, behind confirmation dialogs
- **Desktop mode** — system tray + WebView2 window, boot autostart, `wozzle.json` config, headless server mode
- **NEW: `wozzle-mcp.exe`** — MCP server (8 tools: containers_list / container_inspect / container_logs / container_stats / exec_run / container_action / images_list / system_info)，stdio 传输，兼容 Claude Desktop、Claude Code、opencode 等

## Requirements

- Windows 10/11 with WSL 3.0+ (`wslc.exe` on PATH)
- WebView2 runtime (preinstalled on Windows 10/11)

## Downloads

- `wozzle.exe` — 桌面版（托盘 + 窗口，无控制台）
- `wozzle-console.exe` — 控制台 / 服务版
- `wozzle-mcp.exe` — MCP server（给 AI agent 用，stdio）
