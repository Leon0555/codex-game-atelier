# M3 rc.6 最终只读审计

- 日期：2026-09-08（Asia/Shanghai）
- 审计对象：`0.3.0-rc.6`、源码 `cad37695219946c751cf3ebaa5ecd6dfd31d9fc1`、Marketplace revision `29b25e51b8e79ba948c4c75b2104abfc0b1e1096`、绑定 release evidence 与当前状态文档
- 审计方式：独立只读审计；审计者未修改仓库
- 最终结论：**PASS；0 Blocker、0 High、0 Medium、0 Low**

该结论允许 rc.6 通过本轮架构、安全、许可与发布前终审，不等于批准正式发布，也不把 V1-09 或 V1-12 补成通过。

## 1. 审计范围与结论

| 范围 | 结论 |
| --- | --- |
| rc.3 全局宿主门禁 High | PASS；13 个公开命令在参数解析、项目读取或写入前统一拒绝非 macOS arm64 宿主；精确 `--version` 保持无操作例外 |
| rc.4/rc.5 Plugin prompt 问题 | PASS；打包器强制 1–3 条、非空、单条不超过 128 字符；rc.6 实际长度 92/60/111 |
| 产品范围 | PASS；Godot-only、Plugin-only、macOS Apple Silicon-only；Intel/Windows/Linux 只作 artifact inventory，不冒充原生验证 |
| 分发内容边界 | PASS；未发现具体模型 ID、内部 `AGENTS.md`、Unity 宣传或旧 Foundry 名称 |
| 可复现构建与供应链 | PASS；A/B bundle 与 distribution 一致；archive/checksum、MIT、NOTICE、THIRD_PARTY_NOTICES、Go clean provenance 复验通过 |
| 远程 Plugin 与生命周期 | PASS；远程 Marketplace Plugin 子树与 bundle A 一致；远程取得和相同树的真实用户级生命周期已明确区分并绑定 rc.6 |
| Godot E2E | PASS；特殊路径 Starter、initialize、Headless、GDScript 6/6、Debug build、Release export 和 arm64 smoke 均有当前候选证据 |
| strict 与只读性 | PASS；重跑退出 0，12 PASS、0 BLOCKED、0 NOT_RUN、`release_ready=true`；项目、候选与 evidence 文件树不变 |
| required CI 与分支保护 | PASS；两个目标 GitHub Actions run success；`main` strict required check、admin enforcement、禁止 force push/deletion 均现场复核 |

## 2. 初审 Low 与独立复核

初审发现 1 项非阻断 Low：`docs/project-brief.md` 的当前阶段仍停留在 rc.3，误把 rc.4 修复写成下一步。该内容保守滞后、未进入 Plugin bundle，也未扩大支持范围。

主实现 owner 更新该段，使其准确覆盖 rc.2–rc.5 历史失败、rc.6 已验证范围与尚未执行事项。原审计者随后只读复核：Low 已关闭，`git diff --check` 通过，未引入新问题。最终计数因此为 0/0/0/0。

## 3. 审计复验

- Go formatting/vet/tests：PASS。
- Python validators：53/53 PASS。
- Schema/fixture/negative assertions：PASS。
- Plugin bundle、trusted native smoke、archive、distribution 与生成态 Marketplace：PASS。
- A/B 逐文件一致，GitHub Marketplace Plugin 子树与 bundle A 一致。
- strict `release check`：12/12 PASS，且输入树前后摘要不变。
- 远程 CI 与当前 branch protection：只读复核 PASS。

## 4. 明确保留的 NOT RUN

- V1-09：独立 macOS 用户或第二台 Apple Silicon 机器复验。
- V1-12：用户正式发布批准。
- 受保护版本 ref/tag、正式 Plugin 发布和 GitHub Release。

Windows/Linux 原生验证、npm、standalone archive、DMG/PKG、Apple 签名与公证不属于 v1 门禁，不以 `NOT RUN` 阻断本次审计。

## 5. 审计后的 V1-09 决策

本报告完成时 V1-09 仍要求独立 macOS UID 或第二台 Apple Silicon 机器，因此第 4 节保留当时的 `NOT RUN` 结论。用户随后通过 ADR 0027 选择单机隔离式最终复验，并由新的独立只读政策审计确认现有 rc.6 证据满足该定义。当前 V1-09 状态以 [`m3-rc6-single-machine-policy-audit-2026-09-08.md`](m3-rc6-single-machine-policy-audit-2026-09-08.md) 和验收基线为准；本报告的原始事实与计数未被回写。
