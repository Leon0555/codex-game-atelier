# M3 rc.6 单机隔离式 V1-09 政策审计

- 日期：2026-09-08（Asia/Shanghai）
- 审计对象：ADR 0027、V1-09、当前范围/状态文档与既有 rc.6 evidence
- 审计方式：独立只读审计；审计者未修改仓库
- 最终结论：**PASS；0 Blocker、0 High、0 Medium、0 Low**

## 1. 决策与证据充分性

审计确认 ADR 0027 没有删除远程安装、候选绑定、Godot 实际运行、生命周期、CI 或终审要求，只取消第二 macOS UID/第二台实体机器的强制重复。新的干净环境定义具体且可复验，并明确不声称多用户或多机器验证。

rc.6 既有证据覆盖：

- 固定 clean source revision 与两次可复现构建。
- 未复制现有配置/凭据的全新隔离 `CODEX_HOME`。
- 固定 GitHub Marketplace ref 的远程 Plugin 安装，以及 cache/bundle 逐文件一致。
- 此前不存在的中文、空格和特殊字符项目路径。
- Godot 4.7.2 Headless、固定 GDScript 6/6、Debug/Release 技术导出与 Apple Silicon target smoke。
- 真实用户级升级、损坏升级拒绝、回滚、重装、卸载和状态精确恢复。
- 无系统设置/Gatekeeper 绕过或 quarantine 移除。
- required CI、绑定 strict 12/12 与独立最终只读审计。

GitHub 托管 Apple Silicon CI 只被描述为另一机器上的源码构建、Go、Schema、分发契约和原生 CLI pair smoke，没有冒充完整 Godot/Plugin 生命周期重复。

## 2. 文档一致性修复

初审发现两项非阻断 Low：README 的 ADR 概览停在 0026；架构文档仍把已完成的 rc.6 终审写成待办。主 owner 修复后，原审计者重新从磁盘只读复核：两项均已关闭，未引入新问题，最终计数为 0/0/0/0。

当前 README、项目简报、架构、路线图、Support Matrix、V1-09、ADR 0025/0027 与 rc.6 说明使用同一边界。历史 rc.2–rc.5 文档和 rc.6 机器可读 JSON 均未改写。

## 3. 发布边界

- V1-09：PASS；采用 ADR 0027 的单机隔离式证据。
- V1-12：NOT RUN；用户尚未批准正式发布。
- 受保护版本 ref/tag、正式 Plugin 发布和 GitHub Release：未执行。
- 本次决策不需要产品代码或 Schema 变更，不改变 rc.6 candidate hash。
