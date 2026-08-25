# ADR 0012：已提交 Run 的结构化日志投影

状态：Phase 1 已接受的生产薄切片
日期：2026-08-25

## 背景

v1.0 需要结构化日志、稳定错误和 secret redaction，但当前 Headless/test 路径故意不持久化 Godot 原始 stdout/stderr。原始项目输出可能包含绝对路径、凭据、高熵秘密或大量文本；现有字符串门禁只适合约束既有 result/report，不能证明任意自由文本已经完整脱敏。

Phase 1 已有严格 committed run 闭包：intent、result、单一 validation/test payload、evidence record、hash/size/path/producer/time 和 payload 语义都由同一次 rooted、有界读取验证。第一步应先让用户安全查询这些已经存在的结构化事实，不提前冻结 raw 日志的保留、脱敏和分片策略。

## 决策

### 1. 公开命令

- 新增 `logs [--project <dir>] --run-id <canonical-run-id>`。
- `--run-id` 必须匹配现有严格 run ID；不接受任意路径、前后缀、模糊查询、`--raw`、`--follow`、`--tail` 或格式切换。
- 第一版必须显式选择 run，不定义“latest”的排序或索引语义；以后如需默认选择，再单独冻结。
- 结果中的 project 参数规范化为 `.`，不回显调用者绝对路径。

### 2. 单 Run、同次读取的验证快照

- `logs` 直接打开指定 `.gameatelier/runs/<run-id>`，不扫描其他 run，也不依赖当前不维护的 `last_command_result_ref`。
- 复用 scanner 的 `os.Root` containment、目录和文件 `Lstat`/open/`SameFile`、no-symlink/regular-file、64 KiB cancellation chunk 与严格 JSON/闭包校验。
- 同一次读取把已验证 result、payload 和 evidence record 保留在内存；投影后不得按路径重新打开，避免验证与展示之间的二次 TOCTOU。
- 单 run 最多读取四个闭包文件：intent 256 KiB，result/payload/evidence record 各 4 MiB，总预算 12.25 MiB。任何缺失、替换、截断、hash/size/path/producer/time/outcome 或语义不一致都 fail closed。
- `logs` 自身纯读：不创建 run、evidence、lock、索引或缓存，不启动 Godot。

### 3. 零自由文本结构投影

成功结果只包含白名单结构字段：

- source command/outcome/started/finished/exit code；
- evidence kind、SHA-256、byte size 和 producer version；
- 按原报告顺序生成的 `check-0001`/`test-0001` 与 outcome；不返回项目控制的原始 ID；
- 按 result 顺序生成的 `error-0001` 与 outcome；不返回 source error code 或 message/remediation/details；
- 固定 `command-finished` 事件；
- `raw_output_included=false`。

不投影 source result summary、error code/message/remediation/details、check/test ID/summary、intent、payload 原文、evidence 路径或任何 raw stdout/stderr。事件 ID 完全由 CLI 按序号生成。这样即使项目控制字段包含没有关键词的秘密或编码数据，`logs` 也不会二次暴露它。

事件 level 由 outcome 确定：FAIL 为 ERROR，BLOCKED 为 WARNING，其余为 INFO。当前最大 321 项：256 个 test/check 上限中的较大者、64 个 result errors 和一个结束事件。

### 4. 结果分类

- 非法参数：`INVALID_ARGUMENT`/2。
- 请求的 project root 无法安全解析：`RUN_LOGS_FAILED`/7。
- 项目未初始化或 run 不存在：`PROJECT_NOT_INITIALIZED`/4、`RUN_NOT_FOUND`/4。
- incomplete/orphan：`RUN_NOT_COMMITTED`/4，不返回部分事件。
- 未来/未知 schema：`RUN_SCHEMA_UNSUPPORTED`/7，作为 protected 状态，不混同可清理候选。
- corrupt/unsafe closure：`RUN_LOGS_UNSAFE`/7。
- 单 run 读取预算耗尽：`RUN_LOGS_LIMIT_EXCEEDED`/7。
- 调用方取消/deadline：`COMMAND_CANCELLED`/6。

只有完整 committed closure 返回 PASS/0。业务结果本身可以是 PASS、FAIL、BLOCKED 或 SKIPPED；已提交失败仍是可查询事实。

## 备选方案

### 返回完整 result、report 和 evidence record

不采用。它诊断信息更多，但会把项目控制的 summary/error 文本重新暴露给调用方；现有启发式关键词检查不能构成完整 secret redaction。

### 立即保存和查询原始 stdout/stderr

延期。需要先决定写入端和读取端脱敏、二进制/无效 UTF-8、分片、截断、保留期限、升级迁移和用户删除语义。这会改变隐私与存储边界，必须单独审阅。

### 默认返回最新 run

延期。它要求冻结排序、无 committed run、较新 corrupt/incomplete run、扫描预算和未来索引的一致语义。显式 ID 更小、更确定。

## 风险

- 这是结构化诊断索引，不是原始引擎日志；没有文本上下文时，一些问题仍需结合稳定错误码或以后获批的受控日志捕获。
- 闭包自洽不证明同一用户可写项目目录从未整体伪造；本契约防止路径逃逸和验证后重读，不把本地项目目录当作敌对用户隔离边界。
- 未知 schema 只受保护而不迁移；恢复/迁移命令仍属于 Phase 2。

## 回退

可以移除公开 dispatcher、`logs-data` schema 和结构投影，但不得修改或删除任何既有 run。scanner 对未知 schema 的 protected 分类可以保留；回退不得引入 raw 日志持久化或改变原 committed closure。
