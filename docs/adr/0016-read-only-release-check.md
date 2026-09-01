# ADR 0016：只读 Release Check 与当前证据聚合

- 状态：Accepted（M2 生产契约；strict 分发门禁仍待 M3）
- 日期：2026-08-29
- 决策范围：`release check` 的副作用、模式解析、证据新鲜度、artifact 复验和退出语义
- 后续变更：ADR 0022 接入本地 candidate；ADR 0026 接入与 candidate 绑定的单一外部 release evidence。只读、零项目 evidence 与保守阻断语义不变。

## 背景

ADR 0005/0008 的早期草案把 `release check` 与生产执行命令一起列入持久化 run。M2 随后冻结的三里程碑方案要求它是只读聚合器。若审计命令自身写入被审计的 run store，它会在扫描后改变状态、需要第二轮自验证，并让“检查前状态”和“检查后状态”不再一致。

同时，简单寻找“任意历史 PASS”会让较新的失败被旧记录掩盖；只信任 export manifest 又会让已删除或篡改的 ZIP 继续被视为可用。

## 决策

1. 公开命令为 `release check --project <path> [--mode manual|standard|strict]`。省略 `--mode` 时解析项目 state 的 mode；显式覆盖只影响本次调用，不改写项目。
2. 命令零文件写入、零 evidence、不创建 lock、不启动 Godot、不执行外部发布。它从 persisted-run allow-list 与 `run-intent` 命令枚举中移除。
3. `manual` 只检查项目状态与 Godot 4.7.2-stable standard/GDScript 支持范围；通过时仍返回 `release_ready: false`。
4. `standard` 还要求有界 run store 不含 incomplete、orphan 或 corrupt，且当前 project revision 内最新的 Headless validate、固定 test、Release export 都为 `PASS`。`latest` 按严格验证过的 `finished_at` 选择，旧 PASS 不能覆盖新失败。
5. 对所选 Release export，命令从受限 `.gameatelier/artifacts/<run-id>/game-release.zip` 重新读取文件，复算 byte size/SHA-256，并重新验证 bounded ZIP 与唯一 Universal 2 Mach-O 形状；缺失、篡改或不安全文件使该门禁 `BLOCKED`。
6. `strict` 是唯一可能返回 `release_ready: true` 的模式。M3 尚未实现的 source、Plugin、Starter、许可/来源与 CI 聚合必须逐项输出 `NOT_RUN`，并使本次命令 `BLOCKED`/4；不得用措辞冒充发布就绪。
7. run scan 的非法结构或预算耗尽返回 `FAIL`/7；取消或 deadline 返回 `FAIL`/6。所有失败均不返回部分发布结论。

## 备选方案

### 持久化 release-check run

拒绝。它制造自引用审计与无价值的状态变化；上游执行命令已经留下不可变事实，CI 可保存本次 stdout 作为流水线 artifact。

### 接受任意当前 revision 的 PASS

拒绝。较新失败必须推翻较早成功，否则“latest”门禁没有实际约束力。

### 只验证 manifest，不重读 ZIP

拒绝。manifest 证明 export 当时的观察，不证明 artifact 现在仍存在且未改变。

## 风险

- 重新读取最大 4 GiB artifact 有确定性的 I/O 成本；调用方可以取消，读取沿用有界流式实现，不把文件载入内存。
- 活动 writer 可能暂时表现为 incomplete，因此 release check 会保守阻断；正式发布检查不应与生产执行并发。
- 当前 strict 必然因 M3 项目 `NOT_RUN` 阻断。这是已知阶段边界，不是可绕过的警告。

## 迁移与回退

项目尚未发布，因此没有持久化 `release check` run 需要迁移。Schema `1.0.0` 的 `run-intent` 在首次外部冻结前移除该命令；现有 validate/test/build/export 闭包不变。若未来确需审计报告，优先让 CI 保存只读 command-result；任何重新引入项目写入的方案必须新建 ADR，并处理自引用与状态快照语义。
