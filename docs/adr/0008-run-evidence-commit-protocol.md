# ADR 0008：Run 与 Evidence 逻辑原子提交协议

- 状态：Accepted（Phase 1 macOS Apple Silicon/APFS 静态 validate 生产基线；尚非 v1 跨宿主承诺）
- 日期：2026-08-25
- 决策范围：run 目录、正式提交点、引用闭包、崩溃恢复、state index 与外部产物边界

## 背景

ADR 0005 已确定 `validate`、`test`、`build`、`export`、`release check` 默认持久化最小可审计 run/evidence，且 manual/standard/strict 不关闭记录。ADR 0007 只证明单个 `project.json` 的 no-replace 发布，不能自动证明 payload、evidence record、result 和外部 artifact 的多文件事务安全。

本协议需要保证：崩溃或任一文件写入失败时，要么没有正式结果，要么正式结果引用的整个 evidence 闭包完整；绝不能留下可被 status、CI 或 release check 聚合成 `PASS` 的半事务。

## 决策

### 1. 每个 run 自包含

```text
.gameatelier/runs/<run-id>/
├── intent.json
├── payloads/
│   └── 0001-<kind>.<ext>
├── evidence/
│   └── 0001-<kind>.json
├── result.json
└── .run.lock                 # 未来 recovery/clean 使用；首个 commit slice 不创建
```

- `run-id` 沿用小写 ASCII 的 `atelier-<utc>-<random>`；run 目录必须唯一且 no-replace。
- `intent.json` 是不可变恢复意图，不是成功事实。目录存在但没有合法 `result.json` 时一律为 `incomplete`。
- evidence id 为 `<run-id>-eNNNN`；ref 固定指向同一 run 目录内的 evidence record。
- 第一版不使用全局散落的 `.gameatelier/evidence/`，避免 run 闭包跨目录扩散。
- 首个事务薄切片要求每条 evidence 的 outcome 与顶层 command result 完全一致，避免 `result=PASS` 引用 `FAIL/BLOCKED/NOT_RUN` 证据。出现真实的混合检查调用方后，再通过新契约定义聚合规则。
- 首个事务薄切片只允许一份严格 `validation-report`，且 result `check_count` 必须等于 report checks 数量；多 evidence 聚合暂不实现。

### 2. `result.json` 是唯一正式提交点

提交顺序固定为：

1. 安全加载已初始化 project state，冻结 `project_id`、观察到的 revision、policy mode 和 engine。
2. 通过固定 `.gameatelier` root 创建并安全打开唯一 run root。
3. 原子发布脱敏、规范化的 `intent.json`。
4. 执行命令；只允许契约声明且 containment 通过的项目相对写入。
5. 对 operation result 与 payload 完成有界预检，生成确定性的 evidence refs，并把最终 command-result 编码为唯一一份待提交 bytes；在此时拒绝超限或不可编码结果，尚不发布 result。
6. 对每个 payload 执行 temp → write → file sync → close → no-replace publish → directory sync，并计算 SHA-256 与 byte size。
7. payload 发布并复验后，以同样方式发布对应 evidence record。
8. 验证完整引用闭包：regular/no-symlink、schema、ID/run ID、path containment、hash、size、时间和脱敏门禁全部一致。
9. 将第 5 步准备的 result bytes 同步到 temp，最后 no-replace 发布 `result.json` 并同步目录。该发布是逻辑 commit。
10. stdout 直接输出第 9 步同一份 bytes，不重新生成 run ID、时间、summary 或 evidence refs。

在 `result.json` 发布前，payload/evidence 文件与目录项必须完成当前宿主已承诺的 durability barrier。result 发布后目录 sync 失败表示逻辑事务完整可见但断电持久性未确认；不得删除或重写 result。

### 3. Operation outcome 与 commit outcome 分离

- 命令自身的 `FAIL`、`BLOCKED`、`SKIPPED` 只要闭包与 result 成功提交，就是合法 committed run。
- intent 后、result 前发生 handled 写入/闭包失败时，stdout 必须降为 `FAIL`/7、`RUN_COMMIT_FAILED`、`evidence: []`；不得输出原 operation 的 `PASS`。
- run root 前无法启用事务时返回 `FAIL`/7、`RUN_RECORDING_UNAVAILABLE`；run root 已创建但 intent 未发布时返回 `FAIL`/7、`RUN_PREPARE_FAILED`。两者均不得声称存在 incomplete intent。
- result 已发布但 stdout 写失败时，run 仍是权威 committed 事实；不得重执行或改写同一 run。
- result 已发布但最终目录 durability 或 close 未确认时，stdout 与进程退出码仍使用同一份权威 result bytes，并在 stderr 输出固定诊断；不得用第二份 JSON 改写已提交 operation outcome。
- result 存在但闭包缺失/hash 不符时标为 `RUN_CORRUPT`，不得聚合 PASS。

### 4. 第一版不更新 `project.json`

run 使用唯一目录、append-only/no-replace，正常提交不持有 project lock 跨越 Godot/test/build/export 执行。第一版不修改 `active_run_refs`、`last_command_result_ref`、revision 或 updated_at；当前 `status` 仍只读 `project.json`，未来安全 scanner 才负责发现 committed、incomplete 与 orphan runs。

未来 state index 只能在 result commit 后短暂持 project lock进行，并且是可重建派生数据。index 更新失败不能推翻已经 committed 的 run，也不能把 operation PASS 改成事务 FAIL。只有真实 state index 变化才增加 revision。

### 5. Schema 増补

- 新增 immutable `run-intent`：schema/run/project/revision/mode/command/started_at/producer/expected result ref/declared project writes/符号化 external writes。
- evidence record 必须包含 `run_id`；持久事务中的 regular payload 必须包含 `sha256` 与 `byte_size`。
- runtime 强制 `ref.id == record.id`、record/result/enclosing dir run ID 一致、payload hash/size 一致。
- build/export 多文件产物使用 immutable artifact manifest；hash 只证明记录时快照，不宣称外部 artifact 永久不变。
- metadata 保持有界，不能成为未脱敏原始日志或任意参数垃圾桶。

### 6. 恢复与清理

- run root 创建前的预检失败：没有 run，公开 `validate` 返回固定 `RUN_RECORDING_UNAVAILABLE`，不泄露内部路径或原始系统错误。
- 唯一 run root 已创建但 `intent.json` 尚未完成 no-replace 发布时发生故障：可以留下没有 intent 的 `orphan` 目录；它永远不是 PASS，未来 scanner 必须与有 intent 的 `incomplete` 分开报告，清理仍需精确列出和显式确认。
- intent 后、result 前崩溃：保留 incomplete run 与已写材料，不自动 glob 删除，不计 PASS。
- recovery 只能在精确 run root 的 `.run.lock` 下进行。只有预生成 result 与完整闭包均可证明时才能 no-replace finalize；未知 engine outcome 不得猜测 PASS。首个 commit slice 不实现 recovery 或创建 `.run.lock`，只保留/识别 incomplete；实现 recovery 前该项保持 NOT RUN。
- 普通重试生成新 run ID；同一 completed run 永不覆盖或重执行。
- `clean` 未来只能先列出 incomplete/orphan 的精确路径，删除仍需独立预览和确认。

## 明确边界

- 这是 result 引用闭包的逻辑原子提交，不是 `.gameatelier` 与外部 build/export artifact 的分布式事务。
- macOS/APFS 以外的 durability、Windows/Linux、SMB/FAT/exFAT/网络盘在原生故障注入前保持 NOT RUN 或 gate。
- macOS 实现对 pinned `.gameatelier`、`runs`、具体 run、`payloads` 与 `evidence` 逐层执行 `Fstatfs`，要求全部为 APFS 且 FSID 与 state root 相同；因此既有真实目录形式的嵌套其他卷/网络挂载也不能绕过 gate。可注入 FSID 回归已通过，真实嵌套非 APFS mount 故障演练仍记为 NOT RUN。
- raw Godot stdout/stderr 必须有固定上限、UTF-8 策略、脱敏、truncated 与原始/保留字节计数；secret scan 在 result commit 前是硬门禁。
- 已批准的 evidence 政策不等于允许 Godot 写用户 HOME、全局配置或网络。ADR 0009 仅允许用户显式授权 Godot 官方标准 `user://` 位置，并要求 intent 记录符号化外部写入；其他外部写入仍不获授权。
- 不提供 `--no-record` 绕过关键命令 evidence，也不自动重试具有外部副作用的 engine/build/export 操作。

## 故障注入验收

至少覆盖每个 create/write/sync/close/publish、每个 evidence 第 N 项、result temp sync 前后、result publish 前后、stdout failure 和进程 hard-exit。每次必须满足：没有 result，或 result 的整个引用闭包严格完整。

## 备选方案

### 把 `project.json` index 当强事务成员

拒绝。result 必须先存在才能被引用，后续 index 写失败会制造“完整 PASS 是否算失败”的循环。index 应是可重建派生数据。

### payload/evidence 写完即算成功

拒绝。崩溃可能留下任意前缀，无法确定 operation 是否完整结束。

### run store 成功就宣称 build/export artifact 原子

拒绝。外部 artifact 具有独立覆盖、备份和回滚语义，只能由各命令契约与 manifest 证明。

## 回退

本 ADR 已在独立只读审计、故障注入、race/vet/schema 与三目标交叉编译后接受为 Phase 1 限定基线。回退只移除尚未发布的 run-store 命令路径；不得自动删除用户项目中已生成的 orphan/incomplete/committed run。任何清理必须遵守精确列出与显式确认。
