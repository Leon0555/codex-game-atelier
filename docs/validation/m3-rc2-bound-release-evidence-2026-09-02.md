# M3 rc.2 绑定式发布证据

- 日期：2026-09-02（Asia/Shanghai）
- 候选版本：`0.3.0-rc.2`
- 源码提交：`280de4a9939bb437379ad4c743df57b1d9ce9aba`
- Marketplace ref：`marketplace/v0.3.0-rc.2`
- Marketplace revision：`577bccd217d2a9a563f0495c6499745db32a8f1d`
- 绑定 evidence：[`evidence/m3-release-evidence-2026-09-02/release-evidence.json`](evidence/m3-release-evidence-2026-09-02/release-evidence.json)
- 预审结论：候选的本地分发、远程 Plugin、新任务 Skill 发现、生命周期和 required CI 形成了 1.0.0 机器可读闭包；strict `release check` 为 12/12 PASS、`release_ready=true`。随后最终独立审计发现 1 High、1 Medium，因此 rc.2 已被拒绝，不得晋升；详见 [`m3-rc2-final-readonly-audit-2026-09-02.md`](m3-rc2-final-readonly-audit-2026-09-02.md)。

## 1. 候选与绑定

`0.3.0-rc.2` 从 clean source revision 构建两次，Plugin bundle A/B 与 distribution candidate A/B 均逐字节一致。绑定摘要为：

| 对象 | SHA-256 |
| --- | --- |
| `DISTRIBUTION-MANIFEST.json` | `fbc2017fae87303cb752765e457095c95850abccfc64e615dbe9c458e43dea2d` |
| Plugin archive | `a2429934318ed5dded9d39240e7a7fbf9d50a66b34e49b9794b1794b565e73f1` |

分发 manifest 保持 `local-candidate` 和 `external_publication_performed=false`；远程事实由独立、只读的 release evidence 输入补充，不回写候选自述。

## 2. 远程 Plugin 与 Godot 闭环

GitHub Marketplace 分支首次安装前发现 Marketplace 内部名称仍为本地测试名，安装器因此不能用正式 selector 安装。远程测试分支修复为 `codex-game-atelier` 后重新安装；该修复只改变 Marketplace identity，不改变候选 Plugin archive。隔离 `CODEX_HOME` 中完成：

| 检查 | 结果 |
| --- | --- |
| Git-backed Marketplace clone 与 Plugin install | PASS |
| 安装 cache 与本地 bundle A 逐文件比较 | PASS；无差异 |
| public CLI `--version` | PASS；`0.3.0-rc.2` |
| private runner 直接调用拒绝 | PASS；退出 `125` |
| 特殊路径 Starter 创建与初始化 | PASS |
| Godot 4.7.2 Headless validation | PASS；8/8 checks |
| 固定 GDScript 测试 | PASS；6/6 |
| 下载属性 | 只有 `com.apple.provenance`；无 quarantine |
| Gatekeeper/System Settings/xattr 绕过 | 均未发生 |

第一次未显式提供命令级代理的隔离 clone 在当前网络环境超时；使用既有 LAN 代理完成远程 Git 读取。代理只解决网络路由，不是 Gatekeeper、系统设置放行或 quarantine 移除。

## 3. 用户级最终候选生命周期

操作前私有快照只记录权限、大小与 SHA-256，不输出配置正文。起始状态为 13 个已安装 Plugin、5 个 Marketplace、0 个 Atelier 条目；`config.toml` 权限 `0600`、大小 `6308`、SHA-256 `6a15ef7294c6b29b0cd57c1b882a2de3bb7c32d17f025f08a3d33d2c10f8b837`。

1. 从远程 `rc.1` 成功升级到 `rc.2`，安装 cache 与候选逐文件一致。
2. 第一次故障注入先移除了当前 Marketplace，失败后没有保留 active Atelier；该尝试如实记为无效测试设计，不计 PASS。
3. 恢复 `rc.2` 后，让正式 Marketplace 与独立命名的无效远程 Marketplace 并存。不可解析 `plugin.json` 的安装明确退出 `1`，而 `rc.2` 始终是唯一 active Atelier 版本；失败升级门禁 PASS。
4. 无效 Marketplace 被移除，远程临时故障分支被删除。
5. `rc.2 → rc.1` 回滚 PASS；实际 CLI 返回 `0.3.0-rc.1`，cache 与已审计 rc.1 bundle 逐文件一致。
6. 重新安装 `rc.2`，供新任务发现验证。
7. 显式卸载 Plugin 和 Marketplace。结束时恢复为原 13 个 Plugin、5 个 Marketplace、0 个 Atelier 条目。
8. Codex CLI 会重写 `config.toml`；使用演练前私有快照恢复唯一该文件后，权限、大小、SHA-256 和逐字节比较均与起点一致。空 Atelier cache 和私有快照随后精确删除。

## 4. 新任务 Skill 发现

在用户级远程 `rc.2` 活动时创建只读临时 Codex 任务 `01a05e59-7eaa-77b1-814d-ebc234bb9398`。该任务没有继承当前任务的说明，仍自动发现并读取：

`/Users/leon/.codex/plugins/cache/codex-game-atelier/codex-game-atelier/0.3.0-rc.2/skills/develop-godot-game/SKILL.md`

任务报告 Skill 名称 `codex-game-atelier:develop-godot-game`，并正确概括其 Godot/GDScript 工作流用途；没有创建项目或修改文件。验证完成后任务已归档。

## 5. Required CI 与分支保护

GitHub Actions run [`33521593327`](https://github.com/Leon0555/codex-game-atelier/actions/runs/33521593327) 在 `main` 的候选源码提交上完成，workflow `.github/workflows/ci.yml`、job `verify-macos-arm64`、结论 `success`。

现场只读复核确认 `main` 已配置：

- required status check：`verify-macos-arm64`；`strict=true`。
- `enforce_admins=true`。
- force push 与 branch deletion 均关闭。
- required signatures 关闭；v1 不把提交签名或 Apple 公证作为该门禁。

## 6. rc.2 新鲜项目与 strict 聚合

候选 Plugin 在新的中文、空格、`#` 路径创建 Starter 并初始化；Headless 8/8、GDScript 6/6 和 Release technical export 均 PASS。Release ZIP 为 Universal 2，并实际完成 Apple Silicon target smoke；不声称 Intel 实机验证，也不要求游戏签名/公证。

第一次 export 因使用未与项目本地 export templates 相邻的 Godot App 路径而返回 `GODOT_EXPORT_TEMPLATES_MISSING`；失败 run 保留。改用已配套同一官方 4.7.2 二进制和项目本地 templates 的 `Godot-Atelier` 入口后 PASS。

最终命令：

```text
codex-game-atelier release check --project <fresh-rc2-project> --mode strict --distribution-candidate <rc2-candidate> --release-evidence <bound-evidence>
```

结果：退出 `0`，12 PASS、0 BLOCKED、0 NOT_RUN、`release_ready=true`。命令只在 stdout 记录两个输入为 `provided`，没有回显绝对路径，也没有修改项目、candidate 或 evidence。

## 7. 未覆盖与后续门禁

- 按用户决定，独立 macOS 用户或第二台 Apple Silicon 机器复验延后到最终 RC；当前仍为 `NOT RUN`。
- Windows/Linux 原生验证不属于 v1 支持或发布门禁。
- 正式版本 ref/tag、Plugin 正式发布和用户发布批准尚未执行。
- npm、独立二进制下载、DMG/PKG、Apple 签名与公证不属于 v1 Plugin-only 路径。
- 仍需在最终文档状态上完成独立只读架构、安全、许可与发布终审。

## 8. 最终审计结果

最终审计确认上述命令和 hash 事实真实，但发现 rc.2 CLI 仍把 Windows/Linux amd64 判为 v1 supported，与 ADR 0025 冲突；同时 1.0.0 evidence 对生命周期和 branch protection 只保存 PASS 声明，独立复核信息不足。rc.2 状态因此为 `FAIL`。修复后的 1.1.0 contract 和 macOS-only runtime 必须通过全新的 rc.3 闭环。
