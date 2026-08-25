# 本机环境盘点

状态：2026-08-25 Phase 1 项目本地工具链快照；可能随机器状态变化
主机：macOS 26.6.2（Build 25G83），Apple Silicon `arm64`

| 类别 | 当前观察 | Phase 0 必需 | 用途/影响 | 后续建议 |
| --- | --- | --- | --- | --- |
| Git | `2.50.1 (Apple Git-155)` | 是，已有 | 本地版本控制 | 无需安装 |
| Godot | 项目本地 `.tools/godot/4.7.2` 已安装 `4.7.2.stable.official.ed1daf0bf` 标准版和匹配模板 | 否 | Phase 1 Godot adapter/headless/export 的核心外部依赖 | 官方包散列、版本、Notarized Developer ID、headless 与技术导出已实测；macOS `user://` 仍尝试用户 Application Support，隔离验收 BLOCKED |
| Node.js | `v22.23.1` | 否 | 仅可能用于 npm 包验证/发布工具；不是已冻结 Go CLI 的产品运行时 | 保留现状；不得把 Node 变成普通用户使用 CLI 的前置条件 |
| npm | `11.18.0` | 否 | 包名检查、未来 Trusted Publishing | 已高于 npm Trusted Publishing 文档要求的 11.5.1；Phase 0 不登录/发布 |
| Codex CLI | `0.149.0-alpha.4.1`；执行时提示无法创建 PATH aliases（权限限制） | 是，已有 | 本项目 Codex 原生开发宿主 | Phase 1 验证 Plugin/Agents/Skills 与当前稳定客户端兼容；该 alpha 版本不可直接作为 v1 支持承诺 |
| GitHub Connector | 已认证 `Leon0555`，GitHub App installation 存在 | 否 | 可在已授权仓库范围内读写 GitHub | 当前未创建项目远程；权限按 App installation/仓库控制 |
| GitHub CLI (`gh`) | `2.97.0`；`Leon0555` 的本地 token 已失效 | 否 | 未来终端创建 remote、push 或发布 | 用户确认暂不处理；首次终端 push 前提醒并显式重新登录 |
| Python | PATH 默认 `/usr/bin/python3` 为 `3.9.6`；Codex bundled runtime 另含 `3.12.13` | 否 | 可用于辅助验证，不作为已决定运行时；bundled runtime 不等于用户全局 Python | 无需安装，也不把 Codex 私有 runtime 当作产品依赖 |
| GNU Make | `3.81` | 否 | 可选构建编排 | 无需安装 |
| Apple Clang | `21.0.0` | 否 | 可辅助 macOS 产物、链接和签名验证；当前 Go CLI 样品使用 `CGO_ENABLED=0` | 已有；不作为普通用户前置条件 |
| Xcode Command Line Tools | `/Library/Developer/CommandLineTools` | 否 | macOS 原生构建/签名工具基础 | 已有；完整 Xcode/签名账号未检查也不登录 |
| CMake | 未发现 | 否 | 不属于当前 Go v1 CLI 路径；仅在未来明确依赖需要时使用 | 不安装；如未来确有需要，先列明用途、体积和影响并请求批准 |
| Rust / Cargo | `.tools` 内 `rustc 1.98.0`、`cargo 1.98.0` arm64；另含 rustfmt、clippy、rust-std 与离线 docs | 否 | 非生产对照与历史研究证据 | 官方清单校验、fmt/build/test/clippy 与结构化 Spike 已实测；不进入 v1 生产路径；Rust 目录约 1.3 GiB |
| Go | `.tools/go/1.27.0`，`go1.27.0 darwin/arm64` | 否 | v1 CLI 生产实现工具链 | 官方 SHA-256 通过；首批 `detect`/`doctor`/`status` 已在正式包路径实现，完整 CLI 与目标宿主验证仍未完成 |
| Schema/Plugin validators | `.tools/python-validators`：PyYAML 6.0.3、jsonschema 4.25.1 及固定传递依赖 | 否 | Draft 2020-12 fixtures 与官方 Plugin/Skill validator | 仅项目本地约 2.9 MiB；不属于产品运行时，不修改系统 Python |
| Docker / Podman | 均未发现 | 否 | 可选的 Linux 构建/干净环境工具，不是 v1 普通用户依赖 | 先评估 hosted runner/原生构建，不因 Phase 0 安装容器运行时 |
| 磁盘 | 工作盘仍有约 452 GiB；仓库 `.tools` 当前约 6.2 GiB | 是，充足 | Godot/templates、已校验下载包、Go/Rust 工具链、缓存和 artifacts | 原 5 GiB 是预估警戒线；用户已允许按真实需要扩展，当前 8 GiB 仅作异常增长软告警 |

## 尚未执行或未通过

- 未检查 Windows/Linux 实机或 runner。
- 未登录 npm/GitHub，未创建 token、组织、远程仓库或 package。
- Rust 与 Go 只在 macOS arm64 实际运行；Go 的三个 x64 交叉产物尚未在目标宿主执行，Rust x64 targets 未安装/未构建。
- 未安装 CMake 或容器运行时。
- Godot clean headless 仍因 macOS 标准 `user://` 外部写入尝试而 BLOCKED。

## 体积与影响说明

截至 2026-08-25：Godot macOS Universal 标准版为 170,622,178 bytes，匹配 export templates 为 1,281,349,702 bytes；Go 1.27.0 archive 为 68,303,667 bytes。Rust 原计划的 minimal 三组件压缩传输约 99,090,420 bytes，但中断状态恢复时实际补装了 rustfmt、clippy 与离线 docs；最终 Rust 目录约 1.3 GiB。当前 `.tools` 约 6.2 GiB，其中 Godot 2.3 GiB、Rust 1.3 GiB、已校验下载包 1.4 GiB、Godot 解压核验目录约 337 MiB、Go 271 MiB、构建/导出缓存与产物约 630 MiB。压缩传输量与解压安装占用明确分列。
