# M3 全新 Starter 候选端到端复验

- 日期：2026-08-31
- 候选源码提交：`de3f7420d346222a515b3ffc07b9a76b01807fe2`
- 候选：`.tools/distributions/codex-game-atelier-0.2.0-m3-strict-de3f742-a/`
- 执行入口：候选 Plugin 内的 macOS Universal 2 public CLI
- 项目副本：从候选 Starter archive 新解包到中文、空格与 `#` 路径
- 宿主：macOS Apple Silicon
- Godot：`4.7.2.stable.official.ed1daf0bf` standard/GDScript
- 结论：当前宿主上的 fresh Starter 初始化、Headless、6 项 GDScript 测试、Debug/Release build、直接 Release export、Apple Silicon target smoke 与 standard/strict 聚合均按契约完成；strict 只剩 required hosted CI 为 `NOT_RUN`

本记录证明的是当前用户与当前机器上的全新项目副本，不是全新 macOS 用户、全新机器或首次下载后的 Gatekeeper 环境。没有修改用户级 Codex、没有安装 Plugin、没有登录、remote、push、升级、回滚或外部发布。

## 1. 输入与边界

候选先由 distribution verifier 复验；Starter archive 摘要为：

`14837f802739c4d705bce625eecfdf132681ddd14616bcca52810a120afd8d3e`

解包后的项目最初没有 `.gameatelier/`。候选 CLI 完成 `initialize` 后，所有 Atelier 状态、run evidence 与导出 artifact 都留在该项目副本的 `.gameatelier/`；Godot 获准使用标准 `user://`。Plugin candidate、Starter archive 与源码仓库没有被命令改写。

## 2. 端到端结果

| 步骤 | 结果 | 关键证据 |
| --- | --- | --- |
| Starter 解包与 `initialize` | PASS | 新建 schema `1.0.0`、revision `0`、standard/GDScript project state |
| Headless `validate` | PASS | run `atelier-20260831t094212.515864000z-77c33e3b4d76`；8/8 checks |
| 固定 GDScript `test` | PASS | run `atelier-20260831t094325.125217000z-f55eb5f3250f`；6/6 tests |
| `build --profile debug` | PASS | run `atelier-20260831t094358.169617000z-e919891936fc`；Universal 2 + arm64 smoke |
| `build --profile release` | PASS | run `atelier-20260831t094513.539215000z-02a1e38d4b93`；Universal 2 + arm64 smoke |
| 直接 `export --profile release` | PASS | run `atelier-20260831t094725.997254000z-d92a2e3bb06b`；Universal 2 + arm64 smoke |
| standard `release check` | PASS/0 | 6 PASS、0 BLOCKED、0 NOT_RUN；`release_ready=false`，未冒充 strict |
| strict candidate `release check` | BLOCKED/4（按设计） | 10 PASS、0 BLOCKED、1 NOT_RUN；仅 `required-ci` 未运行，`release_ready=false` |

`build --profile release` 与直接 `export --profile release` 都使用同一底层 Godot 导出链，但 release check 有意要求直接 Release export evidence，因此两步分别执行，没有把 build 记录冒充 export 记录。

## 3. 产物

| 产物 | SHA-256 | 大小 | 约束 |
| --- | --- | --- | --- |
| Debug build ZIP | `8f7c537c885d0bc0c7caafdc287d33e012ed721b7f24a4dd4169a1572a80d632` | 约 61 MiB | Universal 2；Apple Silicon headless one-frame smoke PASS |
| Release build ZIP | `ead33dc87f008bb970fcdab0f83b1a29fbb6e2a29cf9c9676b5544d994f365fe` | 约 57 MiB | Universal 2；Apple Silicon headless one-frame smoke PASS |
| Direct Release export ZIP | `a57467c6865a9f26075218a44200c794901d8f1a343750ec6bd367e65cfa4436` | `59,656,747` bytes | unsigned、not notarized、public_distribution_ready=false；arm64 smoke PASS |

Universal 2 只表示产物含 `amd64 + arm64` slices；本轮只实际启动 Apple Silicon slice，不声称 Intel 实机通过。签名和公证不属于 v1.0 Godot 游戏技术导出门禁。

## 4. 受限沙箱负例

第一次 Headless run `atelier-20260831t040104.531268000z-d18a124a13f4` 在 Codex 受限文件沙箱内返回 `FAIL/5`、`GODOT_REPORTED_ERRORS`。复现表明，CLI 的临时 ad-hoc Godot 快照在该沙箱内不能写已声明的标准 macOS `user://` 日志目录；使用同一候选、项目、Godot 和显式 `--allow-engine-user-data`，在允许该标准目录的普通宿主权限下立即 PASS。

这一现象与既有 [`phase1-godot-headless-2026-08-25.md`](phase1-godot-headless-2026-08-25.md) 记录一致，不是新发现的 Starter 或 CLI 回归。失败 run/result/evidence 保留，未为获得绿色结果而删除。CI 或其他沙箱执行方必须显式提供 Godot 标准用户数据写入能力，否则严格 ERROR 检测会保守失败。

## 5. 只读性复验

完整 E2E 完成后，对项目副本全部 regular files 计算排序 SHA-256 树摘要；再次执行 strict candidate check 前后均为：

`91d88a89384912163b3abcb770f202e3f0912116d77ee5595d736bd257aa18ad`

strict 命令退出 `4` 但没有写项目或 candidate；candidate 绝对路径未进入结构化 stdout，只记录 `distribution_candidate=provided`。

## 6. 仍未完成

- GitHub-hosted required CI、远程 provenance、attestation 与 SBOM 未运行。
- Framework CLI/Plugin 的真实下载 quarantine、Gatekeeper 与发布身份仍未关闭。
- 全新 macOS 用户或全新机器的零构建安装路径仍未复验。
- 成功升级、失败升级、上一版本回滚仍按用户决定留到最终候选。
- Windows/Linux 原生 runner 证据、Support Matrix 最终降级决策和最终独立只读审计仍未完成。
- 没有 remote、push、tag、GitHub Release、npm publish 或 Marketplace 发布。
