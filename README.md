# Wozzle

> 像 [Dozzle](https://github.com/amir20/dozzle) 监控 Docker 那样，用浏览器实时监控 **WSL 3.0 原生容器（wslc）**：日志流、资源曲线、容器事件、网页终端，一个 exe 全搞定。

## 功能

- **实时日志**：`wslc logs -f` 流式桥接到浏览器；时间戳、正则/文本过滤、暂停/继续、下载、断线自动重连、容器重启自动续流
- **资源监控**：CPU / 内存 / 网络 / 块 IO / PIDs，2 秒刷新，Dashboard 滚动曲线
- **事件流**：`wslc events` 实时推送，容器列表即时刷新
- **网页终端**：xterm.js 直连 `wslc exec -i -t`
- **控制操作**：start / stop / kill / restart / remove（带确认）
- **单文件分发**：前端 `embed` 进 Go 二进制，`wozzle.exe` 拷走即用

## 环境要求

- Windows 10/11 + **WSL 3.0+**（`wsl --version` ≥ 2.9.3，容器功能 GA 版更好）
- `wslc.exe` 在 PATH（`C:\Program Files\WSL\wslc.exe`，随 WSL 附带）
- 已有容器运行（`wslc run ...`），镜像拉取建议配置可达的 registry（如 `docker.m.daocloud.io/library/alpine`）

## 构建与运行

```powershell
# 后端（Go 1.26+）
go build -o wozzle.exe .

# 运行（默认 http://127.0.0.1:8080）
.\wozzle.exe
.\wozzle.exe -addr 127.0.0.1:9090   # 换端口
.\wozzle.exe -wslc "C:\Program Files\WSL\wslc.exe"  # 显式指定 wslc
```

前端单独构建（改 UI 后需要，产物嵌入 exe）：

```powershell
cd web
pnpm install
pnpm build
cd ..
go build -o wozzle.exe .
```

开发模式：终端 1 跑 `go run .`，终端 2 跑 `cd web; pnpm dev`（Vite 代理 `/api` → `127.0.0.1:8080`）。

## 架构

```
浏览器 (Vue3 + xterm.js + uPlot)
   │ HTTP / WebSocket（docs/API.md 契约）
wozzle.exe (Go, 单二进制, 前端 embed)
   │ 子进程 + JSON/流式 stdout
wslc.exe  ── list/stats/logs -f/events/exec -i -t
   │
WSL 容器引擎（会话管理器）
```

- `internal/wslc`：wslc CLI 封装与 JSONL 解析（schema 样本见 `docs/schema/`）
- `internal/store`：容器清单（2s 轮询 + 事件驱动即时刷新）
- `internal/server`：REST + 4 类 WebSocket、日志流监控器（ring buffer 回填、断流重连）
- `web/`：Vue3 前端
- `tools/wssmoke`：WebSocket 端到端冒烟测试（`go run ./tools/wssmoke`，需 `wozzle-test` 容器在跑）

## 已知限制

- exec 终端**不支持会话中途 resize**（wslc CLI 限制，`resize` 消息被忽略；初始行列通过 COLUMNS/LINES 环境变量传递）
- stats 为 2 秒轮询快照（wslc stats 无流式模式）
- `list` 中 Labels 字段是嵌套字符串，暂未解析（UI 未用到）
- 默认绑定 127.0.0.1，无认证（本机工具定位；不要暴露到公网）

## 设计文档

- [docs/PLAN.md](docs/PLAN.md) — 总体方案、选型论证（Go vs Rust vs C++）、里程碑
- [docs/API.md](docs/API.md) — REST / WebSocket 契约（前后端唯一真相源）
- [docs/schema/](docs/schema/) — wslc 各命令原始 JSON 输出样本
