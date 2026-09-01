# ADR 0014：Starter Template 干净身份与分发配对边界

- 状态：Accepted（干净模板边界继续有效；独立 archive 取得方式已由 ADR 0023 部分取代）
- 日期：2026-08-26
- 决策范围：Starter Template 内容、身份/缓存边界、固定测试入口和普通用户首次使用路径

## 背景

Starter Template 是普通用户主要入口之一，但不能把源码仓库研发制度、已生成项目身份、历史 evidence、Godot 缓存或工具链复制进新游戏。另一方面，若为了“模板可独立使用”把 Skill 与三宿主 CLI 再嵌入每个游戏项目，会复制约 26 MiB 展开产物并形成第二套升级/回滚责任。

## 已实证的候选内容契约

1. 模板是 Godot `4.7.2-stable` standard/GDScript 项目，不含 .NET、Unity 或未来引擎占位实现。
2. 固定包含可运行 `main.tscn`、Atelier Spark 收集/胜利/重置玩法、UI、signal/input/game-state、中文与空格资源路径、`res://tests/atelier_test_runner.gd` 六项测试，以及关闭签名/公证的 `macOS Technical` preset。
3. 模板不得包含 `.gameatelier/`、`.godot/`、导出物、二进制、SDK、依赖、凭据、遥测、Git hooks、`AGENTS.md`、具体模型 ID 或源码仓库旧产品名。
4. 模板不携带 `project_id`；每个复制项目由显式 `initialize` 使用 CSPRNG 建立新身份。复制后重复 initialize 必须保持 state bytes 与 mtime 不变。
5. `.gitignore` 只忽略 Godot 缓存、导出凭据和 build output。在 `.gameatelier` 的版本控制政策冻结前，不得默认把它隐藏；用户必须能看到并审阅新生成的状态/evidence。
6. 模板 README 使用三个主要步骤，并明确计数从 Plugin 已安装、Godot 已具备后开始；同时说明 Headless/test 的 `user://` 授权、Phase 1 仅 Apple Silicon 原生实证和不得复制仓库根 `AGENTS.md`。
7. 静态 validator 使用固定文件/目录 allowlist，拒绝 root/内部 symlink、hardlink、特殊/空/超限/可执行内容、.NET、缺失固定结构标记、常见具体模型家族 ID、内部身份或未冻结状态忽略规则。静态模式不可能证明所有未来模型命名，也不冒充 GDScript 解析器；完整分发扫描/审计与可信副本上的真实 Godot Headless/test 都是独立门禁。

## 分发决策

### A. Template 与 Codex Plugin 配套（已采用；分发形态由 ADR 0023 收敛）

用户取得干净游戏模板，Codex 工作流与预构建 CLI 由安装的 Plugin 提供。Skill、CLI、checksum、升级和回滚只有一个 owner，不在每个游戏仓库复制平台二进制。ADR 0023 进一步规定模板随 Plugin 提供并由 Plugin 内 CLI/Skill 初始化，不再发布独立 Starter archive；复制到用户项目后的干净内容契约保持不变。

确定性 Template archive 不嵌入 `bin/`、`skills/` 或 Plugin manifest；包内 `TEMPLATE-MANIFEST.json` 记录 `embedded=false`、配套 Plugin 名称和本 candidate 已验证的 Plugin 版本。该字段不声称是最低版本或兼容范围；兼容策略在真实升级/回滚实证后另行冻结。Plugin 安装后可离线使用，不得在模板首次打开时隐式下载工具。

archive 旁的 `.sha256` 和包内 manifest 只能证明同源文件的完整性与自洽，不能证明发布者身份或来源可信。对外执行授权必须来自独立可信渠道中的预期摘要、签名或 package provenance；当前 D-008 仍为 NOT RUN。

### B. Template 自带项目级 Skill 与三宿主 CLI（v1.0 不采用）

模板单独复制即可提供工作流，但每个游戏项目会携带约 26 MiB CLI/runner、Skill 副本与许可证，更新容易漂移，而且会把工具生命周期混入游戏源码。作为未来显式离线/内网变体可重新提案，但不属于 v1.0 默认分发。

### C. Template 首次使用时在线下载 CLI

拒绝作为默认方案。它引入隐藏网络写入、供应链和失败恢复问题；任何显式下载器都需要独立安全决策、固定版本与 checksum，不能成为当前三步体验的隐式副作用。

## 本地候选验证

模板源复制到含中文、空格和 `#` 的新目录后，已通过显式 Godot detect、首次/幂等 initialize、包内 sibling runner Headless validate 和固定 GDScript test。运行后与源模板的内容差异只有副本内 `.gameatelier`；没有把生成状态写回源模板。

脱敏命令、宿主/引擎/工具哈希、initialize 前后状态、Headless/test 结构化结果和模板源哈希已持久化到 [`docs/validation/evidence/phase1-starter-template-2026-08-26/`](../validation/evidence/phase1-starter-template-2026-08-26/)，并由回归测试检查源哈希与结果间的一致性。

方案 A 接受后，已生成不含 Plugin 二进制/Skill 的确定性 `0.2.0` candidate archive，并通过双份逐字节复现、外部 SHA-256、安全解包、配套元数据和源契约重验。2026-08-26 的初始薄模板证据保留为历史；当前可玩模板、特殊路径全链和重打包证据见 [`m1-playable-vertical-slice-2026-08-28.md`](../validation/m1-playable-vertical-slice-2026-08-28.md)。

这些证据不等于：Template 已独立分发、Codex 客户端实际从模板发现 Skill、Plugin 已安装、升级/卸载/回滚通过，或 Linux/Windows 原生通过。

## 风险

- `.gameatelier` VCS 政策尚未冻结；当前“保持可见”只避免静默隐藏，不代表建议提交全部 run evidence。
- 模板固定测试是最小基线，不是完整测试框架或游戏架构。
- 方案 A 要求普通用户先安装一次 Plugin；后续必须用明确 doctor 诊断、版本兼容和可回滚安装降低全局工具版本的影响。
- Godot 打开项目后可能生成 `.godot/` 等正常缓存；这些不属于模板源或 Atelier 身份。

## 回退

若方案 A 无法在受支持 Codex 客户端实现可诊断、可回滚的安装路径，以新 ADR 重新评估分平台 Plugin 或显式 standalone bundle。回退未发布候选不得删除任何用户已从模板创建的游戏项目或 `.gameatelier` evidence。
