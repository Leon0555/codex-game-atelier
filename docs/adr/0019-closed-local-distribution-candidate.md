# ADR 0019：同版本闭合的本地分发候选

- 状态：Accepted（M3 本地候选装配契约）
- 日期：2026-08-30
- 决策范围：Plugin、预构建 CLI/runner、Starter Template、许可证和 checksum 的版本闭合

## 背景

Plugin bundle 与 Starter Template 已分别具有确定性 archive 和内部 manifest，但分开验证不能证明用户拿到的是同一版本组合，也不能为严格发布门禁提供一个固定、可重验的分发根。M3 需要先建立本地候选闭包，再集中演练安装、升级、卸载与回滚；此步骤不能暗中变成用户级安装器或外部发布工具。

## 决策

1. 仓库维护工具 `tools/package_distribution.py` 只接受已通过现有静态验证的 Plugin bundle 与 Starter package，输出一个此前不存在的本地 candidate 目录。它不联网、不登录、不安装、不执行输入二进制、不创建 Git hook，也不发布到 GitHub、npm 或 Marketplace。
2. candidate 固定包含 Plugin archive/外部 checksum、Starter archive/外部 checksum、仓库原始 `LICENSE`、`NOTICE` 与最后生成的 `DISTRIBUTION-MANIFEST.json`。未知文件、symlink、hardlink、特殊文件、非 `0644` mode、大小或 Unicode case-folding 冲突均被拒绝。
3. Plugin、公共 CLI、private runner、Starter Template 与 Starter 所验证的 Plugin 使用完全相同的 SemVer。当前 `0.2.0` 只表示本地开发候选；最终 v1.0 版本号尚未因本 ADR 自动冻结。
4. 独立 `verify` 会重跑两个 component archive 的 bounded 静态验证、读取包内 manifest、核对版本闭合、文件 SHA-256/大小/mode、MIT License 与 NOTICE 原文。candidate manifest 本身由本地可信源码或未来可信发布渠道保护，不能自证发布者身份。
5. manifest 固定记录 `local-candidate` 与 `external_publication_performed=false`。`source_build_required=false`、`telemetry_enabled=false`、`hidden_external_writes=false` 与“不自动安装 Git hooks”是分发策略门禁；后续仍需动态文件/网络审计，不能仅凭字段宣布运行时通过。
6. Godot 游戏技术导出不要求签名/公证。框架自身 Plugin/CLI 下载物的签名、公证与 Gatekeeper 状态单独记录为 `NOT_EVALUATED`，不得借用游戏导出决定冒充已解决。
7. Windows/Linux 产物只做交叉编译和格式验证，原生运行仍为 `NOT_RUN`；candidate 的存在不扩大 Support Matrix。

## 备选方案

### 只发布两个独立 archive

拒绝。无法机器化证明配套版本，也容易在升级或回滚时组合出未验证版本。

### 把 candidate 装配器做成最终用户安装器

拒绝。安装所有权应由 Codex Plugin 客户端或明确批准的生命周期方案承担；维护脚本不能要求用户克隆源码或安装 Python。

### 直接把本地 candidate 标为 release candidate

拒绝。真实 Codex 安装/发现、生命周期、动态外部写入审计、required CI 与最终独立审计尚未通过。

## 风险

- archive 与 checksum 同源，不能独立证明发布者身份；正式发布仍需要可信渠道和 package provenance。
- 当前 framework artifact 的 Gatekeeper/quarantine 行为未评估。
- 验证器针对静止目录设计；同机恶意并发写入不在该本地维护工具的信任边界内，正式发布应在不可变干净 workspace 中运行。
- `0.2.0` 不等于 v1.0 对外版本承诺。

## 迁移与回退

- candidate 输出永不覆盖既有目录；失败只清理本次新建且未完成的输出。
- component 版本不一致时在创建 candidate 前阻断。
- manifest schema 破坏性变化需要新 ADR/版本；旧 candidate 保持可由旧验证器验证。
- 生命周期演练失败不得修改用户游戏项目或凭据，且不得把失败 candidate 切换为 active version。

## 验证

- 相同 Plugin/Starter 输入生成两份逐字节一致的 candidate。
- component 版本不匹配、已有输出、archive/NOTICE 篡改与未知文件负例。
- 当前 Apple Silicon trusted Plugin bundle 入口 smoke；Universal 2 静态双 slice。
- Linux ELF amd64 与 Windows PE32+ amd64 交叉构建/格式检查；原生执行保持 `NOT_RUN`。
