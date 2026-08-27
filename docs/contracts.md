# Phase 1 结构化契约

状态：Phase 1 生产基线；通用 envelope 和首批只读命令已实现，长期兼容仍等待完整薄切片验证。

## 目标

`schemas/v1/` 定义 CLI、状态和证据文件之间的最小边界。契约刻意不规定实现语言，不包含具体模型 ID，也不把 Godot 内部对象扩散到通用任务状态。

## 文件

- `command-result.schema.json`：一次确定性 CLI 调用的机器可读结果。
- `error.schema.json`：稳定错误码、类别、可重试性和修复提示。
- `evidence.schema.json`：日志、报告、构建或导出产物的可定位记录。
- `run-intent.schema.json`：不可变 run 意图与项目内/符号化外部写入范围；它不是成功记录。
- `validation-report.schema.json`：baseline/headless validate payload 的严格检查列表和聚合结果。
- `test-report.schema.json`：固定 GDScript 测试入口产生的严格逐项报告；CLI 解析后重新编码为 evidence。
- `task.schema.json`：有界工作项、所有权、允许路径和生命周期。
- `handoff.schema.json`：可恢复交接，不承载隐藏推理过程。
- `project-state.schema.json`：项目模式、Godot 选择及任务/run 索引。
- `clean-data.schema.json`、`detect-data.schema.json`、`doctor-data.schema.json`、`initialize-data.schema.json`、`logs-data.schema.json`、`status-data.schema.json`、`test-data.schema.json`、`validate-data.schema.json`：已实现命令的精确 `data` 形状。
- `common.schema.json`：上述 schema 共用的窄定义。

## 结果与状态边界

- CLI 命令结果只使用 `PASS`、`FAIL`、`BLOCKED`、`SKIPPED`。
- 验收或证据还可使用 `NOT_RUN`，且不得把它当作通过。
- 任务状态独立使用 `planned`、`ready`、`active`、`review`、`verified`、`done`，以及 `blocked`、`failed`、`cancelled`。
- `summary` 面向人类；自动化必须读取字段、错误码和 evidence reference，不解析自然语言猜状态。

## CLI 退出码类别

| 退出码 | 含义 |
| --- | --- |
| `0` | 命令完成，结果为 `PASS` 或按明确规则 `SKIPPED` |
| `2` | CLI 用法、参数或配置格式错误 |
| `3` | 验证或策略门禁未通过 |
| `4` | 前置条件缺失，结果为 `BLOCKED` |
| `5` | Godot 或受控外部工具执行失败 |
| `6` | 超时或取消 |
| `7` | 状态、锁、恢复或 schema 迁移失败 |
| `8` | 未分类的框架内部错误 |

底层 Godot 退出状态记录在 `data` 或 evidence 中，不直接替代 Atelier CLI 退出码。

## 路径和隐私

持久化根目录为 `.gameatelier/`。state/evidence record 引用使用 `.gameatelier/` 开头的受限小写 ASCII 路径；artifact 路径使用受约束的项目相对路径。两者都拒绝绝对路径、Windows root/drive/UNC/ADS 形态和 `..` 穿越。日志和结构化参数在落盘前必须去除凭据；schema 只能约束形状，不能替代运行时 containment、符号链接和脱敏测试。

## Phase 1 首批命令

| 命令 | 输入 | 默认副作用 | 当前完成边界 |
| --- | --- | --- | --- |
| `clean --list` | 必选 `--list`；可选 `--project` | 零写入、零 evidence、不创建 lock | 有界扫描 run store；完整验证 committed 闭包，只把 incomplete/orphan 列为预览候选，corrupt 受保护；不删除、不恢复、不修复 |
| `detect` | `--project`，可选 `--godot` | 零写入、不启动 Godot | 发现项目、Godot 候选和 Tier 1 宿主 |
| `doctor` | 同上，加 `--timeout-ms`；可选 `--export` | 零文件写入；只执行固定的 `Godot --version` | 检查宿主、项目文件、GDScript 范围、可执行文件和自报的 `4.7.2-stable` 标准版标识；`--export` 还要求匹配版本及当前宿主的 bounded export-template 文件；不以版本文本替代二进制来源验证 |
| `initialize` | `--project` | 首次只原子建立 `.gameatelier/project.json` 与持久 advisory lock 文件 | CSPRNG 项目身份、revision 0、standard mode；合法重跑零修改 |
| `logs` | `--project`、必选 strict `--run-id` | 零写入、零 evidence、不启动 Godot | 同次有界读取并验证一个 committed validate/test 闭包；只输出 ID/outcome/level、时间、退出码和 evidence integrity metadata，不输出 source 自由文本、payload 路径或 raw stdout/stderr |
| `status` | `--project` | 零写入 | 严格读取 `.gameatelier/project.json`，不跟随引用、不修复或迁移 |
| `test` | `--project`；可选 `--godot`、`--timeout-ms`、`--allow-engine-user-data` | 写自包含 immutable run/evidence；启动 Godot 前必须授权标准 `user://` | 固定执行 `res://tests/atelier_test_runner.gd`，严格解析唯一 JSON marker，把逐项 PASS/FAIL、超时、取消、引擎错误和无效报告映射为稳定结果；不接受任意脚本或 Godot 参数 |
| `validate` | `--project`；可选 `--headless`、`--godot`、`--timeout-ms`、`--allow-engine-user-data` | 默认只写自包含 immutable run/evidence；Headless 还需明确授权 Godot 标准 `user://` | 默认静态 baseline；显式 Headless 通过 pinned 项目目录，以及阶段独立的 runner/engine version/scene 快照，固定验证自报版本、主场景一帧、退出状态和 bounded `ERROR:` 输出；瞬时文件清理失败时禁止发布 result |

所有子命令 stdout 只包含一个 command-result JSON。`clean --list`、`detect`、`doctor`、`initialize`、`logs`、`status` 的 `evidence` 为空；`validate`/`test` 成功完成事务时分别引用同一 run 内的一份严格 validation/test report。`clean --list` 最多扫描 512 个严格 run ID 目录、2,048 个闭包文件和 256 MiB 内容；非法目录结构以 `RUN_SCAN_UNSAFE`、累计预算耗尽以 `RUN_SCAN_LIMIT_EXCEEDED` 整体失败且不返回部分候选。`logs` 只读取显式 run 的四个闭包文件，总预算 12.25 MiB；读取和闭包验证使用同一内存快照，输出不含 source 自由文本。调用方取消/deadline 返回 `COMMAND_CANCELLED`/6；无隐式 wall-clock timeout，在 64 KiB 读取块间协作响应。活动 run 可能瞬时显示为 orphan 或 incomplete，因此候选只是预览；未来删除必须先让 writer/cleaner/recovery 实现同一 per-run 协调协议，再逐项锁内重验，详见 ADR 0010。Headless validate/test 未获用户数据授权时在 Godot 启动前提交 `BLOCKED` evidence；获授权时 intent 以 `godot:user-data:standard-os-location` 明示引擎标准外部写入，不落盘用户绝对路径。test command 固定 `test_runner`，result counts 必须与 report tests 一致；PASS 还要求 Godot exit 0、唯一有效报告和全部逐项 PASS。run root 前失败返回 `RUN_RECORDING_UNAVAILABLE`，无 intent orphan 返回 `RUN_PREPARE_FAILED`，intent 后/result 前失败返回 `RUN_COMMIT_FAILED`；三者都保留原命令 scope 且不冒充 committed result。result 已发布但最终 durability/cleanup 未确认时，stdout 和进程退出码保持与权威 result 完全一致，并在 stderr 输出固定警告；stdout 短写返回内部错误 8，不重写或重跑已提交 run。详见 ADR 0008、0009、0011、0012。

## 版本策略

第一版 schema 版本为 `1.0.0`。任何破坏性变化都需要 ADR、迁移预览、备份与回退；文件中的 `schema_version` 不能随 CLI 版本隐式改变。Phase 1 生产基线仍不等于 v1.0 长期兼容冻结。
