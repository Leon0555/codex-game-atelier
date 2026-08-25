# Codex Game Atelier：v1.0 架构基线

状态：Phase 0 已审阅通过；Phase 1 按 ADR 与 Godot 实证继续细化
日期：2026-08-24；更新：2026-08-25

## 1. 架构原则

- **Codex 负责智能，CLI 负责确定性。** 计划、分工、判断和综合由 Codex 原生代理完成；CLI 只处理状态、验证、门禁和 Godot 调用。
- **证据先于声明。** 每个关键命令产生结构化结果和可定位证据。
- **显式副作用。** 登录、安装、发布、Git hooks 和仓库外写入均不隐式发生。
- **Godot 实证抽象。** v1.0 只保留被 Godot 流程验证的公共契约，不为延期的引擎预建大量适配层。
- **恢复优先。** 所有权、任务状态、交接和运行结果落盘，进程退出不丢失关键上下文。

## 2. 逻辑分层

```text
Codex Plugin / Starter Template
        |
        +-- focused Skills ---------> Codex-native orchestration
        |                                  |
        +-- core Agents -------------------+-- bounded subagents
                                           +-- ownership / review / handoff
                                                   |
Deterministic CLI ---------------------------------+
        |
        +-- state + evidence store (.gameatelier/)
        +-- policy gates (manual / standard / strict)
        +-- Godot adapter
        +-- structured output + exit codes
                |
                +-- Godot editor/headless/export templates
                +-- CI and release verification
```

Plugin 是可安装分发入口，Starter Template 是可复制的项目起点；二者共享相同契约和最少工作流，不分别演化成两个产品。CLI 为二者提供确定性底座，也可以独立用于 CI 与高级自动化。

## 3. Codex 原生编排

### 3.1 候选核心 Agents

保持小而稳定，初步只定义四个责任角色：

- **Lead**：范围、任务拆分、所有权、交接与用户决策。
- **Godot Engineer**：Godot 项目、场景、资源、脚本、构建和导出。
- **QA/Recovery Engineer**：测试矩阵、路径/中断/恢复、证据完整性。
- **Read-only Auditor**：架构、安全、兼容性和发布终审；默认不能修改被审对象。

角色声明只描述能力与责任，不指定具体模型 ID。能力解析见 ADR 0001。

### 3.2 候选顶层 Skills

Phase 1 应通过实际任务验证最小集合，当前候选为：

- `gameatelier-init`：检查前置条件并初始化项目状态。
- `gameatelier-develop`：路由实现、测试和恢复协作。
- `gameatelier-verify`：验证、构建、导出与证据审阅。
- `gameatelier-release`：严格门禁与发布准备；不自行执行外部发布。

Skill 负责可复用工作流与触发边界；确定性逻辑下沉 CLI，不用大段提示复制实现规则。根据 OpenAI 官方文档，Skill 是聚焦工作流的作者格式，Plugin 是安装和分发一个或多个 Skills 的主要包装方式；Phase 1 仍需验证最终 Plugin 结构。

### 3.3 子代理生命周期

1. Lead 创建/读取文件化任务，定义验收、边界和唯一 owner。
2. 只对独立工作使用有界并行；默认最多三个子代理。
3. 实现代理只能修改自己的责任范围；评审代理保持只读。
4. 子代理回传结论和证据引用，不倾倒完整日志。
5. Lead 写入交接与任务状态，并决定继续、恢复、升级或请求用户选择。

不实现长期运行的后台 orchestrator；子代理生命周期由当前 Codex 会话管理。

## 4. 确定性 CLI

候选公共命令中，`detect`、`doctor`、`status` 已按 ADR 0006 建立首个生产实现，`initialize` 已按 ADR 0007 建立第二个生产实现；其余仍是后续候选：

- `detect`：发现 Godot 与项目，纯读。
- `doctor`：当前生产切片纯读验证宿主、项目文件、GDScript、Godot 可执行文件和精确版本；导出模板与更完整的平台/配置诊断属于后续实现。
- `initialize`：用户显式请求时，为已有 Godot/GDScript 项目原子创建最小状态；合法重跑零修改，不写 evidence、不修复或覆盖异常状态。
- `validate`：场景、资源、脚本、配置和门禁检查。
- `test`：执行 GDScript/项目测试并记录结果。
- `build --profile debug|release`：面向用户的默认目标工作流，执行相应门禁并复用 `export` 产生 runnable artifact；底层 evidence 只记录一次并互相引用，不假设 Godot 存在独立编译流水线。
- `export`：对指定 Godot preset/目标执行直接导出与产物验证，是 Godot `--export-debug/--export-release` 的确定性包装。
- `logs`：归一化和查询运行日志。
- `status`：当前生产切片只读加载严格的项目状态摘要和引用计数，不跟随任务或 evidence 引用；任务、所有权、门禁和最近 evidence 聚合属于后续实现。
- `clean`：默认只列出可清理内容；实际删除需要显式范围与确认语义。
- `release check/prepare`：框架级编排命令，聚合完整测试、build/export 矩阵、分发和审计 evidence；它不是引擎适配器能力，且不自行执行 npm/GitHub/Marketplace 外部发布。

公共 CLI 契约必须为每个命令定义输入、JSON 输出、稳定退出码、超时、取消、重试/幂等、副作用与证据。引擎适配器契约不包含框架级 `release`；公开命令变化需要 ADR。

## 5. 文件化状态和证据

候选项目内结构（根目录 `.gameatelier/` 已确定，其余写入与恢复语义尚未冻结）：

```text
.gameatelier/
├── project.json              # schema version, engine, policy mode
├── tasks/                    # task, owner, acceptance, status
├── handoffs/                 # bounded recovery summaries
├── runs/                     # immutable run records
├── evidence/                 # logs, summaries, hashes, artifact manifests
├── locks/                    # short-lived ownership/operation leases
└── cache/                    # disposable derived data
```

原则：

- `project.json`、任务和交接可审查；凭据永不写入。
- run/evidence 记录追加或内容寻址，失败证据不被覆盖。
- cache 可删除且不能成为唯一事实来源。
- 每条状态含 schema version；迁移必须可预览、备份和回退。
- 需要进一步决定哪些状态默认提交 Git，哪些仅保留本地/CI artifact。

默认持久化政策已经确定：`detect`/`doctor`/`status` 零写入，`initialize` 只写 project state；`validate`/`test`/`build`/`export`/`release check` 默认写最小可审计 run/evidence，manual/standard/strict 只改变门禁深度而不关闭记录。多文件正式提交点和崩溃恢复仍需下一薄切片实证。

## 6. 门禁模式

| 模式 | 交互定位 | 命令内建最低门禁 | Git hooks | CI 发布检查 |
| --- | --- | --- | --- | --- |
| `manual` | 高级用户显式控制 | 始终执行安全、输入、目标和必要前置检查；不会把 `validate` 责任推给用户 | 不安装；可显式选择 | 必选，不可绕过 |
| `standard` | 默认 | `build`、`export` 自动执行相应 doctor/validate/test 子集并检查证据新鲜度 | 不安装；可显式选择 | 必选 |
| `strict` | 发布准备/高保障 | 更完整测试、干净状态/环境策略、证据完整性和失败即停 | 不安装；可显式选择 | 必选且使用完整发布矩阵 |

门禁是命令语义，不是单独 `validate` 命令的约定。可选 Git hooks 只提供早期反馈；即使未安装或用 `--no-verify` 绕过，CLI 和 CI 仍守住相应边界。

## 7. Godot 适配器

Godot 适配器是 v1.0 唯一生产适配器，负责：

- 可执行文件、版本、宿主架构与 export templates 检测。
- `project.godot`、场景、资源和导出预设识别。
- Headless 启动、脚本/场景加载、测试、构建和导出调用。
- Godot 日志归一化、超时、异常退出、残留/锁状态诊断。
- 中文、空格、特殊路径和干净环境验证。

适配器不安装 Godot，不管理账号，不隐藏下载模板，不把任意脚本求值作为通用接口。

## 8. 分发与运行时

推荐方向（见 ADR 0004）：

- 普通用户：Plugin 或 Starter Template，不需要 clone 源码或执行项目构建。
- CLI：生产实现语言为 Go；提供签名/校验的预构建跨平台产物。npm 包作为高级用户便利入口，包含已构建产物而非要求本机编译。
- Rust/Go 语言对照已完成并由用户于 2026-08-25 冻结 Go。Phase 1 继续验证 macOS arm64、Windows x64、Linux x64 的实际运行、Plugin 打包、evidence 和 CI；交叉编译文件形状不等于目标宿主支持。
- 发布使用受保护 tag、GitHub-hosted runner、OIDC Trusted Publishing、最小权限和 provenance；不用长期发布 Token。

## 9. 安全与隐私

- 默认无遥测；若未来提出遥测，必须另立决策、默认关闭并获用户批准。
- 所有外部写入、安装、登录和发布都需要显式动作/批准。
- 日志与证据进行 secret redaction；凭据文件不得进入仓库或一般 evidence。
- Git hooks 从不自动安装，安装前列出准确路径、行为和卸载方式。
- CLI 只允许已定义的 Godot/文件操作，不提供通用 `eval` 或任意 shell 代理接口。

## 10. 待 Phase 1 验证

- Plugin 能否可靠携带/定位各平台 CLI，以及最小安装步骤。
- Go CLI 的 Plugin 携带方式、包体积、目标宿主运行、升级/回滚路径。
- `.gameatelier` 中应提交与不应提交的精确边界。
- Godot 测试框架选择与“无额外依赖”降级路径。
- 构建、导出与 release 在三种模式下的精确门禁矩阵。

## 11. 当前依据

- [OpenAI 官方：Subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)
- [OpenAI 官方：Build skills](https://learn.chatgpt.com/docs/build-skills)
- [OpenAI 官方：Plugins](https://learn.chatgpt.com/docs/plugins)
- Godot 版本和平台依据见 [support-matrix.md](support-matrix.md)。
