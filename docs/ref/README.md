# 官方参考文档（来自 microsoft/mxc）

以下文档下载自 [microsoft/mxc](https://github.com/microsoft/mxc) 仓库 `main` 分支，
路径为 `docs/backends/wslc/` 及 `docs/development/`（2026-10 仓库重组后的新路径）。
抓取日期：2026-10-09。上游更新时需手动重新下载。

| 文件 | 原路径 | 说明 |
|---|---|---|
| [wsl-container-getting-started.md](wsl-container-getting-started.md) | docs/backends/wslc/ | WSLC 后端入门：环境要求（WSL 2.9.9+、wslcsdk.dll）、安装、镜像来源、自定义注册表/tar 导入、超时配置示例 |
| [wslc-state-aware.md](wslc-state-aware.md) | docs/backends/wslc/ | WSLC 有状态感知：容器生命周期/状态机、SDK daemon 协议细节 |
| [wslc-registry-allowlist-policy.md](wslc-registry-allowlist-policy.md) | docs/backends/wslc/ | 注册表白名单策略（策略驱动隔离的一部分） |
| [wslc-sdk-bindings.md](wslc-sdk-bindings.md) | docs/development/build-and-test/ | WSLC SDK FFI 绑定（bindgen 生成 wslcsdk_sys.rs）维护手册 |
| [linux-wsl-roadmap-june-2026.md](linux-wsl-roadmap-june-2026.md) | docs/development/plans/ | 2026-06 Linux/WSL 路线图：wslc 后续能力规划 |

对 Wozzle 的参考价值：
- **wslcsdk.dll / SDK daemon 协议**：除 `wslc.exe` CLI 外还存在官方客户端 SDK，
  可直连 WSL 服务获取容器列表/状态/事件，是未来替代 CLI 轮询的候选方案
  （对应架构里的 Provider 抽象层）。
- **WslcGetMissingComponents()**：官方的"环境可用性检测"入口，可用于 Wozzle
  启动时的环境自检与引导提示（wsl --update --pre-release 需 ≥2.9.9）。
- **状态机/事件模型**：wslc-state-aware.md 中的生命周期定义可对照 Wozzle 的
  events 解析逻辑。

## 实测补充（2026-10-10，两台机器交叉验证）

wslc bridge 模式的真实网络姿态：**仅公网出站可用**。

| 容器 → 目标 | 结果 |
|---|---|
| 公网 IP / 域名 | ✓（会话 VM NAT 出站） |
| Windows 宿主服务（LAN IP 或 172.17.0.1 的 TCP） | ✗ 快速 RST——会话 VM 自己拒绝，包未到 Windows（ICMP ping 宿主能通，是假阳性） |
| 其他局域网机器 | ✗ no route / unreachable |

结论：容器需要访问的依赖（backend/Redis 等）**不能放在 Windows 宿主或其他局域网机器上**，
要么与使用方跑在同一个 wslc bridge 网络里（容器间互通），要么等官方开放 VM 级网络能力
（路线图 SDK dep #1）。与 wslc-state-aware.md "does not promise reachability across NAT"
的说法一致。
