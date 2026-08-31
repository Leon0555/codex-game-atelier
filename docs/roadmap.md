# Codex Game Atelier 路线图

状态：Phase 0 已审阅通过；M1/M2 本地实现已完成，当前进入 M3
更新日期：2026-08-31

## 1. 已完成基线

Phase 0 已于 2026-08-24 通过用户审阅，确立了 Godot-only v1.0、Go CLI、MIT License、文件化状态与证据、Plugin + Starter Template 配套分发等边界。

Phase 1 已完成的生产薄切片包括：

- `detect`、`doctor`、`status` 与幂等 `initialize`。
- 静态及 Godot Headless `validate`、固定零依赖 GDScript `test`。
- 原子 run evidence、结构化 `logs`、有界 run scanner 与只读 `clean --list`。
- 可复现的多宿主预构建 Plugin bundle；当前只有 macOS Apple Silicon 完成原生执行验证。
- 与已安装 Plugin 配套的干净 Starter Template 及可复现 archive/checksum。

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
4. **已完成**：实现只读 `release check`；manual/standard 不冒充严格发布就绪，standard 聚合当前 revision 最新 evidence 并复验 Release ZIP，strict 未实现的 M3 分发项明确阻断；不实现自动外部发布，也不增加独立 `release prepare` 流水线。
5. **已完成（本地）**：提供一个显式安装、可列出、可卸载的轻量 `pre-commit` hook；不自动安装、不覆盖既有 hook，CLI 与 CI 门禁不依赖它。
6. **已完成实现、托管运行 NOT RUN**：建立单一 macOS Apple Silicon CI job，固定只读权限与 action SHA，验证 Go 1.24 最低版本、Python/Schema、Plugin/Template 静态完整性和本机 CLI pair；因尚无远程仓库，首次 GitHub-hosted 结果仍待后续授权 push。

M2 不做：隐藏规划器、常驻多代理服务、通用策略引擎、完整任务数据库、派生索引、任意代码执行、自动安装 hooks。

退出条件：三种模式的安全边界、模型抽象、一次可恢复原生协作、hook 可选性及 CI 不可绕过性均有正反例证据。

### M3：可安装、可发布验证

目标：集中完成一次真实生命周期和 v1.0 发布审计，避免在每个功能切片重复安装演练。

实施顺序：

1. **已完成（本地开发候选）**：冻结 Plugin/Starter Template/CLI 的精确版本闭合和分发清单；两次候选逐字节一致。当前 `0.2.0` 不是 v1.0 最终版本冻结，framework artifact Gatekeeper 状态仍 `NOT_EVALUATED`。
2. **进行中（最小真实闭环已 PASS）**：当前 Codex CLI 已从专用本地 marketplace A 完成真实注册、安装、安装态校验、全新任务 Skill 发现、包内 CLI 调用、卸载和 marketplace 清理；其他 Plugin 清单未变。失败升级、成功升级与上一版本回滚按用户收敛范围留到最终候选，详见 [`m3-minimal-plugin-install-2026-08-31.md`](validation/m3-minimal-plugin-install-2026-08-31.md)。
3. **进行中（发布前只读审计已完成）**：checksum、manifest、archive 安全、静态秘密/网络/遥测/写入边界 PASS；审计发现 dirty-worktree 二进制 provenance、Go third-party notice、framework Gatekeeper 与三宿主 Tier 1 证据四项 release blocker，详见 [`m3-supply-chain-readonly-audit-2026-08-31.md`](validation/m3-supply-chain-readonly-audit-2026-08-31.md)。修复必须在独立 Implementation 步骤完成后重新审计。
4. **设计已冻结、发布配置 NOT RUN**：npm Trusted Publishing、2FA 与 package provenance 方向已记录；当前没有 npm package、remote、release workflow、OIDC attestation 或 SBOM，不实际发布。
5. 冻结 Support Matrix，完成架构、安全、许可证、性能、文档和分发的独立只读终审。
6. 用户明确批准后，才允许 GitHub Release、npm publish 或 Marketplace 提交。

M3 不做：Godot 游戏产物签名/公证、自动账号登录、长期发布 Token、未经授权的远程写入。

退出条件：冻结范围内所有必选发布门禁为 PASS，Blocker/High 为零，用户明确批准发布。

## 3. 明确延后但不隐藏的项目

以下项目不进入当前 M1/M2 日常开发路径：

- Windows x64 与 Linux x64 原生 runner/机器验证。
- 完整 schema migration 引擎、派生索引和通用恢复数据库。
- `clean` 的实际删除能力；当前保持只读 `clean --list`。
- 第三方 GDScript 测试框架、过滤/异步 fixture 平台。
- raw 日志捕获、follow/tail、分片和通用脱敏流水线。
- 多套顶层 Skills 或为每种责任创建独立常驻 Agent。
- npm 包装实现及完整 SBOM，直到候选版本确有发布需要。

这些延后项如果改变已冻结产品承诺，必须在实施前重新决策；不能因为未排入当前开发循环就被描述成已经支持。

## 4. Windows/Linux 决策检查点

用户已决定当前不建设 Windows/Linux 原生 runner。M2 结束时再确认 v1.0 最终对外范围：

1. 补齐 Windows x64、Linux x64 原生端到端证据，维持三宿主 Tier 1；或
2. 通过 Support Matrix 决策把 v1.0 生产级承诺限定为 macOS Apple Silicon，并把其他宿主明确降为预览/产物可用性声明。

在此决策前，交叉构建只证明 artifact 可生成，不证明对应宿主受支持。

## 5. 后续版本

Unity、Godot .NET/C#、Web/移动/主机导出和签名/公证均不属于 v1.0。Unity 只有在 v1.0 稳定后才能重新立项；必须先做官方 CLI、许可证和平台 Spike，且 `unity eval` 默认关闭。
