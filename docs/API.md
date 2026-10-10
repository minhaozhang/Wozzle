# Wozzle API 契约（前后端唯一真相源）

后端默认监听 `127.0.0.1:8080`。所有响应 JSON。除 `/api/*` 外的 GET 一律回落到 SPA 的 `index.html`（history 路由）。

## 数据模型（后端已归一化，前端按此消费；除标注必填外均可缺省）

```ts
interface Container {
  id: string;            // 必填，完整 ID
  shortId: string;       // 前 12 位
  name: string;          // 必填
  image: string;
  state: string;         // created|running|exited|paused|restarting|dead|unknown（必填）
  health: string;        // healthy|unhealthy|starting|none
  statusText: string;    // 人类可读状态，如 "Up 5 minutes"
  createdAt: string;     // RFC3339
  startedAt: string;     // RFC3339
  command: string;
  ports: { host: number; container: number; protocol: string }[];
  labels: Record<string, string>;
}

interface LogLine { ts: string | null; text: string }   // text 可含 ANSI 颜色码
interface Stat {
  id: string; cpuPercent: number;
  memBytes: number; memLimitBytes: number; memPercent: number;
  netRx: number; netTx: number; blockRead: number; blockWrite: number;
  pids: number; ts: string;
}
interface Event { type: string; id: string; name: string; image: string; ts: string; raw?: unknown }
interface Image { id: string; repository: string; tag: string; sizeBytes: number; createdAt: string }
```

## REST

| 方法/路径 | 说明 |
|---|---|
| `GET /api/system/info` | `{ wslVersion, wslcVersion, kernel, os, wozzleVersion, running, total }` |
| `GET /api/containers` | `Container[]`（全部，含已退出） |
| `GET /api/containers/{id}` | `{ container: Container, inspect: <wslc inspect 原始 JSON> }` |
| `GET /api/containers/{id}/logs?tail=300&since=&until=` | `{ lines: LogLine[] }` 历史批量（不 follow） |
| `GET /api/images` | `Image[]` |
| `POST /api/containers/{id}/start\|stop\|kill\|restart?confirm=1` | `{ ok: true }`，无 confirm 返回 400 |
| `DELETE /api/containers/{id}?confirm=1&force=0` | `{ ok: true }` |

## WebSocket（均为 JSON 文本帧，`t` 字段区分类型）

### 1. `/api/ws/logs/{id}?tail=300`

```
S→C  { "t":"backfill", "lines":[LogLine...] }     // 连接后先回填（ring buffer + tail）
S→C  { "t":"line", "line":LogLine }               // 实时行
S→C  { "t":"notice", "message":"container restarted" }
S→C  { "t":"end", "reason":"container-exited" }
C→S  { "t":"pause" } / { "t":"resume" } / { "t":"tail", "n":100 } / { "t":"ping" }
S→C  { "t":"pong" }
```

> 已退出的容器：连接后不启动 follow 进程 —— 直接回填历史日志（wslc 会把已退出容器的日志写到
> stderr，服务端已兼容），随即发送 `end`（reason=`container-exited`），并以 1000 正常关闭连接。
> 实时流结束后服务端同样以 1000 关闭（不再异常断开）。

### 2. `/api/ws/stats`（每 2s 一帧，仅在有客户端时轮询）

```
S→C  { "t":"stats", "ts":"...", "stats":[Stat...], "host":HostStat? }
S→C  { "t":"statsError", "error":"..." }
```

> `host` 为可选字段：WSL 宿主机（容器所在的 utility VM，所有 WSL2 发行版共享）实时指标。
> 通过 `wsl.exe`（优先 `docker-desktop` 发行版，回退默认发行版）读取 `/proc/meminfo`、
> `/proc/stat`、`/proc/uptime` 计算得出；读取失败时整帧省略 `host`，不影响容器 stats。
> `cpuPercent` 由相邻两帧的 /proc/stat 差值求得，首帧恒为 0。
> HostStat：`{ cpuPercent, memBytes, memTotalBytes, memPercent, uptimeSeconds }`
> （memBytes = MemTotal − MemAvailable）。

### 3. `/api/ws/events`

```
S→C  { "t":"event", "event":Event }
```

### 4. `/api/ws/exec/{id}?cmd=sh&rows=24&cols=80`（网页终端）

```
C→S  { "t":"input", "data":"ls -la\r" }
C→S  { "t":"resize", "rows":30, "cols":100 }
C→S  { "t":"ping" }
S→C  { "t":"output", "data":"<含 ANSI 的原始终端输出>" }
S→C  { "t":"exit", "code":0 }
S→C  { "t":"error", "message":"..." }
S→C  { "t":"ready" }          // pty 已建立
```

## 前端页面要求

路由（history 模式）：`/`（Dashboard）、`/container/:id/logs`、`/container/:id/inspect`、`/container/:id/terminal`。

- 侧栏：容器卡片（状态点/health/CPU/MEM 迷你条），文本过滤；来自 `GET /api/containers` + events 即时刷新 + stats 更新迷你条。
- 日志页：虚拟化（渲染上限可裁剪）、暂停/继续、时间戳开关、正则+文本过滤、下载 .log、回到底部悬浮键、notice 分隔线。
- Dashboard：每容器 CPU/内存 uPlot 曲线（滚动窗口 ~5min）+ 事件时间线。
- 终端页：xterm.js，断线提示重连。
- 控制操作在侧栏卡片菜单：start/stop/kill/restart/remove，均需确认弹层。
- 暗色主题为主。
