# ADR 0010：Run scanner 与只读 `clean --list`

状态：Phase 1 已接受的限定基线
日期：2026-08-25

## 背景

ADR 0008 已把 `.gameatelier/runs/<run-id>/result.json` 定义为唯一逻辑提交点，但首个切片只负责写入，尚不能安全回答“哪些 run 已提交、哪些因崩溃未完成、哪些可以进入清理预览”。直接按文件名、目录年龄或命令 outcome 清理会误删有效失败证据、活动中的 run 或已损坏但仍需诊断的材料。

公共 `clean` 命令及退出码/JSON 形状属于公开契约变化，必须先限定读取、分类和失败边界。此 ADR 只接受扫描与预览，不授权删除、自动修复、恢复提交或索引写入。

## 决策

### 1. 第一版公开入口

- 只实现 `clean --list [--project <dir>]`。
- `--list` 必须显式提供；裸 `clean` 返回用法错误 2。
- 命令不创建 run、不写 evidence、不创建 lock、不更新时间戳或索引，stdout 仍只有一个 command-result JSON。
- scanner 继承调用方 context。SIGINT/SIGTERM 或调用方 deadline/cancel 会在目录、文件和 64 KiB 读取块边界协作停止，返回 `FAIL`/6 `COMMAND_CANCELLED`，不返回部分候选。本切片不另设隐式 wall-clock timeout；单次底层文件读取是否能立即中断仍取决于宿主文件系统。
- `data` 固定返回 `scope`、`.gameatelier/runs` 相对路径、是否完成全量扫描、四类计数、候选列表和受保护 corrupt 列表。
- `data` 中的 run 列表只使用严格的 ASCII 项目相对路径，不持久化扫描到的用户绝对路径；command envelope 仍按既有 CLI 契约回显用户传入的 `--project` 参数。

### 2. 状态分类

scanner 在验证当前 `project.json` 后，按精确 run ID 目录逐项分类。intent 的 project ID 必须属于当前项目，记录的 revision 不得大于当前 revision；历史 revision 的合法 run 不因后续项目状态前进而变成 corrupt：

- `committed`：`intent.json` 有效，`result.json` 存在且规范编码，result 与 immutable intent 一致，payload/evidence regular-file 闭包、schema、ID、路径、producer、时间、outcome、hash 和 byte size 全部通过现行预检。
- `incomplete`：存在有效 intent，但没有 result。它永远不计 PASS，可进入清理预览，原因固定为 `RESULT_MISSING`。
- `orphan`：intent 与 result 都不存在。它永远不计 PASS，可进入清理预览，原因固定为 `INTENT_AND_RESULT_MISSING`。
- `corrupt`：存在 result 但 intent/结果/引用闭包不能证明，或者任何现有 intent/result 文件不安全或无效。corrupt 只列入 `protected`，不得成为清理候选。

command outcome 与提交状态继续分离：一个结构完整的 `FAIL`、`BLOCKED` 或 `SKIPPED` result 仍是 `committed`，不是清理候选。

### 3. 目录安全与有界性

- `.gameatelier`、`runs` 和每个 run 必须逐层以 no-symlink real-directory + open identity 检查打开。
- run 目录名必须完全匹配当前固定 run ID 语法；非法名、符号链接、非目录项、打开时身份变化或超过 512 个目录时，整个扫描返回 `FAIL`/7 `RUN_SCAN_UNSAFE`。
- 全扫描最多读取 2,048 个闭包文件、累计 256 MiB；预算耗尽返回 `FAIL`/7 `RUN_SCAN_LIMIT_EXCEEDED`。scanner 只读一次 payload/evidence bytes，再在内存中完成 preflight、hash 和 canonical-record 复验，不为同一闭包重复 I/O。
- 整体失败时 `scanned=false`，所有计数为零，候选和 protected 列表为空；不得输出部分清理结论。
- 现有 intent/result 必须是有界 regular file；duplicate keys、未知字段、尾随 JSON、非 UTF-8、超限和符号链接均不能被当作有效提交。

512 是此切片对内存、stdout 大小和审阅可操作性的安全上限，不是 v1.0 永久容量承诺。未来分页或索引需要新契约和迁移决策。

### 4. 并发与未来删除边界

此只读 scanner 不创建 `.run.lock`，因此不会阻塞正在写入的 run。活动 run 在 run root 创建后、intent 发布前可能被观察为 `orphan`，在 intent 发布后、result 发布前可能被观察为 `incomplete`；候选列表只是预览，不是删除授权。

未来任何实际删除必须作为独立实现，并且上线前先升级 writer、cleaner 与 recovery 使用同一 per-run 协调协议。该协议必须解决 run-root/lock 首次创建竞态，并保证 writer 从 run root 可见前到 result commit/放弃期间持有 cleaner 能观察的排他所有权；仅由 cleaner 单方面创建 `.run.lock` 不构成安全同步。在共同协议实现并通过故障注入前不得接入删除。届时删除还必须：

1. 用户明确选择精确 run ID/路径并确认。
2. 按共同协议获取精确 per-run lock/lease，并证明没有活动 writer。
3. 在锁内重新打开并重新分类，确认仍为 `incomplete` 或 `orphan`。
4. 列出/验证目录内的实际成员，使用 rooted、no-follow 操作，不使用 glob。
5. 状态发生变化、出现 corrupt/committed 或安全检查失败时拒绝删除。

恢复也不在本 ADR 范围。只有未来能证明预生成 result 与完整闭包时，才能在锁内 no-replace finalize；未知引擎 outcome 不得猜测 PASS。

## 备选方案

### `status` 顺带扫描 runs

拒绝。`status` 已冻结为只读 `project.json` 摘要；把昂贵闭包扫描隐式加入会改变其性能、失败语义和稳定用途。`clean --list` 使用户意图与成本明确。

### 只看 `result.json` 是否存在

拒绝。存在但闭包缺失、hash 不符或 intent 不一致的 result 不能证明 committed。

### 把 corrupt 也列为可清理

拒绝。corrupt 可能包含唯一故障证据或被恶意构造；在专门的诊断、备份和显式确认流程出现前必须保护。

### 自动删除 orphan/incomplete

拒绝。无锁扫描可能观察到活动 run，自动删除会破坏正在进行的事务和故障证据。

## 风险

- 512 个目录、2,048 个读取文件或 256 MiB 累计内容以上的项目当前会整体失败，需要未来分页/派生索引方案。
- 扫描成本与完整闭包数量线性相关；第一版选择有界、可取消、可证明正确性而非快速索引。
- 只读扫描无法区分崩溃遗留与正处于 root→intent 或 intent→result 窗口的有效 active run，因此任何删除必须在共同 writer/cleaner 协议下锁内重验。
- 当前闭包预检覆盖已实现的 `validate` 与 ADR 0011 `test` 记录。新增 `build/export/release check` 前必须扩展 scanner 的命令特定闭包验证，不能把未知形状误判为 committed。

## 回退

可以移除公开 `clean --list` dispatcher、`clean-data` schema 和 scanner，但不得删除、改写或重新分类用户已有 run。ADR 0008 的 result commit 语义保持不变。回退前生成的 `clean` stdout 不属于持久化 run evidence，无需迁移项目状态。
