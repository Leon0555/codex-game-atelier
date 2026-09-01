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
- `export-artifact.schema.json`：macOS 技术导出的 target/profile、Godot 版本、ZIP 摘要和非公开分发就绪声明。
- `capability-profile-catalog.schema.json`：四个逻辑 Profile、能力等级、权限/独立性语义，以及任务覆盖、用户映射、会话继承、主机默认的绑定优先级；不包含具体模型 ID。
- `gate-policy.schema.json`：`build`、`export`、`release-check` 在 `manual`、`standard`、`strict` 下的版本化、单调前置条件表。
- `release-check-data.schema.json`：只读发布聚合的所选模式、项目 revision、逐门禁结果、计数和严格发布就绪布尔值。
- `distribution-manifest.schema.json`：维护端本地 candidate 的单 Plugin archive、内含 CLI/runner/Starter 精确版本闭合、固定文件清单、许可和无隐式安装策略；它不是 CLI command result，也不表示已经外部发布。
- `starter-create-data.schema.json`：包内 Starter 创建结果；记录创建事实、模板版本、文件数/展开大小和显式的下一条初始化命令，不回显用户路径。
- `task.schema.json`：有界工作项、所有权、允许路径和生命周期。
- `handoff.schema.json`：可恢复交接，不承载隐藏推理过程。
- `project-state.schema.json`：项目模式、Godot 选择及任务/run 索引。
- `clean-data.schema.json`、`detect-data.schema.json`、`doctor-data.schema.json`、`export-data.schema.json`、`initialize-data.schema.json`、`logs-data.schema.json`、`release-check-data.schema.json`、`starter-create-data.schema.json`、`status-data.schema.json`、`test-data.schema.json`、`validate-data.schema.json`：已实现命令的精确 `data` 形状。
- `common.schema.json`：上述 schema 共用的窄定义。

## 结果与状态边界

- CLI 命令结果只使用 `PASS`、`FAIL`、`BLOCKED`、`SKIPPED`。
- 验收或证据还可使用 `NOT_RUN`，且不得把它当作通过。
- 任务状态独立使用 `planned`、`ready`、`active`、`review`、`verified`、`done`，以及 `blocked`、`failed`、`cancelled`。
- `summary` 面向人类；自动化必须读取字段、错误码和 evidence reference，不解析自然语言猜状态。
- `owner.logical_profile` 是可选的逻辑 Profile 引用，用于 task/handoff 恢复时重建责任和能力意图；它不记录或解析具体模型。旧状态省略该字段仍然有效。

Plugin bundle 必须带上 `common`、`error`、`task`、`handoff`、`evidence` 的版本化 Schema 闭包，使已安装 Skill 可以在没有源码 checkout 时先验证恢复状态。打包验证只允许闭包内的本地 `$ref`，并拒绝缺失文件、越界引用和无法解析的 JSON Pointer。

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
| `build` | `--project`；`--profile debug|release`；可选 `--mode manual|standard|strict`、`--godot`、`--timeout-ms`、`--allow-engine-user-data` | 与 `export` 相同；不开放 preset 参数 | 固定选择 `macOS Technical` 并复用同一 export 执行/evidence；standard 在同一总 timeout 内先执行 Headless 与固定 tests；当前只启用 macOS Apple Silicon |
| `clean --list` | 必选 `--list`；可选 `--project` | 零写入、零 evidence、不创建 lock | 有界扫描 run store；完整验证 committed 闭包，只把 incomplete/orphan 列为预览候选，corrupt 受保护；不删除、不恢复、不修复 |
| `detect` | `--project`，可选 `--godot` | 零写入、不启动 Godot | 发现项目、Godot 候选和 Tier 1 宿主 |
| `doctor` | 同上，加 `--timeout-ms`；可选 `--export` | 零文件写入；只执行固定的 `Godot --version` | 检查宿主、项目文件、GDScript 范围、可执行文件和自报的 `4.7.2-stable` 标准版标识；`--export` 还要求匹配版本及当前宿主的 bounded export-template 文件；不以版本文本替代二进制来源验证 |
| `export` | `--project`；`--profile debug|release`；固定 `--preset "macOS Technical"`；可选 `--mode manual|standard|strict`、`--godot`、`--timeout-ms`、`--allow-engine-user-data` | 写 immutable run/evidence 与一个 ZIP artifact；standard/strict 的嵌套 gate 各自写独立事实；获准时使用 Godot 标准 `user://` | manual 只省略 workflow 扩展；standard 先 Headless/test、失败即停；strict 先完成 standard 再对 M3 未实现项阻断；真正导出只在有界项目快照上运行，验证 Universal 2 ZIP 与 Apple Silicon target smoke |
| `hooks list/plan/status` | `--project` | 零写入、零 evidence、不创建 hooks 目录 | 只支持项目根的普通 `.git` 目录与默认 hooks path；列出固定 `pre-commit`、ownership manifest、manual release check 和 absent/installed/stale/conflict 状态 |
| `hooks install/uninstall` | `--project`；子命令本身就是显式动作 | 只写/删 `.git/hooks/pre-commit` 与 `.git/hooks/codex-game-atelier.manifest.json` | 从不覆盖已有 hook；install 绑定当前 CLI 的 `release check --mode manual`；uninstall 只删除内容摘要仍匹配 manifest 且带 ownership marker 的 managed hook；不支持 `core.hooksPath` 与 linked worktree `.git` 文件 |
| `initialize` | `--project` | 首次只原子建立 `.gameatelier/project.json` 与持久 advisory lock 文件 | CSPRNG 项目身份、revision 0、standard mode；合法重跑零修改 |
| `logs` | `--project`、必选 strict `--run-id` | 零写入、零 evidence、不启动 Godot | 同次有界读取并验证一个 committed validate/test 闭包；只输出 ID/outcome/level、时间、退出码和 evidence integrity metadata，不输出 source 自由文本、payload 路径或 raw stdout/stderr |
| `release check` | `--project`；可选 `--mode manual|standard|strict`，省略时读取项目 mode；`--distribution-candidate` 只允许与显式 strict 一起使用 | 零写入、零 evidence、不启动 Godot、不执行 candidate 代码 | manual 检查状态与支持范围；standard 还要求完整 run store 和当前 revision 的最新 headless/test/release-export 均 PASS，并重新验证 ZIP 摘要与 Universal 2 形状；strict 可在内存中有界验证单 Plugin candidate、包内 Starter、notices 与六文件/八架构 Go provenance；未提供时四项 `NOT_RUN`，失败时四项 `BLOCKED`；required CI 与远程 Plugin 无阻断安装在证据接入前保持 `NOT_RUN`，manual/standard 永不返回 `release_ready: true` |
| `starter create` | 必选 `--project <new-directory>`；父目录必须已存在，目标必须不存在 | 只创建目标项目目录；不写 evidence、不初始化 Atelier/Git、不启动 Godot、不联网、不写用户级 Codex 状态 | 在已验证 macOS Apple Silicon 上从当前 Plugin 根定位 embedded Starter，严格验证 package manifest、Plugin/Starter 版本配对、固定 inventory/hash/mode/大小后，私有 staging 并以 no-replace 原子发布；输出只记录路径为 `provided`；Windows/Linux 原生发布语义验证前阻断 |
| `status` | `--project` | 零写入 | 严格读取 `.gameatelier/project.json`，不跟随引用、不修复或迁移 |
| `test` | `--project`；可选 `--godot`、`--timeout-ms`、`--allow-engine-user-data` | 写自包含 immutable run/evidence；启动 Godot 前必须授权标准 `user://` | 固定执行 `res://tests/atelier_test_runner.gd`，严格解析唯一 JSON marker，把逐项 PASS/FAIL、超时、取消、引擎错误和无效报告映射为稳定结果；不接受任意脚本或 Godot 参数 |
| `validate` | `--project`；可选 `--headless`、`--godot`、`--timeout-ms`、`--allow-engine-user-data` | 默认只写自包含 immutable run/evidence；Headless 还需明确授权 Godot 标准 `user://` | 默认静态 baseline；显式 Headless 通过 pinned 项目目录，以及阶段独立的 runner/engine version/scene 快照，固定验证自报版本、主场景一帧、退出状态和 bounded `ERROR:` 输出；瞬时文件清理失败时禁止发布 result |

所有子命令 stdout 只包含一个 command-result JSON。`clean --list`、`detect`、`doctor`、`initialize`、`logs`、`release check`、`status` 的 `evidence` 为空；`validate`/`test` 成功完成事务时分别引用同一 run 内的一份严格 validation/test report，`build`/`export` 引用同一形状的 `export-artifact` manifest。build/export 的显式 `--mode` 是单次覆盖：原命令及其内部 gate intent 记录覆盖值，`project.json` 不变。standard/strict 共用调用者给定的一个总 timeout，不把三段执行时间偷偷相加；workflow gate 失败会阻止真正导出，并让原命令与上游 gate 分别保留可验证闭包。`clean --list` 与 `release check` 最多扫描 512 个严格 run ID 目录、2,048 个闭包文件和 256 MiB 内容；非法目录结构以 `RUN_SCAN_UNSAFE`、累计预算耗尽以 `RUN_SCAN_LIMIT_EXCEEDED` 整体失败且不返回部分结论。`release check` 的“latest”按当前 revision 内同类命令的 `finished_at` 选择；较早 PASS 不能覆盖较新失败。它还重新打开并检查所选 release ZIP，artifact 缺失、篡改或不再满足固定 Universal 2 形状会阻断发布门禁。显式 strict candidate 输入只在 stdout 记录 `provided`，不回显用户绝对路径或 archive 自由文本；固定 `1.2.0` manifest、顶层 allowlist、hash/size/mode、tar/gzip metadata、外部 checksum、Plugin 内含 Starter inventory/版本、notice 和实际 Go build info 任一失败都会保守阻断四项本地分发 gate。该同源闭包不证明发布者身份、真实远程 Plugin 无阻断安装、远程 CI 或原生平台支持。`logs` 只读取显式 run 的四个闭包文件，总预算 12.25 MiB；读取和闭包验证使用同一内存快照，输出不含 source 自由文本。调用方取消/deadline 返回 `COMMAND_CANCELLED`/6；无隐式 wall-clock timeout，在 64 KiB 读取块间协作响应。活动 run 可能瞬时显示为 orphan 或 incomplete，因此候选只是预览；未来删除必须先让 writer/cleaner/recovery 实现同一 per-run 协调协议，再逐项锁内重验，详见 ADR 0010。执行 Godot 的命令未获用户数据授权时在引擎启动前提交 `BLOCKED` evidence；获授权时 intent 以 `godot:user-data:standard-os-location` 明示标准外部写入，不落盘用户绝对路径。`build`/`export` 另在 run 内创建并清理有界项目快照，Godot 不直接写源项目；该隔离不等于网络/绝对路径恶意代码沙箱。test command 固定 `test_runner`，result counts 必须与 report tests 一致；PASS 还要求 Godot exit 0、唯一有效报告和全部逐项 PASS。run root 前失败返回 `RUN_RECORDING_UNAVAILABLE`，无 intent orphan 返回 `RUN_PREPARE_FAILED`，intent 后/result 前失败返回 `RUN_COMMIT_FAILED`；三者都保留原命令 scope 且不冒充 committed result。result 已发布但最终 durability/cleanup 未确认时，stdout 和进程退出码保持与权威 result 完全一致，并在 stderr 输出固定警告；stdout 短写返回内部错误 8，不重写或重跑已提交 run。详见 ADR 0008、0009、0011、0012、0015、0016、0017、0022、0023。

Hooks 命令同样不写 `.gameatelier` evidence；只有显式 `hooks install/uninstall` 修改表中两个 Git 路径。build/export scanner 还必须验证原命令 `mode` 与 intent `policy_mode` 一致。standard/strict 的单一 operational deadline 覆盖自动 gates、项目快照、Godot、产物复验/复制和 target smoke；result/evidence 的最终 durability 收口在操作停止后仍必须完成，不能留下假 committed 状态。

`starter create` 同样不写 evidence。它只复制面向用户的 Starter 源文件与根 LICENSE/NOTICE，不复制 package manifest、Plugin、CLI/runner、Skill、内部 `AGENTS.md`、缓存或 `.gameatelier`；成功后调用方必须显式运行 `initialize`。其公共契约见 ADR 0024。

## 版本策略

第一版 schema 版本为 `1.0.0`。任何破坏性变化都需要 ADR、迁移预览、备份与回退；文件中的 `schema_version` 不能随 CLI 版本隐式改变。Phase 1 生产基线仍不等于 v1.0 长期兼容冻结。

维护端 `tools/package_distribution.py` 不属于最终用户 CLI。它只在已验证、已内含 Starter 的 Plugin bundle 之上创建或静态复验一个此前不存在的本地 candidate；不会安装 Plugin、修改 Codex 配置、联网或发布。Plugin 与 distribution manifest 必须记录由打包器从六个二进制文件、八个架构记录实测得到的 clean Git revision、精确 Go 版本、`-trimpath` 与 `CGO_ENABLED=0`；dirty 或来源不一致在创建输出前阻断。包含 Go 二进制的 Plugin/candidate 必须携带仓库 `THIRD_PARTY_NOTICES`。candidate 的 `local-candidate` 状态、远程 Plugin 门禁 `NOT_RUN` 以及 Windows/Linux 原生 `NOT_RUN` 不得被解释成严格发布通过；Apple 公证不是默认门禁。
