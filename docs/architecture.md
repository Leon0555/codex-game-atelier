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
Codex Plugin (includes Starter Template)
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

Plugin 是 v1.0 唯一用户安装入口。Starter Template 是 Plugin 内含的项目起点，公开 CLI 与私有 runner 也是 Plugin 内部组件；三者共享同一版本和完整性闭包，不分别作为用户下载产品。当前 GitHub-hosted CI 直接验证源码与本机构建，普通用户仍只通过 Plugin 取得工具；该 CI 不恢复独立 CLI 用户分发。

## 3. Codex 原生编排

### 3.1 候选核心 Agents

保持小而稳定，初步只定义四个责任角色：

- **Lead**：范围、任务拆分、所有权、交接与用户决策。
- **Godot Engineer**：Godot 项目、场景、资源、脚本、构建和导出。
- **QA/Recovery Engineer**：测试矩阵、路径/中断/恢复、证据完整性。
- **Read-only Auditor**：架构、安全、兼容性和发布终审；默认不能修改被审对象。

角色声明只描述能力与责任，不指定具体模型 ID。分发 Skill 使用 `lead`、`implementation`、`fast-read`、`independent-audit` 四个逻辑 Profile；绑定优先级和阻断语义由随 Skill 分发的能力目录定义，具体绑定留在用户/Codex 主机环境。能力解析见 ADR 0001。

Plugin 同时分发有界原生协作参考及 `common`、`error`、`task`、`handoff`、`evidence` 五份最小 Schema 闭包。新代理必须先验证文件化状态再恢复工作；写 ownership 对目录重叠、符号链接、同对象、大小写和 Unicode 别名采取保守阻断。该机制是 Codex 原生协作约定，不是后台调度器或 CLI 模型路由。

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

公共命令中，`detect`、`doctor`、`status` 已按 ADR 0006 建立首个生产实现，`initialize` 已按 ADR 0007 建立第二个生产实现，`validate` 与 run/evidence 事务已按 ADR 0008、0009 覆盖静态 baseline 和明示授权的 Godot Headless 薄切片，`clean --list` 已按 ADR 0010 建立只读 scanner，`test` 已按 ADR 0011 建立固定 GDScript 协议，`logs` 已按 ADR 0012 建立 committed run 的零自由文本结构投影，`starter create` 已按 ADR 0024 建立 embedded Starter 的安全创建路径：

- `detect`：发现 Godot 与项目，纯读。
- `doctor`：当前生产切片纯读验证宿主、项目文件、GDScript、Godot 可执行文件和自报的精确标准版标识；版本文本不替代安装来源、散列或签名验证。导出模板与更完整的平台/配置诊断属于后续实现。
- `initialize`：用户显式请求时，为已有 Godot/GDScript 项目原子创建最小状态；合法重跑零修改，不写 evidence、不修复或覆盖异常状态。
- `starter create`：从当前 Plugin 根内严格验证 embedded Starter 的固定 inventory、hash、mode 与版本配对，向一个不存在的新目录私有 staging 后 no-replace 原子发布；不复制 package-only 文件，不初始化状态/Git，不运行 Godot，不联网，不写用户级 Codex 状态。
- `validate`：默认验证 pinned 项目状态、regular `project.godot`、GDScript 边界和持久化能力；显式 `--headless` 在用户授权标准 `user://` 后，固定配套 runner 与 Godot 的已打开源文件身份，并为 version/scene 分别创建阶段独立的 runner/engine 快照，通过继承的 pinned 项目目录 fd 执行固定验证，再把外部写入符号化记录在 intent。项目公开路径身份在引擎前后核对，路径被并发替换时 observation 作废；Godot/runner 公共路径被替换不会重定向已固定执行。任何瞬时文件清理失败都会阻止 `result.json` 发布。完整场景/资源图和日志保留仍属后续切片。
- `test`：当前固定执行 `res://tests/atelier_test_runner.gd`，不接受任意脚本/Godot 参数；复用 pinned version/test 双阶段执行，严格解析唯一 JSON marker，把断言失败映射为 3、引擎/协议失败映射为 5、超时/取消映射为 6，并原子记录逐项 test report。它执行用户拥有或已审阅的项目代码，不声称提供代码沙箱；第三方测试框架适配、过滤和原始日志保留属于后续切片。
- `build --profile debug|release`：面向用户的默认目标工作流，执行相应门禁并复用 `export` 产生 runnable artifact；底层 evidence 只记录一次并互相引用，不假设 Godot 存在独立编译流水线。
- `export`：对指定 Godot preset/目标执行直接导出与产物验证，是 Godot `--export-debug/--export-release` 的确定性包装。
- `logs`：当前要求显式 strict run ID，只从同次有界读取中验证 committed closure，输出由 CLI 编号的 check/test/error/result allow-list 结构事件与 evidence hash/size，不输出 source ID、error code、自由文本或 raw stdout/stderr；原始日志保留、脱敏、分片和 follow/tail 需要新的隐私与存储决策。
- `status`：当前生产切片只读加载严格的项目状态摘要和引用计数，不跟随任务或 evidence 引用；任务、所有权、门禁和最近 evidence 聚合属于后续实现。
- `clean`：当前只实现显式 `--list`，以 512 目录/2,048 文件/256 MiB 总预算和调用方 context 有界验证 committed 闭包，只把瞬时也可能是活动 writer 的 incomplete/orphan 列入预览，corrupt 受保护；实际删除必须先让 writer/cleaner/recovery 建立同一 per-run 协调协议，再实现精确范围、锁内重验与确认语义。
- `release check/prepare`：框架级编排命令，聚合完整测试、build/export 矩阵、分发和审计 evidence；它不是引擎适配器能力，且不自行执行 npm/GitHub/Marketplace 外部发布。

公共 CLI 契约必须为每个命令定义输入、JSON 输出、稳定退出码、超时、取消、重试/幂等、副作用与证据。引擎适配器契约不包含框架级 `release`；公开命令变化需要 ADR。

## 5. 文件化状态和证据

候选项目内结构（根目录 `.gameatelier/` 已确定，其余写入与恢复语义尚未冻结）：

```text
.gameatelier/
├── project.json              # schema version, engine, policy mode
├── tasks/                    # task, owner, acceptance, status
├── handoffs/                 # bounded recovery summaries
├── runs/<run-id>/            # self-contained immutable run closure
│   ├── intent.json
│   ├── payloads/             # bounded reports/logs/manifests
│   ├── evidence/             # hashes, sizes, producer and payload refs
│   └── result.json           # published last; only logical commit point
├── locks/                    # short-lived ownership/operation leases
└── cache/                    # disposable derived data
```

原则：

- `project.json`、任务和交接可审查；凭据永不写入。
- run/evidence 记录追加或内容寻址，失败证据不被覆盖。
- cache 可删除且不能成为唯一事实来源。
- 每条状态含 schema version；迁移必须可预览、备份和回退。
- 需要进一步决定哪些状态默认提交 Git，哪些仅保留本地/CI artifact。

默认持久化政策已经确定：`clean --list`/`detect`/`doctor`/`release check`/`status` 零写入，`initialize` 只写 project state；`validate`/`test`/`build`/`export` 默认写最小可审计 run/evidence，manual/standard/strict 只改变门禁深度而不关闭这些生产命令的记录。`release check` 只聚合已经持久化的事实并重新验证当前 release artifact，不用一条自引用的审计记录改变被审计状态。`validate`、`test` 与 build/export 已实证以 `result.json` 最后发布的多文件正式提交点；run scanner 能验证四类 committed 闭包并预览 incomplete/orphan，实际 recovery/delete 与派生索引仍待后续薄切片。详见 ADR 0016。

## 6. 门禁模式

| 模式 | 交互定位 | 命令内建最低门禁 | Git hooks | CI 发布检查 |
| --- | --- | --- | --- | --- |
| `manual` | 高级用户显式控制 | 始终执行安全、输入、目标和必要前置检查；不会把 `validate` 责任推给用户 | 不安装；可显式选择 | 必选，不可绕过 |
| `standard` | 默认 | `build`、`export` 自动执行 Headless 与固定 GDScript tests；失败即停止真正导出 | 不安装；可显式选择 | 必选 |
| `strict` | 发布准备/高保障 | 先执行完整 standard 子集；M3 的 run-store/source/distribution 门禁未实现前明确阻断 | 不安装；可显式选择 | 必选且使用完整发布矩阵 |

门禁是命令语义，不是单独 `validate` 命令的约定。可选 Git hooks 只提供早期反馈；即使未安装或用 `--no-verify` 绕过，CLI 和 CI 仍守住相应边界。

build/export 默认读取 `project.json` 的 mode，也允许 `--mode` 只覆盖当前调用。覆盖值记录进本次原命令与嵌套 gate 的 run intent，不改写 project state；项目级 mode 的持久修改要等 state replacement/migration 协议单独冻结，不能由普通构建暗中完成。

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

- 普通用户：只安装 Codex Plugin，不需要 clone 源码、单独下载 Starter/CLI 或执行项目构建。
- Plugin 闭包：Go 公开 CLI、sibling 私有 runner、Starter Template、Skills、Schemas、LICENSE、NOTICE 与第三方声明绑定到同一精确版本。`darwin-universal2`、`linux-amd64`、`windows-amd64` 仍是包内固定目录；当前只有 Apple Silicon 完成原生执行验证。
- macOS 发布门禁：不预先实施 Developer ID 签名或 Apple 公证。门禁改为从真实远程 Plugin 来源在干净 Apple Silicon 环境安装，且无需系统设置放行、`xattr` 清除或其他隐藏策略修改即可调用包内 CLI/runner 并完成真实 Godot 工作流。若该实测失败，公证仅作为需新 ADR 和用户批准的备选方案。
- v1.0 不发布独立 CLI archive、GitHub Release 二进制 ZIP、npm CLI 包、Homebrew、DMG 或 PKG。历史 `0.2.0` 双 archive 本地候选只保留为证据，不代表未来分发形状。
- Rust/Go 语言对照已完成并由用户于 2026-08-25 冻结 Go。Phase 1 只把 macOS Apple Silicon 作为当前原生验证宿主；Windows/Linux 仅保留交叉构建 artifact 形状，原生 runner 已按用户决定延期，不能因此扩大支持声明。
- 首个受验证远程来源是 `https://github.com/Leon0555/codex-game-atelier` 的固定 Marketplace ref；当前 hosted CI 已 PASS。正式 Plugin 发布仍必须使用受保护 tag、最小权限和可审计 provenance，不用长期发布 Token。

## 9. 安全与隐私

- 默认无遥测；若未来提出遥测，必须另立决策、默认关闭并获用户批准。
- 所有外部写入、安装、登录和发布都需要显式动作/批准。
- 日志与证据进行 secret redaction；凭据文件不得进入仓库或一般 evidence。
- Git hooks 从不自动安装，安装前列出准确路径、行为和卸载方式。
- 当前可选 hook 只在显式 `hooks install` 时写默认 `.git/hooks/pre-commit` 与 ownership manifest；不合并/覆盖现有 hook，不支持自定义 `core.hooksPath` 或 linked-worktree `.git` 文件。它只运行当前 CLI 的 manual release check，不能替代 build/export 内建门禁或 CI。
- CLI 只允许已定义的 Godot/文件操作，不提供通用 `eval` 或任意 shell 代理接口。

## 10. 待 Phase 1 验证

- `0.3.0-rc.1` 单 Plugin 候选已从 clean `969bef0...` 生成并逐字节重现；远程 Git-backed 安装的 cache 又与该候选逐文件一致，包内 CLI/runner、特殊路径 Starter、Headless validate 和固定 GDScript 6/6 已 PASS。真实用户级升级、失败升级不替换 active version、回滚、卸载与精确状态恢复也已 PASS。
- Go CLI 的当前本地 Plugin archive 约 12 MiB；Apple Silicon 已通过本地与远程 Plugin 入口。Linux/Windows 原生运行仍属 v1 不支持范围；远程安装态的新任务 Skill 发现与全新用户/机器复验仍待验证。
- `.gameatelier` 中应提交与不应提交的精确边界。
- 第三方 Godot 测试框架适配、测试过滤、异步 fixture 和固定零依赖协议的升级路径。
- strict `release check` 的本地 `1.2.0` contract 已迁移为单 Plugin archive；外部 `1.0.0` release evidence contract 又以单一只读输入绑定 candidate version/source revision/manifest/archive hash、macOS Apple Silicon 远程 Plugin 观察和固定 GitHub-hosted required CI。输入不会联网或自证发布者身份；真实 `rc.2` 证据、branch protection 和完整 strict PASS 仍须在当前实现提交后生成。

## 11. 当前依据

- [OpenAI 官方：Subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)
- [OpenAI 官方：Build skills](https://learn.chatgpt.com/docs/build-skills)
- [OpenAI 官方：Plugins](https://learn.chatgpt.com/docs/plugins)
- Godot 版本和平台依据见 [support-matrix.md](support-matrix.md)。
