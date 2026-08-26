# ADR 0014：Starter Template 干净身份与分发配对边界

- 状态：Proposed（模板项目本体已本地实证；独立分发还是与 Plugin 配套尚待用户决定）
- 日期：2026-08-26
- 决策范围：Starter Template 内容、身份/缓存边界、固定测试入口和普通用户首次使用路径

## 背景

Starter Template 是普通用户主要入口之一，但不能把源码仓库研发制度、已生成项目身份、历史 evidence、Godot 缓存或工具链复制进新游戏。另一方面，若为了“模板可独立使用”把 Skill 与三宿主 CLI 再嵌入每个游戏项目，会复制约 26 MiB 展开产物并形成第二套升级/回滚责任。

## 已实证的候选内容契约

1. 模板是 Godot `4.7.2-stable` standard/GDScript 项目，不含 .NET、Unity 或未来引擎占位实现。
2. 固定包含可运行 `main.tscn`、基础 UI、signal/input/game-state 示例、中文与空格资源路径，以及 `res://tests/atelier_test_runner.gd`。
3. 模板不得包含 `.gameatelier/`、`.godot/`、导出物、二进制、SDK、依赖、凭据、遥测、Git hooks、`AGENTS.md`、具体模型 ID 或源码仓库旧产品名。
4. 模板不携带 `project_id`；每个复制项目由显式 `initialize` 使用 CSPRNG 建立新身份。复制后重复 initialize 必须保持 state bytes 与 mtime 不变。
5. `.gitignore` 只忽略 Godot 缓存、导出凭据和 build output。在 `.gameatelier` 的版本控制政策冻结前，不得默认把它隐藏；用户必须能看到并审阅新生成的状态/evidence。
6. 模板 README 使用三个主要步骤，并明确计数从 Plugin 已安装、Godot 已具备后开始；同时说明 Headless/test 的 `user://` 授权、Phase 1 仅 Apple Silicon 原生实证和不得复制仓库根 `AGENTS.md`。
7. 静态 validator 使用固定文件/目录 allowlist，拒绝 root/内部 symlink、hardlink、特殊/空/超限/可执行内容、.NET、缺失固定结构标记、常见具体模型家族 ID、内部身份或未冻结状态忽略规则。静态模式不可能证明所有未来模型命名，也不冒充 GDScript 解析器；完整分发扫描/审计与可信副本上的真实 Godot Headless/test 都是独立门禁。

## 尚待冻结的分发选择

### A. Template 与 Codex Plugin 配套（当前候选，推荐）

用户取得干净游戏模板，Codex 工作流与预构建 CLI 由单独安装的 Plugin 提供。优点是 Skill、CLI、checksum、升级和回滚只有一个 owner，不在每个游戏仓库复制平台二进制；缺点是“取得 Starter Template”本身不能替代 Plugin 安装，需要把三步文案解释为“模板 + Plugin 配套入口”。

### B. Template 自带项目级 Skill 与三宿主 CLI

模板单独复制即可提供工作流。优点是离线、单包；缺点是每个游戏项目携带约 26 MiB CLI/runner、Skill 副本与许可证，更新容易漂移，而且会把工具生命周期混入游戏源码。还需要实证 Codex 对项目级 Skill/二进制的发现、权限和安全边界。

### C. Template 首次使用时在线下载 CLI

拒绝作为默认方案。它引入隐藏网络写入、供应链和失败恢复问题；任何显式下载器都需要独立安全决策、固定版本与 checksum，不能成为当前三步体验的隐式副作用。

## 本地候选验证

模板源复制到含中文、空格和 `#` 的新目录后，已通过显式 Godot detect、首次/幂等 initialize、包内 sibling runner Headless validate 和固定 GDScript test。运行后与源模板的内容差异只有副本内 `.gameatelier`；没有把生成状态写回源模板。

脱敏命令、宿主/引擎/工具哈希、initialize 前后状态、Headless/test 结构化结果和模板源哈希已持久化到 [`docs/validation/evidence/phase1-starter-template-2026-08-26/`](../validation/evidence/phase1-starter-template-2026-08-26/)，并由回归测试检查源哈希与结果间的一致性。

这些证据不等于：Template 已独立分发、Codex 客户端实际从模板发现 Skill、Plugin 已安装、升级/卸载/回滚通过，或 Linux/Windows 原生通过。

## 风险

- `.gameatelier` VCS 政策尚未冻结；当前“保持可见”只避免静默隐藏，不代表建议提交全部 run evidence。
- 模板固定测试是最小基线，不是完整测试框架或游戏架构。
- Option A 会要求修订“Plugin 或 Starter Template”措辞；Option B 会扩大 artifact 与生命周期矩阵。
- Godot 打开项目后可能生成 `.godot/` 等正常缓存；这些不属于模板源或 Atelier 身份。

## 回退

在 ADR Accepted 前，模板只作为 Phase 1 candidate。回退可以移除未发布模板源和 validator，但不得删除任何用户已从模板创建的游戏项目或 `.gameatelier` evidence。最终分发选择必须由用户确认，并在真实安装/卸载与三步演练后把本 ADR 转为 Accepted。
