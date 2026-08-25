# Phase 1 结构化契约

状态：Phase 1 生产基线；通用 envelope 和首批只读命令已实现，长期兼容仍等待完整薄切片验证。

## 目标

`schemas/v1/` 定义 CLI、状态和证据文件之间的最小边界。契约刻意不规定实现语言，不包含具体模型 ID，也不把 Godot 内部对象扩散到通用任务状态。

## 文件

- `command-result.schema.json`：一次确定性 CLI 调用的机器可读结果。
- `error.schema.json`：稳定错误码、类别、可重试性和修复提示。
- `evidence.schema.json`：日志、报告、构建或导出产物的可定位记录。
- `task.schema.json`：有界工作项、所有权、允许路径和生命周期。
- `handoff.schema.json`：可恢复交接，不承载隐藏推理过程。
- `project-state.schema.json`：项目模式、Godot 选择及任务/run 索引。
- `detect-data.schema.json`、`doctor-data.schema.json`、`initialize-data.schema.json`、`status-data.schema.json`：首批命令的精确 `data` 形状。
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
| `detect` | `--project`，可选 `--godot` | 零写入、不启动 Godot | 发现项目、Godot 候选和 Tier 1 宿主 |
| `doctor` | 同上，加 `--timeout-ms` | 零文件写入；只执行固定的 `Godot --version` | 检查宿主、项目文件、GDScript 范围、可执行文件和官方 `4.7.2-stable` 版本 |
| `initialize` | `--project` | 首次只原子建立 `.gameatelier/project.json` 与持久 advisory lock 文件 | CSPRNG 项目身份、revision 0、standard mode；合法重跑零修改 |
| `status` | `--project` | 零写入 | 严格读取 `.gameatelier/project.json`，不跟随引用、不修复或迁移 |

所有子命令 stdout 只包含一个 command-result JSON。当前已实现的四个命令中 `evidence` 必须为空；这表示多文件 evidence 写入尚未实现，而不是没有证据需求。用户已批准后续 `validate`、`test`、`build`、`export`、`release check` 默认持久化最小结构化 run/evidence；三种门禁模式只改变检查深度，不关闭关键证据。只读命令见 ADR 0006，初始化写入、锁和恢复边界见 ADR 0007，基础 evidence 政策见 ADR 0005。

## 版本策略

第一版 schema 版本为 `1.0.0`。任何破坏性变化都需要 ADR、迁移预览、备份与回退；文件中的 `schema_version` 不能随 CLI 版本隐式改变。Phase 1 生产基线仍不等于 v1.0 长期兼容冻结。
