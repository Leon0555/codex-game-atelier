# ADR 0002：分层门禁与显式可选 Git Hooks

- 状态：Accepted（2026-08-24 Phase 0 审阅通过）
- 日期：2026-08-24
- 决策范围：本地工作流、CLI、Git hooks 与 CI 的强制边界

## 背景

Codex 没有理由依赖某个外部模板的 hook 语义；用户也可能没有安装 hooks、用 `--no-verify` 绕过，或只调用 `build/export/release`。把所有安全性放在单独 `validate` 命令或自动安装 hooks 中，既不可靠也侵犯用户控制。

## 决策

采用四层互补门禁：

1. **Codex 工作流门禁**：Skills/Agents 在关键阶段调用确定性 CLI 并解释结果。
2. **CLI 命令内建门禁**：`build`、`export`、`release` 根据模式自动执行相应前置检查；用户不需要先运行 `validate`。
3. **显式可选 Git hooks**：只提供早期反馈，从不自动安装，也不成为唯一安全边界。
4. **CI 必选发布检查**：发布候选必须运行完整矩阵；hooks 缺失/绕过不影响 CI。

### 模式

| 模式 | 定位 | 最低语义 |
| --- | --- | --- |
| `manual` | 高级用户控制 | 仍执行命令安全、输入、目标、版本和不可缺少的前置检查；只省略非必需自动扩展检查 |
| `standard` | 默认 | build/export 自动运行相应 doctor/validate/test 子集并检查 evidence 新鲜度 |
| `strict` | 发布准备 | 更完整测试、干净状态/环境策略、证据完整性和失败即停；release 使用完整冻结矩阵 |

具体 gate graph 由版本化 policy/schema 表达。`validate` 是可单独调用的诊断入口，不是其他命令的免责条件。

### Git hooks 安装边界

- 初始安装、Plugin 安装和 `initialize` 均不写 hooks。
- 提供 `hooks list/plan/install/uninstall/status` 候选命令；`plan` 必须列出精确路径、已有 hook 冲突、行为和回退。
- `install` 需要用户显式调用/确认，不覆盖非本项目拥有的 hook；无法安全组合时停止并给手动方案。
- 每个安装物带所有权标记和可逆清单；卸载只移除本项目拥有的内容。
- Hook 只调用同一版本的确定性检查，不复制业务逻辑。

### CI 发布边界

- Release workflow 直接调用 `release check --mode strict` 并保存其唯一 JSON stdout，不调用本地 hook。
- 必选：schema/CLI 测试、Godot 冻结矩阵、参考游戏、分发内容、安全/许可/来源扫描和 artifact 验证。
- 发布 job 使用最小权限、受保护 tag/environment 与独立审批；真正外部发布仍需用户授权。

## 备选方案

### A. 自动安装 hooks

拒绝。会修改用户 Git 行为、产生冲突且卸载困难。

### B. 只依赖 `validate`

拒绝。用户可能忘记调用，自动化也容易错用。

### C. 只依赖 CI

拒绝。反馈太晚，且本地 build/export 仍可能生成误导产物。

### D. 所有模式完全相同

拒绝。无法同时满足高级用户可控性与发布高保障；但模式差异不得削弱不可缺少的安全门禁。

## 理由

命令内建门禁保证行为正确，CI 保证发布不可绕过，Codex 工作流提供可理解协作，hooks 只优化反馈速度。四层即使单层缺失也不会把发布安全性完全丢失。

## 风险

- gate graph 可能过重，需测量时间并区分 fast/full 层。
- evidence 新鲜度定义不当会导致假通过或频繁重跑。
- 跨平台 hooks 行为不同，必须视为便利功能而非生产承诺核心。

## 迁移与回退

- Policy schema 版本化；模式默认 `standard`，项目可显式改为 manual/strict。
- Hook 安装前产生 plan，安装失败不得改变已有 hooks；卸载按 manifest 回退。
- CI policy 变更必须通过 ADR/发布规则审阅，不能由项目配置静默关闭必选项。

## 验证

- 对每个命令与模式生成门禁矩阵和负例。
- 在未安装 hooks、hooks 失败和 `--no-verify` 情况下，CLI/CI 仍正确阻断。
- 验证 hooks 安装/冲突/卸载只影响列明路径。
- 验证普通 build/export/release 无需先手动 validate。

## Phase 1 实现记录（2026-08-29）

- 随 Plugin 分发 `gate-policy.json`，固定 `manual < standard < strict` 的单调门禁集合，默认 `standard`。
- `build` 与 `export` 使用完全相同的门禁图；`manual` 仍必须保留项目状态、宿主、Godot 标准版、GDScript、外部写入授权、固定 preset、artifact 完整性和 target smoke。
- `standard` 在此基础上加入主场景 Headless 与固定 GDScript tests；`strict` 再加入 run store、source policy 和分发 metadata。
- `release-check` 只有 strict 集合包含 Plugin、Starter、许可/来源与 required CI 完整发布项；manual/standard 不能被描述为发布就绪。
- Schema、contract fixture、语义测试和 Plugin 打包器共同阻止降级、非单调模式或缺少 mandatory gate 的分发。
- Go CLI 现已消费 artifact workflow：`standard` 在一次 build/export 总 timeout 内依次执行 Headless 与固定 GDScript tests，任一失败都会停止真正导出，并让原命令提交一份结构化失败闭包；`strict` 先执行同一 standard 子集，再对 M3 尚未实现的 run-store/source/distribution 项明确 `BLOCKED`。
- build/export 可用 `--mode manual|standard|strict` 对单次命令显式覆盖项目 mode；覆盖值进入原命令及嵌套 gate 的 immutable intent，但不改写 `project.json`。这提供可用的三模式入口而不提前引入 project-state replacement/migration 协议。
- CLI 消费不依赖 Git hook。`hooks list/plan/status/install/uninstall` 现只管理一个显式 `pre-commit` 与 ownership manifest；已有 hook、`core.hooksPath` 或 linked-worktree `.git` 均保守阻断，卸载只删除摘要仍与 manifest 相符的 owned 内容。
- 最小 CI 现为一个 `macos-15` Apple Silicon job，最小 `contents: read` 权限、外部 action 完整 SHA、Go 1.24.x 最低工具链、Python/Schema/分发测试和本机 CLI pair smoke。它不调用 hook，也不宣称替代 M3 Godot/分发/许可完整发布矩阵；首次 GitHub-hosted 运行在远程仓库获准创建与 push 前保持 `NOT RUN`。
