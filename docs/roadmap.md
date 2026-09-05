# Codex Game Atelier 路线图

状态：Phase 0 已审阅通过；M1/M2 本地实现已完成，当前进入 M3
更新日期：2026-09-01

## 1. 已完成基线

Phase 0 已于 2026-08-24 通过用户审阅，确立了 Godot-only v1.0、Go CLI、MIT License、文件化状态与证据；2026-08-31 又通过 ADR 0023 把 v1 用户分发收敛为单一 Codex Plugin，Starter Template 与 CLI/runner 均内含其中。

Phase 1 已完成的生产薄切片包括：

- `detect`、`doctor`、`status` 与幂等 `initialize`。
- 静态及 Godot Headless `validate`、固定零依赖 GDScript `test`。
- 原子 run evidence、结构化 `logs`、有界 run scanner 与只读 `clean --list`。
- 可复现的多宿主预构建 Plugin bundle；当前只有 macOS Apple Silicon 完成原生执行验证。
- 干净 Starter Template 及其独立 archive/checksum 历史验证；下一候选将其作为 Plugin 内含能力，不再单独面向用户发布。

这些结果证明了当前垂直骨架，不自动等于 v1.0 发布门禁已经通过。

## 2. 三里程碑实施路线

后续不再按“核心、适配器、参考游戏、分发、发布”串行铺开，而按三个可运行的垂直里程碑推进。每个里程碑都必须产生实际命令、产物和 evidence。

### M1：可运行、可导出（已完成）

目标：在 macOS Apple Silicon 上完成从模板到可启动 Godot 游戏产物的闭环。

实施顺序：

1. **已完成**：`doctor` 检查并冻结 Godot 版本匹配的 export templates。
2. **已完成**：指定 preset/profile 的 Debug/Release `export`，记录超时、退出码、产物 manifest/hash 和分发就绪限制；Universal 2 由实际 Mach-O slices 验证。
3. **已完成**：`build --profile debug|release` 薄封装；复用同一 export 执行和 evidence，不制造第二条 Godot 流水线。
4. **已完成**：Starter Template 与参考项目扩展为 Atelier Spark 小型完整玩法，包含输入、signals、中文资源、UI、胜利与重置循环及六项固定测试。
5. **已完成**：在中文、空格和 `#` 项目根闭合 `detect → doctor → initialize → validate → test → Debug build → Release export → target smoke → clean --list`。

前三项的底层导出证据见 [`m1-macos-export-build-2026-08-27.md`](validation/m1-macos-export-build-2026-08-27.md)；最终玩法和特殊路径全链见 [`m1-playable-vertical-slice-2026-08-28.md`](validation/m1-playable-vertical-slice-2026-08-28.md)。M1 本地退出条件已满足，但这不替代 M3 干净用户环境和最终发布审计。

M1 不做：Windows/Linux 原生运行、签名/公证、商店发布、完整场景图解析器、第三方测试框架适配、raw 日志系统。

退出条件：Apple Silicon 上 Debug 与 Release 技术导出均有可复现 PASS evidence，产物可启动并正常退出；未验证范围被明确标记。

### M2：可协作、可管控（本地实现已完成）

目标：用最小机制证明 Codex 原生协作与发布前门禁，不建设通用编排或策略平台。

实施顺序：

1. **已完成**：用逻辑能力 Profile 表达能力等级、会话继承和用户覆盖；分发内容不含具体模型 ID。Plugin 内目录、公共 Schema、九项解析矩阵、打包门禁与 task/handoff 可选引用均已实证。
2. **已完成**：用一次真实有界子代理工作流验证单一 owner、只读审计和 task/handoff/evidence 恢复；实现代理和两轮审计代理均从文件恢复、无对话继承，首轮 FAIL 后由实现 owner 修复、全新只读审计 PASS；未实现常驻服务。
3. **已完成（M3 门禁按阶段阻断）**：随 Plugin 分发的前置条件表冻结 `manual < standard < strict`；build/export 默认读取项目 mode，也可单次覆盖。standard 自动执行 Headless/test 且失败即停，strict 完成 standard 子集后对尚未实现的 M3 run-store/source/distribution 项明确阻断；没有把 `NOT_RUN` 冒充通过。
4. **已完成实现、候选重验中**：实现只读 `release check`；manual/standard 不冒充严格发布就绪，strict 在内存中验证本地 candidate 与绑定 external evidence，不执行包内代码、不回显绝对路径。rc.2 曾取得 12/12 PASS，但最终审计发现候选运行时支持范围冲突和外部记录过薄；external evidence 已升级为 1.1.0，须由 rc.3 重新取得完整 PASS。
5. **已完成（本地）**：提供一个显式安装、可列出、可卸载的轻量 `pre-commit` hook；不自动安装、不覆盖既有 hook，CLI 与 CI 门禁不依赖它。
6. **已完成托管与强制门禁**：单一 `macos-15` Apple Silicon CI job 以固定只读权限与 action SHA 完成 Go 1.24 最低版本、Python/Schema、Plugin/Template 静态完整性、artifact-only 交叉构建和本机 CLI pair smoke。候选源码 run `33521593327` 全部 PASS；GitHub `main` 已将 `verify-macos-arm64` 配为 strict required check，并对管理员生效，禁止 force push 和 branch deletion。

M2 不做：隐藏规划器、常驻多代理服务、通用策略引擎、完整任务数据库、派生索引、任意代码执行、自动安装 hooks。

退出条件：三种模式的安全边界、模型抽象、一次可恢复原生协作、hook 可选性及 CI 不可绕过性均有正反例证据。

### M3：可安装、可发布验证

目标：集中完成一次真实生命周期和 v1.0 发布审计，避免在每个功能切片重复安装演练。

实施顺序：

1. **rc.2 已拒绝，rc.3 修复中**：rc.2 A/B、本地 Godot 与分发门禁本身通过，但最终审计发现 CLI host 判定违反 macOS-only ADR。代码已改为仅 `darwin/arm64` supported，并增加 macOS/Intel/Windows/Linux 矩阵测试；须在修复合并后从 clean revision 重建 rc.3。
2. **rc.2 历史生命周期已完成，rc.3 须重跑**：rc.2 的远程安装、失败升级保持 active、回滚、新任务 Skill 发现和精确状态恢复都是真实证据，但不能绑定到修复后的二进制。rc.3 必须以 rc.2 为 previous version 重跑同一闭环。
3. **当前机器无阻断安装结论仍有效但须绑定 rc.3**：rc.2 没有 quarantine 或系统设置绕过，Apple 公证继续不是 v1 门禁。全新用户或第二台 Apple Silicon 机器复验按用户决定延后到最终 RC。
4. **主分支强制已完成，external evidence 加固中**：`main` required CI 已启用；1.1.0 证据新增生命周期操作/退出码、前后状态、Codex CLI/Skill 观察和 branch-protection 快照。rc.3 strict PASS、最终终审、版本 ref、Plugin 发布与用户批准仍待完成。rc.2 失败详情见 [`m3-rc2-final-readonly-audit-2026-09-02.md`](validation/m3-rc2-final-readonly-audit-2026-09-02.md)。
5. 冻结 Support Matrix，完成架构、安全、许可证、性能、文档和分发的独立只读终审。
6. 其余门禁通过后再请求用户批准正式 Plugin 发布；v1 不执行 npm publish 或独立二进制 GitHub Release。

M3 不做：Godot 游戏产物签名/公证、框架预防性 Apple 公证、自动账号登录、长期发布 Token、未经授权的远程写入。

退出条件：冻结范围内所有必选发布门禁为 PASS，Blocker/High 为零，用户明确批准发布。

## 3. 明确延后但不隐藏的项目

以下项目不进入当前 M1/M2 日常开发路径：

- Windows x64 与 Linux x64 原生 runner/机器验证。
- 完整 schema migration 引擎、派生索引和通用恢复数据库。
- `clean` 的实际删除能力；当前保持只读 `clean --list`。
- 第三方 GDScript 测试框架、过滤/异步 fixture 平台。
- raw 日志捕获、follow/tail、分片和通用脱敏流水线。
- 多套顶层 Skills 或为每种责任创建独立常驻 Agent。
- npm 包装实现、独立 CLI archive、DMG/PKG 与预防性 Apple 公证。

这些延后项如果改变已冻结产品承诺，必须在实施前重新决策；不能因为未排入当前开发循环就被描述成已经支持。

## 4. Windows/Linux 决策

用户于 2026-09-01 批准 ADR 0025：v1.0 生产级承诺只包含 macOS Apple Silicon。Windows/Linux 保留交叉构建 artifact 作为未来工程输入，但在 v1 中明确不支持、不执行、不宣传，也不进入发布门禁。未来纳入任一宿主必须新立 ADR 并补齐原生矩阵。

## 5. 后续版本

Unity、Godot .NET/C#、Web/移动/主机导出和签名/公证均不属于 v1.0。Unity 只有在 v1.0 稳定后才能重新立项；必须先做官方 CLI、许可证和平台 Spike，且 `unity eval` 默认关闭。
