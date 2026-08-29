# ADR 0017：Build/Export 三模式运行时门禁

- 状态：Accepted（M2 生产契约；strict 的 M3 扩展保持阻断）
- 日期：2026-08-29
- 决策范围：build/export 模式解析、自动 gate、总 timeout、失败证据与项目状态副作用

## 背景

ADR 0002 与分发 `gate-policy.json` 已冻结 `manual < standard < strict`，但仅有策略文件不能证明普通 build/export 自己执行门禁。若只依赖用户先运行 validate 或可跳过的 Git hook，直接调用 export 就能绕过生产检查。

项目 state 当前有 mode 字段，但还没有安全的 replace/revision/migration 写协议。为了让三种模式可实际调用，不能要求用户手改 JSON，也不能让普通构建暗中重写 project state。

## 决策

1. build/export 默认使用 `project.json` 的 mode，并接受 `--mode manual|standard|strict` 单次覆盖。覆盖进入原命令 immutable intent，不修改项目 state。
2. `manual` 只省略 workflow 扩展；现有宿主、授权、Godot standard/GDScript、固定 preset/templates、artifact integrity、Universal 2 与 Apple Silicon target smoke 门禁全部保留。
3. `standard` 在真正导出前依次调用同一生产 `validate --headless` 与固定 `test` 实现。两条 gate 各自提交 run/evidence，原 build/export 也提交最终 PASS/FAIL/BLOCKED manifest；不实现第二套影子检查器。
4. 任一 gate 非 PASS 时立即停止，不启动真正导出。原命令使用稳定 `POLICY_GATE_FAILED`（取消使用 `COMMAND_CANCELLED`），并只在 details 中记录上游命令、outcome、退出码与错误码，不复制自由文本或路径。
5. standard/strict 的 Headless、test、export 共用调用者给定的一个总 timeout；每段只取得剩余时间，不能把三份 timeout 串联放大。
6. `strict` 先完整执行 standard 子集，再对尚未实现的 `run-store-integrity`、`source-tree-policy`、`distribution-metadata` 返回 `STRICT_GATES_UNAVAILABLE`/`BLOCKED`。在 M3 契约冻结前不启动真正导出，也不声称 strict 可用。
7. 内部 gate 的 intent 继承本次有效 mode；这不改变公开 validate/test 参数或项目 state。

## 备选方案

### 只依赖 Git hook 或 CI

拒绝。直接调用 CLI、`--no-verify` 或没有 Git 的 Starter 项目都可绕过。

### 发现已有 PASS evidence 就跳过 gate

拒绝。M2 没有冻结输入树摘要和 evidence cache key；重用旧结果会产生错误新鲜度声明。

### 立即实现项目 mode 持久修改

拒绝。它需要 state replace、revision、并发锁、迁移和回滚协议，超出本切片；单次 override 已提供确定性入口且零配置副作用。

## 风险

- standard 构建时间增加，因为每次都真实执行 Headless 与 tests；这是没有可信 cache key 时的保守成本。
- gate 与 outer command 产生多条 run；release check 按各自命令类型和时间聚合，不把 outer manifest 当作 nested gate 替代品。
- strict 当前必然阻断。用户可显式选择 standard，但不得把它描述为 strict 或发布就绪。

## 迁移与回退

历史 build/export intent 没有 `command.arguments.mode`，仍按其已有 `policy_mode` 合法读取；新运行会在 arguments 与 intent 中记录有效 mode。回退时可移除可选 flag 而不改旧 project state。任何项目级 mode 持久修改必须另立 ADR 并实现原子 replace/revision/migration 验证。
