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
4. **已完成（required CI 保持阶段阻断）**：实现只读 `release check`；manual/standard 不冒充严格发布就绪，standard 聚合当前 revision 最新 evidence 并复验 Release ZIP；strict 可显式读取本地 candidate，在内存中关闭 clean-source、Plugin、Starter、license/provenance 四项，不执行包内代码、不回显绝对路径；required CI 在托管证据可用前继续 `NOT_RUN`。不实现自动外部发布，也不增加独立 `release prepare` 流水线。
5. **已完成（本地）**：提供一个显式安装、可列出、可卸载的轻量 `pre-commit` hook；不自动安装、不覆盖既有 hook，CLI 与 CI 门禁不依赖它。
6. **已完成托管运行**：单一 `macos-15` Apple Silicon CI job 以固定只读权限与 action SHA 完成 Go 1.24 最低版本、Python/Schema、Plugin/Template 静态完整性、artifact-only 交叉构建和本机 CLI pair smoke。首次 run 暴露并修复 Go 1.24 `os.Root.Name()` 测试假失败；修复后 run `33515728377` 全部 PASS。branch protection required check 尚未配置。

M2 不做：隐藏规划器、常驻多代理服务、通用策略引擎、完整任务数据库、派生索引、任意代码执行、自动安装 hooks。

退出条件：三种模式的安全边界、模型抽象、一次可恢复原生协作、hook 可选性及 CI 不可绕过性均有正反例证据。

### M3：可安装、可发布验证

目标：集中完成一次真实生命周期和 v1.0 发布审计，避免在每个功能切片重复安装演练。

实施顺序：

1. **已完成（单 Plugin 本地候选）**：历史 `0.2.0` 双 archive 候选继续只作证据。`0.3.0-rc.1` 已从 clean `969bef0...` 生成 `1.2.0` 单 Plugin candidate；A/B 逐字节一致，可信本机 CLI/runner smoke、特殊路径 `starter create → initialize → status` 和 strict clean-source/Plugin/embedded-Starter/license-provenance 四项均 PASS。详见 [`m3-plugin-only-candidate-2026-09-01.md`](validation/m3-plugin-only-candidate-2026-09-01.md)。
2. **已完成真实远程生命周期**：`0.3.0-rc.1` 从 GitHub Marketplace ref 安装到隔离 `CODEX_HOME`，cache 与本地审计候选逐文件一致，包内 CLI/runner 与特殊路径 Starter/Headless/6-test 工作流 PASS。真实 `~/.codex` 又完成 `rc.0 → rc.1`、无效候选不替换 active `rc.1`、`rc.1 → rc.0`、卸载和精确状态恢复。详见 [`m3-remote-plugin-lifecycle-2026-09-01.md`](validation/m3-remote-plugin-lifecycle-2026-09-01.md)。
3. **进行中（远程无阻断安装与 Godot E2E 已 PASS）**：Git-backed 远程 Plugin 没有 quarantine，不需系统设置放行、`xattr` 或隐藏策略修改；因此 Apple 公证继续不是 v1 门禁。仍待从远程安装态新建 Codex 任务并由主机自动发现 Skill，以及全新用户/机器的独立复验。
4. **Plugin 发布配置进行中**：公开 remote、`main` 与固定 Marketplace 测试分支已建立，hosted CI 已 PASS；但正式 release workflow、受保护 tag、branch protection required check 和 attestation 仍 NOT RUN。npm CLI 包已移出 v1 计划。
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
