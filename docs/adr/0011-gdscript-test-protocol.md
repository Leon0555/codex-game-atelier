# ADR 0011：固定 GDScript 测试协议

状态：Phase 1 已接受的生产薄切片
日期：2026-08-25

## 背景

v1.0 验收要求把 GDScript 测试的通过、断言失败、引擎失败、超时和取消映射为稳定 CLI 结果，并保留可验证 evidence。已有 Headless 路径已经固定项目目录、配套私有 runner 和 Godot 可执行文件身份，但主场景一帧不能表达逐项测试结果；直接开放任意 `--script`、任意 Godot 参数或 shell 命令又会把通用任意代码执行变成公共接口。

Godot 社区测试框架各有依赖、安装和报告格式。本切片需要先验证零额外依赖的公共契约，并让 Starter Template/参考游戏具有可运行基线，而不是提前冻结第三方框架。

## 决策

### 1. 公开命令与固定入口

- 新增 `test [--project <dir>] [--godot <executable>] [--timeout-ms <1..3600000>] [--allow-engine-user-data]`。
- `test` 不接受任意 Godot 参数、任意脚本路径、测试过滤表达式或 shell 命令。
- 第一版只执行固定项目入口 `res://tests/atelier_test_runner.gd`。该文件必须是项目根内 `tests/` 普通目录中的非符号链接 regular file，大小为 1 byte 到 1 MiB。
- 私有 runner 固定执行 `Godot --headless --path . --script res://tests/atelier_test_runner.gd --no-header`；公开 CLI 不能改变这组参数。
- `test` 会执行用户项目中的 GDScript。它只适用于用户拥有或已审阅的项目；当前不隔离项目代码发起的网络、绝对路径或其他外部操作，也不把这种项目代码执行扩展为通用 eval/shell 能力。

### 2. 受控执行和副作用

- `test` 复用 ADR 0009 的固定项目目录、阶段独立 runner/engine 快照、总超时、取消、进程组回收和每流 256 KiB 输出上限。
- version 与 test 两阶段都从已打开的相同 runner/Godot 源身份建立独立瞬时快照；执行前后校验源和快照散列。任一瞬时文件清理失败时禁止发布 `result.json`。
- Godot 标准 `user://` 必须由 `--allow-engine-user-data` 明示授权。授权写入在 intent 中只记录符号值 `godot:user-data:standard-os-location`，不记录用户绝对路径。
- 固定测试入口在执行前后读取并比较 SHA-256。文件或项目公开路径身份变化时丢弃观察结果；这不是项目全树快照，项目中的其他受信代码/资源仍可能在运行中被修改，完整源码快照不属于本切片。

### 3. 测试报告协议

测试入口必须在 stdout 恰好输出一行：

```text
CODEX_GAME_ATELIER_TEST_REPORT {strict-json-object}
```

JSON 固定包含 `schema_version=1.0.0`、聚合 `outcome=PASS|FAIL` 和 1..256 个 tests。每项包含小写稳定 ID、`PASS|FAIL` 和不超过 512 字符的摘要。CLI 拒绝重复 marker、重复 JSON key、未知字段、重复测试 ID、无效 UTF-8/控制文本、矛盾聚合、尾随 JSON 和输出截断。

- 报告 `PASS`、所有测试 PASS、Godot exit 0 且无 bounded `ERROR:`：命令 `PASS`/0。
- 报告 `FAIL`、至少一个测试 FAIL、Godot exit 1 且无引擎错误/截断：`FAIL`/3，`GDSCRIPT_TESTS_FAILED`。
- 缺失/无效/矛盾报告：`FAIL`/5，`GDSCRIPT_TEST_REPORT_INVALID` 或底层稳定引擎错误。
- 超时或取消：`FAIL`/6；版本或固定入口缺失等前置条件：`BLOCKED`/4。

原始 stdout/stderr 暂不持久化，避免在日志脱敏契约完成前保存任意项目输出。持久 evidence 是 CLI 严格解析后重新编码的 `test-report`，包含 Godot 自报版本、逐项测试和聚合计数。

### 4. Evidence 与闭包

- `test` 默认创建 immutable intent、自包含 `test-report` payload/evidence，最后 no-replace 发布 `result.json`。
- `data` 固定为 `scope=gdscript`、test/passed/failed counts 和已验证 engine version；未启动/无有效报告时 counts 为零。
- run preflight 和只读 scanner 同时验证 test command 参数、external-write 声明、result 计数、报告逐项语义、hash/size 和 intent/result 一致性。未知或矛盾闭包归为 corrupt/protected，不能成为清理候选。
- result 提交失败仍按 ADR 0008 返回 `RUN_COMMIT_FAILED`，不得把内存中的测试 PASS 输出为成功。

## 备选方案

### 第一版直接依赖 GUT/WAT

暂不采用。它们可以在未来作为 Starter Template 选项或适配层，但 v1 公共结果协议不应先绑定某一第三方版本、安装方式和报告格式。

### 开放 `--test-script` 或原样转发 Godot 参数

拒绝。它扩大路径、参数注入和支持矩阵，也会把任意脚本启动变成公共核心接口。需要新框架时应增加经过验证的固定适配器，而不是放开 argv。

### 把测试继续塞进主场景 Headless validate

拒绝。验证主场景能启动不等于逐项测试；混合后无法稳定区分资源/场景错误、断言失败和测试协议错误。

### 持久化全部 stdout/stderr

延期到 `logs` 契约。项目输出可能含绝对路径、凭据或大量非结构化文本；在脱敏、分片和查询边界冻结前，只保存严格派生报告。

## 风险

- 固定入口是最小协议，不是完整测试框架；fixture、异步测试、测试隔离和过滤仍需后续由实际需求扩展。
- 测试执行的是受信项目代码，不是安全沙箱。CLI 只控制它如何启动、捕获和记录，不能证明项目代码没有额外副作用。
- 仅比较固定 runner 前后内容，不能阻止其他脚本/资源的并发变化；v1 是否需要项目内容 manifest 由 build/export 与干净环境验收共同决定。
- Windows x64、Linux x64 原生执行按用户决定延期，当前只有交叉编译，不得据此宣称三宿主测试运行均已验证。

## 回退

可以移除公开 `test` dispatcher、固定 runner stage、`test-data`/`test-report` schema 和模板入口，但不得删除已经提交的 test run。scanner 必须继续把旧合法 test 闭包识别为 committed，或在破坏性 schema 迁移前将其明确保护；不得把未知旧记录误列为删除候选。
