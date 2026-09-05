# M3 rc.2 最终只读审计

- 日期：2026-09-02
- 审计对象：`0.3.0-rc.2`、源码 `280de4a9939bb437379ad4c743df57b1d9ce9aba` 及其绑定发布证据
- 审计方式：独立 Sol/xhigh 只读审计；审计者未修改仓库
- 结论：`FAIL`；0 Blocker、1 High、1 Medium

## High：运行时支持范围与 ADR 0025 冲突

`supportedHost()` 仍把 Windows amd64 与 Linux amd64 判为 v1 supported；`detect`、`doctor` 和 `initialize` 的文案也仍把三宿主称为 Tier 1。该行为与 ADR 0025 冻结的 macOS Apple Silicon-only v1 直接冲突，而且 rc.2 Plugin 确实包含这些二进制，因此不能按“文档问题”豁免。

影响：rc.2 不是可接受的最终候选；V1-10 的诚实范围门禁不能保持 PASS。修复必须改变源码 revision 和二进制 hash，并生成新候选。

处理：CLI host 判断已改为可测试的纯函数，只允许 `darwin/arm64`；Windows/Linux 继续生成 artifact-only 交叉构建，但运行时返回 unsupported。三平台矩阵测试与用户文案同步修复。后续候选版本为 rc.3，不覆盖 rc.2 历史证据。

## Medium：外部事实记录过薄

rc.2 的 `release-evidence` 1.0.0 只保存生命周期与 branch protection 的 PASS 结论，缺少 Codex CLI 版本、逐操作退出状态、失败升级后的 active version、前后用户状态摘要，以及带采集时间的分支保护规则快照。strict 正确证明了 JSON 与 candidate 的字段绑定，却不能让干净克隆独立审阅这些外部观察的细节。

处理：`release-evidence` 升级为 1.1.0，并要求：

- Codex CLI 版本与远程观察时间。
- 新任务 thread ID、Skill 名称与相对入口路径。
- 固定八步生命周期操作、退出码和关键 active version。
- 安装前后 Plugin/Marketplace 数量与排序清单 hash、Atelier 数量、配置 mode/size/hash，且前后必须完全一致。
- 带 repository、branch、观察时间、required check、strict、admin、force-push、deletion 和 signature 状态的 branch-protection 快照。

1.0.0 只保留为 rc.2 历史证据；rc.3 及后续候选必须提供 1.1.0。

## 已确认无其他中高问题

- rc.2 A/B candidate、manifest/archive hash 和 Marketplace Plugin 树一致。
- rc.2 CLI 的 strict 确实返回 12 PASS、`release_ready=true`，且输入树零写入；该结果只是审计输入，不覆盖终审 High。
- MIT、NOTICE、Go `THIRD_PARTY_NOTICES` 与 clean provenance 未发现新问题。
- Plugin-only、Godot-only、无 Intel/Windows/Linux 原生承诺、无签名/公证对外文档总体诚实；冲突集中在运行时代码。
- 文档没有把 `release_ready=true` 等同于正式发布。

## 仍未运行

- 独立用户或第二台 Apple Silicon 机器复验。
- 受保护版本 ref/tag、正式 Plugin 发布和用户批准。
- rc.3 的重建、远程安装、生命周期、Skill 发现、绑定 strict 与再次独立终审。
