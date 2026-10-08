---
name: wslc-containers
description: Manage and inspect WSL native containers (wslc) — list, logs, stats, exec, lifecycle. Use when the user mentions WSL containers, wslc, Wozzle or its MCP tools.
---

# WSL native containers (wslc)

WSL 3.0+ ships a native container CLI, `wslc.exe` (Docker-like). This skill
covers two ways to work with it: the Wozzle MCP server (preferred) and the
raw CLI (fallback, full of quirks).

## Preferred: Wozzle MCP tools

If the `wozzle` MCP server is connected, always prefer its tools — they
return clean JSON with sizes already parsed to bytes, IDs normalized and
errors surfaced properly:

| Tool | Purpose |
|---|---|
| `containers_list` | all containers: id, name, image, state, health, ports |
| `container_inspect(id)` | full raw inspect JSON |
| `container_logs(id, tail?, since?)` | last N log lines with timestamps |
| `container_stats(id?)` | CPU %, mem used/limit, net/block IO, PIDs |
| `exec_run(id, command, timeout_seconds?)` | one-shot `sh -c` inside container, returns output + exit code |
| `container_action(id, start\|stop\|kill\|restart)` | lifecycle control |
| `images_list` / `system_info` | local images, WSL/runtime versions |

Container references accept names (preferred), full ids, or id prefixes
(≥ 2 chars).

## Fallback: raw wslc CLI quirks

Without the MCP server, these quirks matter:

- Set `WSL_UTF8=1` for stable output encoding.
- Use `--format json` wherever available; output is JSONL (one object per line).
- **ID mismatch**: `wslc list` reports 12-char ids, `wslc stats` reports
  64-char ids — join the two by container **name**, not id.
- **Formatted strings**: stats fields come as strings like
  `"3.543MiB / 15.32GiB"` (mem), `"0.06%"` (cpu) — parse yourself.
- `wslc events` prints plain text lines (e.g. `container start ...`), not JSON.
- `wslc exec -i <id> sh -c '<cmd>'` for one-shot runs (no `-t` without a TTY);
  interactive sessions cannot be resized mid-way.
- `wslc stop` waits ~10 s before SIGKILL — don't treat the pause as a hang.
- Docker Hub is often unreachable; pull from mirrors, e.g.
  `wslc pull docker.m.daocloud.io/library/alpine`.

## Wozzle web UI / REST API

If Wozzle is running locally, the same data is available at
`http://127.0.0.1:8080` (REST under `/api/...`, contract in `docs/API.md`)
and a browser dashboard for humans.
