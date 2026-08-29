# M2 三模式门禁策略契约验证

- 日期：2026-08-29
- 结论：PASS（策略契约子项；运行时结果另见 M2 runtime 记录）

## 决策

门禁规则不散落在 Skill 文案、Go 分支、hook 和 CI 中各自维护，而是先冻结一张随 Plugin 分发的 `gate-policy.json`。默认模式为 `standard`，且每个命令必须满足 `manual ⊂ standard ⊂ strict`。

`manual` 只省略工作流扩展，不能移除 build/export 的项目状态、宿主、Godot 标准版、GDScript、外部写入授权、固定 preset、artifact 完整性和 target smoke。`standard` 加入 Headless 主场景和固定测试；`strict` 再加入 run store、source 和分发检查。`release-check` 的 manual/standard 集合明确不包含完整发布项，只有 strict 才聚合 Plugin、Starter、许可/来源和 required CI。

## 验证

- Draft 2020-12：22 schemas、27 fixtures；分发 policy 与 contract fixture 一致。
- 4 项 policy 语义测试 PASS：默认值、build/export 一致、单调超集、manual mandatory safety、strict-only release set。
- 16 项 Plugin packager 测试中的 gate-policy 篡改负例 PASS；删除 `target-smoke` 会被打包器拒绝。
- 实际 bundle 构建与 Apple Silicon CLI/runner native smoke PASS。
- gate policy SHA-256：`93697a37fc30dd49633afaafc933284739322184100e8a30afe717bb128dea83`。
- bundle manifest SHA-256：`5e65fb31d2a97e654c39c6451ee5258c30730b75e3424c6bc002c9d39121c746`。

## 后续实现状态

- build/export 已执行 standard Headless/test workflow gates；strict 先执行同一子集，再对 M3 尚未冻结的 run-store/source/distribution 项明确阻断。
- 只读 `release check` 已实现 manual/standard/strict 聚合、当前 revision latest 选择及 Release ZIP 复验；manual/standard 不返回 strict release-ready。
- 显式可选 Git hook 生命周期与最小 macOS CI workflow 已实现并通过本地 contract/等价命令；GitHub-hosted 首次运行仍 `NOT RUN`。

本文件原始结论仍只证明门禁表；运行时证据见 [`m2-gate-runtime-2026-08-29.md`](m2-gate-runtime-2026-08-29.md)。由于 strict M3 gates、GitHub-hosted CI 和修复后独立复审尚未完成，V1-06 仍保持 PARTIAL。
