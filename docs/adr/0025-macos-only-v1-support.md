# ADR 0025：v1 生产支持收敛为 macOS Apple Silicon

- 状态：Accepted（用户于 2026-09-01 明确批准）
- 日期：2026-09-01
- 决策范围：v1 宿主、导出目标、CI 与发布门禁
- 取代：此前三宿主 Tier 1 与 Windows/Linux v1 desktop export 建议

## 背景

现有生产实现、真实 Godot 工作流、原子文件语义、Plugin 生命周期和目标 smoke 均只在 macOS Apple Silicon 完成。Windows x64 与 Linux x64 只有 Go 交叉编译及二进制格式/provenance 证据；没有原生文件系统、进程树、Godot、导出或恢复矩阵。继续把它们列为 v1 Tier 1 会让文档承诺与证据不一致，并阻塞已收敛的 Plugin-only v1。

## 决策

1. v1 唯一生产级宿主是 **macOS Apple Silicon**。
2. v1 唯一导出承诺是从该宿主生成 Godot `macOS Technical` Universal 2 Debug/Release 技术产物，并只在 Apple Silicon 运行 smoke；不声明 Intel 运行、签名、公证或公开游戏分发就绪。
3. Windows x64 与 Linux x64 从 v1 Tier 1、测试矩阵和发布门禁移除。当前 Plugin 可继续携带 deterministic cross-build artifact，供未来验证使用，但它们是 `artifact-only / unsupported in v1`：不得执行、宣传、自动选择或据此推导原生支持。
4. v1 required CI 只需要 macOS Apple Silicon 原生 job；job 必须观察并断言 `arm64`，否则只能作为通用静态 CI，不能关闭生产宿主门禁。
5. 新 Godot 4.7.x patch 的 7 天重验目标只覆盖冻结的 macOS Apple Silicon 宿主、核心工作流和 macOS 技术导出。
6. Windows/Linux 后续进入支持范围必须新立 ADR，并分别完成原生 CLI、文件系统/锁/取消、Godot Headless/test/build/export、路径、恢复、安装与升级矩阵。

## 备选方案

### 维持三宿主 Tier 1

不采用。需要新增两类原生机器、Godot 安装与长期 CI 成本，且用户已决定当前不建设这些 runner。

### 从 Plugin 删除 Windows/Linux artifact

本次不采用。现有 artifact 是可复现供应链与未来验证输入，保留不等于支持；若最终包体或审计证明它们没有实际价值，可通过新的分发变更删除。

## 风险与回退

- macOS-only 缩小潜在用户范围，但让 v1 声明与真实 evidence 一致。
- 包内仍携带 unsupported artifact 可能造成误解；Skill、manifest 和 CLI 必须继续阻断非 Apple Silicon 宿主。
- 如后续补齐某一宿主，可用新 ADR 将其升为 preview 或 Tier 1；不能仅改文案或依赖交叉编译。

## 验收

- Support Matrix、路线图、验收门禁、公开 README 和相关 ADR 不再把 Windows/Linux 称为 v1 Tier 1。
- `V1-10` 不再因缺少 Windows/Linux 原生证据而 BLOCKED；其 PASS 仍依赖文档、manifest 与实际 macOS 证据一致。
- CI 明确区分 Apple Silicon 原生验证与 Windows/Linux artifact-only 交叉编译。
