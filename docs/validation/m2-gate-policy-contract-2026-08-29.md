# M2 三模式门禁策略契约验证

- 日期：2026-08-29
- 结论：PASS（策略契约子项；CLI/hook/CI 尚未完成）

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

## 尚未完成

- 现有 build/export 还没有执行 policy 新增的 standard/strict workflow gates。
- 只读 `release check` 命令尚未实现。
- Git hook 安装/卸载与最小 CI 尚未实现。

因此本文件只证明门禁表可分发且不可静默削弱，不把 V1-06 标记为 PASS。
