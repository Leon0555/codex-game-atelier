# ADR 0020：Codex 本地 Marketplace 生命周期演练

- 状态：Accepted（最小本地闭环与远程候选完整生命周期已通过）
- 日期：2026-08-30
- 决策范围：Codex Plugin 的本地安装、升级失败、升级、卸载与回滚验证

## 背景

本地 Plugin bundle 与分发 candidate 已经通过静态验证，但“目录存在”不能证明当前 Codex 客户端可以发现、安装和加载它。当前本机 Codex CLI 明确提供 `plugin marketplace add/list/remove` 与 `plugin add/list/remove`；非默认 marketplace 必须先显式注册。Plugin Creator 规则同时要求 marketplace entry 固定指向 `./plugins/<plugin-name>`，更新后从新任务拾取 Skill，并用 cachebuster 避免本地开发版本缓存混淆。

真实命令会修改用户级 Codex config/cache。用户已于 2026-08-31 授权一次收敛后的最小真实闭环：注册专用 marketplace、安装、在全新任务发现 Skill、调用包内 CLI、卸载并移除 marketplace。升级、失败升级与回滚不在该次授权范围内。

## 提议决策

1. 维护工具 `tools/package_local_marketplace.py` 从已验证 Plugin bundle 生成一个此前不存在的本地 marketplace 根。固定名称为 `codex-game-atelier-local`，唯一 Plugin 为 `codex-game-atelier`，source 为 `./plugins/codex-game-atelier`，policy 为 `AVAILABLE`/`ON_INSTALL`，category 为 `Productivity`。
2. 工具不修改 `~/.codex`、默认 personal marketplace 或现有 Plugin。它拒绝覆盖、symlink、hardlink、特殊文件、未知根内容和错误 mode，并在复制后重跑完整 bundle 验证。
3. 真实演练使用 Codex 自己的 `plugin marketplace` 与 `plugin` 命令，不实现 Atelier 私有用户安装器，也不让普通用户运行 Python/Go 或 clone 源码。
4. 初始安装候选使用开发版本 `0.2.0`。升级候选只为本地演练使用 `0.2.0+codex.lifecycle-b`，公共 CLI 与 private runner 注入相同版本；它不改变源码 manifest 或 v1.0 对外版本决策。
5. 真实演练前后只比较允许范围的路径清单、文件 metadata 与摘要，不输出 config、凭据或 token 内容。用户游戏项目和凭据必须逐字节保持；失败升级不得切换已安装 active version。
6. 演练分两层：最小闭环为初始快照 → 注册 marketplace A → 安装/列出/包内 CLI smoke → 新任务发现 → 卸载 → 移除测试 marketplace → 前后快照对比；最终候选再补无效升级负例、marketplace B 升级、卸载和 marketplace A 回滚。不得把最小闭环描述成完整生命周期通过。
7. 创建新 Codex 测试任务、写用户级 config/cache、安装/卸载 Plugin 均需用户一次明确授权。外部远程、push、登录和发布不在该授权内。

## 备选方案

### 直接复制 bundle 到 Codex cache

拒绝。会绕过客户端安装语义、所有权 metadata 和卸载路径，无法证明用户入口。

### 写入默认 personal marketplace

拒绝。会混入用户已有插件源，也不利于完整移除测试。专用非默认 marketplace 边界更清晰。

### 自定义 Atelier installer

拒绝。增加新的升级和权限 owner，也偏离 Codex Plugin 主入口。

## 风险

- 当前官方网页检索未返回可用正文；命令形状以本机当前 Codex CLI `--help` 与 Plugin Creator 随附规则为实证，客户端升级后需重验。
- 本地 marketplace 不证明未来 Git/Marketplace 远程分发行为。
- Plugin 更新需要新任务拾取；当前任务不能证明新安装 Skill 的发现。
- 用户级演练若中断，必须依据开始时快照逐项恢复测试 Plugin/marketplace，不能影响其他插件。

## 回退

- 在未安装前，删除被忽略的 `.tools/marketplaces/` 候选即可；不影响用户配置。
- 真实演练最终必须运行 `plugin remove` 和 `plugin marketplace remove`，并确认只移除本测试 selector。
- 若任一步无法证明精确所有权或恢复边界，停止后续写入并报告 `BLOCKED`，不手工删除用户 cache。

## 转为 Accepted 的证据

- 当前 Codex CLI 的 add/list/remove JSON 与实际 cache/config diff。最小闭环已于 2026-08-31 PASS，见 [`m3-minimal-plugin-install-2026-08-31.md`](../validation/m3-minimal-plugin-install-2026-08-31.md)。
- 新任务中 Skill 发现及相对路径 CLI/runner 调用。最小闭环已实际证明 Skill 发现和 CLI `--version`；runner 在主任务中证明直接调用固定退出 125。
- 失败升级不切换、成功升级、卸载保留用户项目/凭据、上一版本回滚。
- 最终移除测试 Plugin/marketplace 后，非测试用户状态与开始快照一致。

## Accepted 证据补记

2026-09-01，用户另行批准了远程与真实用户级完整演练。Git-backed `0.3.0-rc.0` 安装、`0.3.0-rc.1` 升级、不可解析 manifest 失败候选不替换 active `rc.1`、回滚 `rc.0`、卸载和完整恢复全部 PASS。演练后非测试 Plugin/Marketplace ID 清单与起点一致，`config.toml` SHA-256 也字节级一致；不需用快照覆盖用户文件。详见 [`m3-remote-plugin-lifecycle-2026-09-01.md`](../validation/m3-remote-plugin-lifecycle-2026-09-01.md)。

当前 Codex CLI 对“同一 Marketplace 改 Git ref”的实测契约是 `marketplace remove → marketplace add(new ref) → plugin add`，而不是就地修改 ref。另外，客户端会把缺少 `version` 的 manifest 宽松安装为 `local`；Atelier 不得因此删除自己的 bundle/version/provenance 完整性验证。
