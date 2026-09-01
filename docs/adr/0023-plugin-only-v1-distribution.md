# ADR 0023：v1 Plugin-only 分发与条件式 Gatekeeper 门禁

- 状态：Accepted（用户于 2026-08-31 明确批准）
- 日期：2026-08-31
- 决策范围：v1 用户安装入口、Starter Template 取得方式、框架 macOS Gatekeeper 验证与 Apple 公证边界
- 部分取代：ADR 0004 的多入口/npm/standalone artifact 建议、ADR 0014 的独立 Starter archive 取得方式、ADR 0019 的双 archive 候选闭包

## 背景

当前 Go CLI 与 private runner 让普通用户无需安装 Node、Go 或从源码构建，但也形成了 macOS 原生可执行文件。M3 的可信本地 marketplace 安装、全新任务 Skill 发现、包内 CLI 调用和卸载已经通过；单独对 archive 添加 quarantine 的 Spike 则证明，手动下载/解包路径可能被 Gatekeeper 阻断。该失败不能自动推广到 Codex 真实远程 Plugin 获取路径，也不足以提前引入 Apple Developer ID、公证、证书和发布流水线。

参考项目的 clone-first/Node 路线避开了自有原生程序分发，但把 Node、clone、依赖安装和本地 build 转嫁给用户，不满足本项目已经冻结的零源码构建目标。

## 决策

### 唯一普通用户分发入口

1. v1 只通过 Codex Plugin 向普通用户分发 Codex Game Atelier。
2. 不提供面向普通用户的 standalone CLI archive、GitHub Release 二进制 ZIP、npm CLI、Homebrew、DMG 或 PKG。
3. Go CLI 与 private runner 继续作为 Plugin 内部、同版本、不可拆分的宿主组件；用户不需要把它们加入 `PATH` 或直接管理。
4. Starter Template 继续是 v1 产品能力，但随 Plugin 提供并由 Plugin 内 CLI/Skill 初始化，不再作为独立公开下载包或第二个安装入口。
5. 源码仓库继续服务贡献者、审计和可复现构建；普通用户不需要 clone 或 build。

### macOS Gatekeeper 门禁

1. Apple Developer ID 签名与公证从 v1 当前实施计划移除，不是默认发布门禁，也不预先建设证书、Keychain 或 notarization 流水线。
2. v1 macOS 框架分发门禁改为：从实际远程 Codex Plugin 渠道安装后，在干净 macOS Apple Silicon 环境中，无系统设置人工放行、无 `xattr` 清除、无隐藏安全策略修改地完成 Plugin 发现、public CLI、private runner 和至少一条真实 Godot 工作流。
3. 本地 marketplace PASS 只证明可信本地安装；人工 quarantine archive FAIL 只证明手动下载/解包不是受支持渠道。两者都不能替代远程 Plugin 实测。
4. 若远程 Plugin 安装无阻断实测 PASS，v1 明确只支持该已验证 Plugin 渠道；手动复制、独立下载或非支持安装器不在支持范围内。
5. 若该实测 FAIL，Apple Developer ID/公证仅作为备选方案重新提案。不得要求普通用户逐个放行隐藏二进制，也不得由 Plugin 自动清除 quarantine。

### 发布与 CI 边界

1. Git-backed Codex marketplace 是首个远程验证渠道；公共 Plugins Directory 只有在其审核/运行规则确认接受当前本地 Go 组件后才可成为正式入口。
2. 远程 marketplace 创建、push、Plugin 提交或发布仍需用户单独授权。
3. 普通用户 Plugin-only 不自动决定无人值守 CI 的取得方式。v1 CI 可在后续 ADR 中选择先安装固定 Plugin，或使用不面向普通用户的受控构建/Action；不得借此重新宣传 standalone 用户安装包。
4. 升级、卸载和回滚必须保留用户 Godot 项目及 `.gameatelier` 数据，并只管理 Atelier 自有 Plugin/cache 状态。

## 验收

远程 Plugin 无阻断门禁至少记录：

- 精确 Codex 版本、marketplace ref/commit、Plugin 版本与安装缓存位置。
- 安装前后用户级配置、Plugin 和 marketplace 差异。
- 安装态文件 hash、mode、平台布局与来源闭包。
- public CLI `--version`、private runner 固定拒绝契约和新任务 Skill 发现。
- CLI/runner quarantine 属性与 Gatekeeper 观察；不得以人工放行获得 PASS。
- 一条通过 Plugin 内 CLI 执行的 Godot Headless/test 或等价生产工作流。
- 成功卸载与测试 marketplace 清理；非测试 Plugin/marketplace 不变。
- 最终候选的成功升级、失败升级不切换 active version，以及上一版本回滚。

## 备选方案

### 预先实现 Developer ID 签名与公证

暂不采用。它需要付费账号、法律身份、证书私钥、签名顺序、公证凭据、每版本远程提交和干净下载复验，而实际受支持的 Plugin 渠道尚未证明需要它。

### 复用 clone-first/Node 本地 build

拒绝。它降低发布者的 macOS 原生分发责任，但违反普通用户无需 clone、npm build 和前沿 Node 的既定目标。

### 继续发布 Plugin 与独立 Starter archive

拒绝作为 v1 用户路径。双入口增加版本配对、来源、升级与回滚责任；Starter 应成为 Plugin 内的初始化资产。

### 自动移除 quarantine 或要求系统设置放行

拒绝。前者是隐藏修改系统安全元数据，后者不能满足无人值守 Plugin/CI 路径，也会把包内两个可执行文件的处理负担转嫁给普通用户。

## 风险

- Codex 远程 marketplace 的获取/cache 行为可能变化；每个 release candidate 都必须复验支持渠道，不能永久推定没有 quarantine。
- OpenAI 公共 Plugin 提交流程目前没有为 Plugin 内 standalone native executable 给出明确公开承诺；正式目录可用性仍待审核或官方确认。
- Plugin-only 会让纯 shell CI 的工具取得更复杂；后续必须定义稳定、可固定版本且不依赖隐藏 cache 路径的 CI 入口。
- 现有 `0.2.0` 本地候选仍是 Plugin + 独立 Starter 双 archive 历史证据。迁移完成前不得把它称为符合本 ADR 的最终候选。

## 迁移与回退

1. 下一版候选把已验证 Starter 内容和 manifest 纳入 Plugin 闭包，删除独立 Starter archive 的发布要求。
2. strict release gate 从“Plugin + Starter 两个 archive”迁移为“一个 Plugin archive 内同时验证 Plugin、CLI/runner、Starter、license 与 provenance”。
3. 历史双包候选、测试和验证记录保留，不重写为新决策下的 PASS。
4. 若 Plugin-only 远程验证失败，先记录可复现 blocker，再由新 ADR 选择 Apple 公证、不同 Plugin 运行形态或其他明确回退；不得静默恢复 standalone 用户下载。

## 实施证据补记

2026-09-01，`0.3.0-rc.1` 已从公开 Git-backed Codex Marketplace 安装到新的隔离 `CODEX_HOME`。安装缓存无 quarantine，未修改 `xattr`、系统设置或隐藏安全策略；public CLI、private runner 固定拒绝契约、特殊路径 embedded Starter、Godot Headless 与 GDScript 6/6 均 PASS。同日真实用户级 `rc.0 → rc.1`、失败升级不替换 active `rc.1`、`rc.1 → rc.0`、卸载和起点状态恢复也已 PASS。因此当前证据不触发 Apple 公证备选方案。详见 [`m3-remote-plugin-lifecycle-2026-09-01.md`](../validation/m3-remote-plugin-lifecycle-2026-09-01.md)。

仍未完成的是远程安装态的新任务 Skill 发现、全新用户/机器独立复验、受保护发布 tag/attestation 和最终独立审计；这些缺口不得由本次 PASS 推定。
