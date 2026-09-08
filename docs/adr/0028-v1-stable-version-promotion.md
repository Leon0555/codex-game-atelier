# ADR 0028：首个对外稳定版本晋升为 1.0.0

- 状态：Accepted（用户于 2026-09-08 批准版本号与最终候选制作）
- 日期：2026-09-08
- 决策范围：v1 首个公开稳定版本号、候选晋升和重验边界
- 关联：ADR 0013、0019、0021、0023、0026、0027

## 背景

`0.3.0-rc.6` 已通过可复现构建、隔离远程 Plugin 安装、真实用户级生命周期、Skill 发现、Godot E2E、required CI、strict 聚合和独立最终只读审计。用户随后批准将首个正式对外稳定版本定为 `1.0.0`。

版本字符串会进入公开 CLI、private runner、Plugin manifest、Starter manifest、bundle/distribution manifest、archive 文件名、checksum 和外部 release evidence。因此不能把 rc.6 归档直接改名或改 tag 冒充 `1.0.0`。

## 决策

1. Codex Game Atelier 首个对外稳定版本号冻结为 `1.0.0`。若最终候选复验发现需要修复的问题，则修复后重建同一尚未发布的 `1.0.0` 候选；在正式发布前不增加无用的新 RC 命名层。
2. 当前源码 Plugin manifest 升级为 `1.0.0`，作为受信任打包的默认稳定身份。Go 源码中未注入的开发构建继续使用 `-dev` 标识；只有受信任构建通过 Go linker 注入精确 `1.0.0`，不让普通本地编译冒充候选。
3. `1.0.0` 必须从包含本决策和版本元数据的新干净 source revision 重建两次；CLI/runner、Plugin bundle、archive 和 distribution candidate 必须逐字节可复现。
4. 最终候选必须重新验证：包结构与供应链、rc.6 到 `1.0.0` 升级、损坏升级保护、回滚 rc.6、重装/卸载/用户状态恢复、远程 Plugin 取得与 Skill 发现、特殊路径 Godot E2E、required CI、绑定 strict `release check` 和独立最终只读审计。
5. rc.6 及其 hash、revision、Marketplace ref 和 evidence 保留为历史事实，不回写成 `1.0.0`。
6. 本决策只授权制作与复验最终候选。受保护 `v1.0.0` tag、GitHub Release、正式 Plugin 外部发布或其他可见的稳定版发布仍必须在所有重验通过后另行获得用户明确批准。

## 备选方案

### 直接将 rc.6 归档改名为 v1.0.0

拒绝。这会使二进制自报版本、Plugin/Starter 元数据、archive 文件名、hash 和绑定 evidence 与公开名称不一致。

### 把 0.3.0 作为首个稳定版

不采用。它会与已冻结的 v1.0 范围、验收表和用户批准的对外版本不一致。

### 先增加 1.0.0-rc.1，再重建 1.0.0

不采用为默认路径。rc.6 已承担代码候选价值；额外 RC 会强制重复两套版本依赖的闭环，没有新增产品信息。

## 风险与缓解

- `1.0.0` 是兼容性承诺，不是把现有 PASS 描述成已发布。它仅在冻结 Support Matrix 内承诺已验证的公开契约；明确延期项不因大版本号被隐式纳入。
- 候选在未发布期间可因终审修复而重建，所以任何候选 hash 在受保护 tag/正式发布前都不是永久公共身份。每次修复必须使用新 revision 并完整复验。
- 源码 manifest 与受信任候选对齐为 `1.0.0`，但不证明候选已发布；candidate manifest 仍必须保留 `status=local-candidate` 和 `external_publication_performed=false`。

## 回退

若复验失败，保持 `1.0.0` 未发布，修复后从新干净 revision 重建；不覆盖失败 evidence。若发现 v1 范围本身无法达到，需新 ADR 与用户决策后才能改变版本策略或验收范围。

## 验收

- 源码 Plugin manifest、最终候选内 Plugin/CLI/runner/Starter 与分发 manifest 均为精确 `1.0.0`。
- 双份候选逐字节一致，并且绑定新的干净 source revision。
- 新 release evidence 绑定 `1.0.0`、新 revision、新 archive/manifest hash、Marketplace ref、required CI 和完整生命周期。
- strict 12/12 PASS，独立最终只读审计为 0 Blocker/High，且所有 Low/Medium 已关闭或明确阻断。
- V1-12 在正式外部发布获得用户单独批准前保持 `NOT RUN`。
