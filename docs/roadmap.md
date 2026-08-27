# Codex Game Atelier 路线图

状态：Phase 0 已审阅通过；Phase 1 按三里程碑精简方案实施
更新日期：2026-08-27

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

### M1：可运行、可导出（当前）

目标：在 macOS Apple Silicon 上完成从模板到可启动 Godot 游戏产物的闭环。

实施顺序：

1. **已完成**：`doctor` 检查并冻结 Godot 版本匹配的 export templates。
2. **已完成**：指定 preset/profile 的 Debug/Release `export`，记录超时、退出码、产物 manifest/hash 和分发就绪限制；Universal 2 由实际 Mach-O slices 验证。
3. **已完成**：`build --profile debug|release` 薄封装；复用同一 export 执行和 evidence，不制造第二条 Godot 流水线。
4. 把 Starter Template 扩展为小而完整的参考游戏/玩法垂直切片。
5. 在中文、空格和特殊路径完成 `initialize → validate → test → build/export → target smoke`。

前三项的最终 macOS Debug/Release 实机结果见 [`m1-macos-export-build-2026-08-27.md`](validation/m1-macos-export-build-2026-08-27.md)。M1 尚未退出：玩法扩展、完整特殊路径 E2E 与 target smoke 仍未完成。

M1 不做：Windows/Linux 原生运行、签名/公证、商店发布、完整场景图解析器、第三方测试框架适配、raw 日志系统。

退出条件：Apple Silicon 上 Debug 与 Release 技术导出均有可复现 PASS evidence，产物可启动并正常退出；未验证范围被明确标记。

### M2：可协作、可管控

目标：用最小机制证明 Codex 原生协作与发布前门禁，不建设通用编排或策略平台。

实施顺序：

1. 用逻辑能力 Profile 表达能力等级、会话继承和用户覆盖；分发内容不含具体模型 ID。
2. 用一次真实有界子代理工作流验证单一 owner、只读审计和 task/handoff/evidence 恢复；不实现常驻服务。
3. 以一张命令前置条件表实现 `manual`、`standard`、`strict`：`manual` 是显式命令语义，`standard` 是默认生产门禁，`strict` 聚合发布条件。
4. 实现只读 `release check`；不实现自动外部发布，暂不增加独立 `release prepare` 流水线。
5. 提供一个显式安装、可列出、可卸载的轻量 Git hook；CLI 与 CI 门禁不能依赖 hook。
6. 建立一个最小 CI workflow，先验证 Go/Python、Schema、Plugin/Template 静态完整性和 macOS 可执行路径。

M2 不做：隐藏规划器、常驻多代理服务、通用策略引擎、完整任务数据库、派生索引、任意代码执行、自动安装 hooks。

退出条件：三种模式的安全边界、模型抽象、一次可恢复原生协作、hook 可选性及 CI 不可绕过性均有正反例证据。

### M3：可安装、可发布验证

目标：集中完成一次真实生命周期和 v1.0 发布审计，避免在每个功能切片重复安装演练。

实施顺序：

1. 冻结 Plugin/Starter Template/CLI 的版本闭合和分发清单。
2. 在干净用户路径集中演练取得、安装、发现、初始化、升级、卸载和回滚；保留用户项目与凭据。
3. 验证 checksum、manifest、LICENSE、NOTICE、provenance、无默认遥测和无隐藏外部写入。
4. 在候选版本阶段才完成 npm Trusted Publishing、2FA 和 package provenance 的只读/预发布设计演练；不实际发布。
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
