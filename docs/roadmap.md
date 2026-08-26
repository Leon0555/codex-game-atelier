# Codex Game Atelier 路线图

状态：Phase 0 已审阅通过；Phase 1 进行中，日期与版本不构成发布承诺
日期：2026-08-24

## Phase 0：开发基线（已完成）

目标：建立本地仓库骨架、范围、架构、ADR、来源审计、Support Matrix 建议、验收草案、许可证和环境盘点。

退出条件：

- 所有 Phase 0 文档可审阅且相互一致。
- v1.0 明确为 Godot-only，Unity 不进入实现或门禁。
- 参考仓库许可证由原始 LICENSE 重新确认。
- Support Matrix 是有证据的推荐而非冻结结论。
- 用户明确回复“Phase 0 审阅通过，进入 Phase 1”。

审阅结果：2026-08-24 通过。通过只授权进入 Phase 1，不等于产品已实现、Support Matrix 已取得生产证据或允许外部发布。

## Phase 1：契约与垂直骨架（当前）

建议顺序：

1. 冻结已批准的最小 Support Matrix 目标与首个 ADR 集合；生产级声明仍等待实证。
2. 用 Spike 验证 CLI 运行时、Plugin 携带/调用方式及零构建安装路径；Rust/Go 对照已完成并于 2026-08-25 冻结 Go。2026-08-26 已完成本地多宿主 bundle、可复现 archive、外部 checksum，以及 Apple Silicon 包内入口、Headless validate 与固定 GDScript test；真实 Codex 安装、Gatekeeper、升级/回滚及 Linux/Windows 原生验证继续进行。
3. 已定义 JSON Schema 初始基线：命令结果、错误、run intent/evidence/validation report、task/handoff 与 project state；initialize 单文件边界由 ADR 0007 冻结，多文件 run 提交由 ADR 0008 在限定范围接受，ADR 0009 增补 Headless 外部写入声明。
4. 已建立生产 Go CLI 的四个垂直切片：只读 `detect`/`doctor`/`status`、幂等 `initialize`、带 evidence 的静态 baseline `validate`、明示授权的 Godot Headless 一帧验证；Windows/Linux 原生运行仍未验证。
5. 已将最小 Codex Plugin/Skill 骨架接到当前 CLI，并通过官方 Plugin/Skill validator；逻辑 Profile、真实安装和原生子代理交接仍继续验证。
6. 用户已批准门禁/引擎命令默认持久化 evidence；ADR 0008 的 run 自包含目录、最后 `result.json` 提交点、故障注入和 public validate 接线已实现，独立终审为 0 Blocker/High/Medium/Low。
7. 已用参考项目完成“CLI → Godot 4.7.2 Headless → 中文/空格资源 → 原子 evidence”的端到端薄切片；Codex 工作流入口和完整测试仍继续。
8. 已建立 ADR 0010 的有界 run scanner 与只读 `clean --list` 生产切片；实际删除、恢复、索引仍留在 Phase 2。
9. 已建立 ADR 0011 的固定 GDScript `test` 生产薄切片：零额外框架依赖、逐项报告、断言/引擎/超时映射与原子 evidence；第三方框架、过滤和三宿主原生验证仍待后续。
10. 已建立 ADR 0012 的只读 `logs --run-id` 生产薄切片：同次验证 committed closure，只投影零自由文本结构事件与 integrity metadata；raw 日志保留/脱敏仍待后续独立决策。
11. 已提出 ADR 0013 并完成预构建 Plugin bundle 本地候选：显式 source allowlist、真实二进制格式/架构检查、CLI/plugin 版本闭合、deterministic archive 与安全解包；ADR 在实际 Codex 安装和 quarantine 验证前保持 Proposed。
12. 已提出 ADR 0014 并完成干净 Starter Template 项目本体：无身份/缓存/内部 AGENTS，可在特殊路径显式初始化并通过 Headless 与固定 test；独立携带 CLI 还是与 Plugin 配套仍待用户决定。
13. 进行独立只读架构、安全和可恢复性评审。

Phase 1 不应先铺开所有命令、Agents 或 Skills；先证明契约、运行时、分发与证据链能闭环。

## Phase 2：确定性核心与门禁

- 在 Phase 1 已实现的只读 run scanner 基础上实现 schema 迁移、锁内恢复、确认式删除与派生索引；单文件初始化和首个多文件 evidence 提交已经建立。
- 从已实现的一帧 Headless、固定 GDScript test 与结构化 logs 薄切片扩展到完整场景/资源图、异步/fixture/过滤测试；raw 日志捕获/脱敏/分片需先形成独立隐私决策，扩展 `initialize` 的迁移/clone 语义前另行决策。
- 实现 `manual`、`standard`、`strict` 及命令内建门禁。
- 建立稳定退出码、超时、取消、重试与幂等测试。
- 建立 CI 基础检查；Git hooks 仅提供显式可选安装器和卸载说明。

## Phase 3：Godot 生产适配器

- Headless 启动、场景/资源/GDScript 验证。
- 输入、信号、资源、UI 和基础玩法工作流。
- Debug/Release 构建和约定目标导出；macOS 仅验证未签名、未公证的 Apple Silicon 技术导出。
- 结构化日志、错误分类、超时、异常退出和恢复。
- 中文、空格、特殊路径与三宿主验证。

## Phase 4：参考游戏与端到端证据

- 建立小而完整的 Godot 垂直切片，不追求内容规模。
- 从空模板/安装入口验证初始化、开发、测试、构建和导出。
- 验证干净环境、版本升级、失败恢复和回滚。
- 建立性能与稳定性基线。

## Phase 5：分发与生命周期

- 完成 Codex Plugin 和 Starter Template。
- 验证最多三个主要步骤的首次使用路径。
- 产出预构建 CLI、校验和、SBOM/来源证明策略。
- 验证安装、升级、卸载、回滚和无残留/无隐藏写入。
- 完成 npm Trusted Publishing 与 package provenance 的发布前演练；不实际发布，除非用户另行批准。

## Phase 6：冻结与发布审计

- 冻结 Support Matrix、命令契约、文档和版本策略。
- 在干净环境执行完整矩阵并保留 evidence。
- 完成安全、架构、许可证、性能与分发的独立只读终审。
- 清零 blocker/high 问题；记录所有 NOT RUN/BLOCKED/SKIPPED。
- 用户明确批准后，才创建外部 Release、发布 npm 或提交 Marketplace。

## 后续版本

Unity 只在 v1.0 稳定后重新立项。必须先做官方 CLI/许可证/平台 Spike 和新 ADR；不得把 v1.0 的 Godot 内部概念直接当作 Unity 契约。`unity eval` 默认关闭并排除在核心发布路径之外。
