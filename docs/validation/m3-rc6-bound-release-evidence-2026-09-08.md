# M3 rc.6 绑定式发布证据

- 日期：2026-09-08（Asia/Shanghai）
- 候选版本：`0.3.0-rc.6`
- 源码提交：`cad37695219946c751cf3ebaa5ecd6dfd31d9fc1`
- Marketplace ref：`marketplace/v0.3.0-rc.6`
- Marketplace revision：`29b25e51b8e79ba948c4c75b2104abfc0b1e1096`
- 绑定 evidence：[`evidence/m3-release-evidence-2026-09-08/release-evidence.json`](evidence/m3-release-evidence-2026-09-08/release-evidence.json)
- 当前结论：候选构建、远程 Plugin 取得、真实用户级生命周期、Skill 发现、Godot E2E、required CI、strict 聚合与独立最终只读审计均已 PASS。该结论不是正式发布批准。

## 1. 候选与绑定

`0.3.0-rc.6` 从受保护 `main` 的 clean source revision 构建两次。两份 CLI/runner、Plugin bundle 和 distribution candidate 均逐字节一致。

| 对象 | SHA-256 |
| --- | --- |
| `DISTRIBUTION-MANIFEST.json` | `18172e2a48cb49249374bca8ad2c960e719450d79b51f1fe9f156a410b83a651` |
| Plugin archive | `cf9512224dc7b33c96f59b28cd5456a196b6d463297fa9b6876ae03b2982ceda` |

Plugin manifest 的三条 `defaultPrompt` 长度为 92、60、111 字符；打包器拒绝零条、超过三条、空字符串或超过 128 字符的条目。rc.4 的数量问题与 rc.5 的长度问题均进入负向回归测试，没有覆盖历史失败记录。

包内 macOS CLI/runner 和游戏技术导出均为 Universal 2；只有 arm64 在 Apple Silicon 上实际运行。Windows/Linux 文件仍只是 artifact inventory，不属于 v1 原生支持。

## 2. 远程取得与真实加载

全新隔离 `CODEX_HOME` 通过 GitHub sparse Marketplace ref 取得并安装 rc.6；安装 cache 与 bundle A 逐文件一致，CLI 返回 `0.3.0-rc.6`。private runner 直接调用明确拒绝并退出 `125`。

隔离环境未复制用户凭据。它在模型响应前完成本地 Plugin manifest 解析，未再出现 Atelier `defaultPrompt` 数量或长度 warning；随后模型请求返回 `401` 属于预期认证隔离，不能冒充 Skill 发现结果。

在真实用户登录态下激活同一 rc.6 Plugin 树后，以 `--ephemeral --sandbox read-only` 启动任务 `01a07eac-e94f-7261-89b9-c9a9678b935c`。任务返回：

- `present=true`
- Skill：`codex-game-atelier:develop-godot-game`
- 版本：`0.3.0-rc.6`
- 路径：`/Users/leon/.codex/plugins/cache/codex-game-atelier/codex-game-atelier/0.3.0-rc.6/skills/develop-godot-game/SKILL.md`

启动日志仍出现外部环境噪声：远程 Plugin catalog EOF、WebSocket 证书异常、若干现有 Plugin 的 catalog-refresh warning，以及不属于 Atelier Skill 的图标路径 warning。Atelier 也被包含在一次“远程目录刷新未发现已配置非 curated Plugin”的批量 warning 中，但同一任务随后从本地 cache 正确发现了它；这不是 manifest 解析 warning，也没有阻断加载。没有修改系统设置、移除 quarantine 或执行 Gatekeeper 绕过。

## 3. 用户级生命周期与精确恢复

远程取得与生命周期状态机分开验证：生命周期使用与远程 cache 逐文件一致的本地 Marketplace worktree，避免把网络重试混入升级/回滚语义。真实用户级操作依次完成：

1. 安装 previous version `0.3.0-rc.5`。
2. `rc.5 → rc.6` 升级；rc.6 为唯一 active 版本，cache 与候选一致。
3. 并存的无效 Marketplace 提供不可解析 `plugin.json`；安装退出 `1`，rc.6 仍保持 active。
4. `rc.6 → rc.5` 回滚，实际 cache 与 rc.5 候选一致。
5. 重新安装 rc.6，并完成登录态 Skill 发现。
6. 显式卸载 Plugin 与 Marketplace，二者退出 `0`。
7. 用演练前私有快照恢复 `~/.codex/config.toml`；逐字节比较、mode `0600`、5955 bytes 与 SHA-256 `cbc4faaece632f785d8d3441216fbf2a39f85c493372217eaba9815f39bfe262` 全部一致。
8. Atelier cache、Marketplace 目录和配置条目均为零。

本次快照按配置中固定排序的本地条目计算：14 个启用 Plugin、6 个 Marketplace。Plugin ID 清单 hash 为 `88cc8c979f040b7b9774f4ab6198d0530610fecf6a389048ca5a413f87404341`；Marketplace 名称清单 hash 为 `fa7db3991654ad43637e2b48b948bcce8b340d10adf1dd84faf97bd04653ae2d`。前后完全一致。旧文档中的 23/5 来自当时远程 catalog 枚举口径，不再复用为本次现场状态。

## 4. 远程安装包 Godot E2E

测试项目由远程安装 cache 中的 CLI 在新目录 `.tools/M3 rc6 远程安装 #20260908/codex-game-atelier-starter` 创建；该路径同时覆盖中文、空格与 `#`。

| 检查 | 结果 |
| --- | --- |
| Embedded Starter create | PASS；11 files、13,617 bytes |
| Atomic initialize | PASS；state schema 1.0.0 |
| Godot 4.7.2 Headless validation | PASS；8 checks |
| 固定 GDScript tests | PASS；6/6 |
| standard Debug build | PASS；Universal 2，arm64 target smoke 退出 0 |
| standard Release export | PASS；Universal 2，arm64 target smoke 退出 0 |

Debug ZIP SHA-256 为 `b58aaf82db7846e3563c9735e027e9ec68945da8ee65b3d7f686fc8d49b5d070`；Release ZIP SHA-256 为 `d080aa6415a9c303f6ff3d2af20ffe9139f951200589b90699abe2abd04a249e`。两份 manifest 均明确 `unsigned=true`、`not_notarized=true`、`public_distribution_ready=false`，符合 Plugin-only 且不把游戏 ZIP 作为独立公开下载包的 v1 决策。

## 5. Required CI 与分支保护

GitHub Actions run [`34175920843`](https://github.com/Leon0555/codex-game-atelier/actions/runs/34175920843) 在候选源码提交 `cad3769` 上完成，event `push`、branch `main`、job `verify-macos-arm64`、结论 `success`。Marketplace ref run [`34176162334`](https://github.com/Leon0555/codex-game-atelier/actions/runs/34176162334) 同样为 `success`。

现场只读 API 复核确认 `main`：

- required status check 为 `verify-macos-arm64`，`strict=true`。
- `enforce_admins=true`。
- force push 与 branch deletion 均关闭。
- required signatures 关闭；Apple 签名/公证不属于 v1 Plugin-only 门禁。

## 6. Strict 聚合

输入为上述 fresh rc.6 项目、distribution candidate A 和 1.1.0 evidence。实际命令退出 `0`，只读返回：

| 结果 | 数量 |
| --- | ---: |
| PASS | 12 |
| BLOCKED | 0 |
| NOT_RUN | 0 |
| `release_ready` | `true` |

十二项依次覆盖 project state、support scope、run-store integrity、最新 Headless、固定 GDScript tests、最新 Release export、clean-source、Plugin bundle、embedded Starter、license/provenance、远程 Plugin/lifecycle 与 required CI。参数路径只记录为 `provided`。

## 7. 全量回归

- Go：`gofmt` 无差异、`go vet ./...` PASS、`go test -count=1 ./...` PASS。
- Python validators：53/53 PASS。
- Draft 2020-12：27 schemas、33 fixtures、11 persisted Starter records、13 persisted collaboration records、44 negative assertions PASS。
- Plugin bundle A/B structural verify PASS；trusted native smoke PASS；Plugin archive verify PASS。
- Distribution candidate A/B verify PASS，A/B 逐文件一致。
- 生成态 local Marketplace verify PASS；其 Plugin 树与 bundle A 及 GitHub Marketplace worktree 一致。
- 分发包具体模型 ID、`AGENTS.md`、Unity 与旧 Foundry 名称扫描为零结果。

一次初始 Go 回归命令因非登录 shell 没有项目本地 Go 的 `PATH` 而退出 `127`；改用已批准工具链的绝对路径后全部通过。一次错误地把含完整源码的 Git worktree 传给“仅接受生成态 Marketplace 根”的 verifier，按固定根目录契约被拒绝；改对生成态 payload 执行后 PASS，且 payload 与 worktree 的 Plugin 子树一致。这两项是测试调用问题，不覆盖或删除。

## 8. 尚未完成

- 独立架构、安全、许可与发布最终只读审计已 PASS，最终计数为 0 Blocker、0 High、0 Medium、0 Low；详见 [`m3-rc6-final-readonly-audit-2026-09-08.md`](m3-rc6-final-readonly-audit-2026-09-08.md)。
- 后续决策：用户通过 ADR 0027 接受本记录中的单机隔离式干净环境组合证据，不再要求独立 macOS UID 或第二台实体机器；V1-09 因此改为 PASS。该变更不声明多用户/多机器验证，独立只读政策审计最终 0 Blocker、0 High、0 Medium、0 Low；详见 [`m3-rc6-single-machine-policy-audit-2026-09-08.md`](m3-rc6-single-machine-policy-audit-2026-09-08.md)。
- 受保护版本 ref/tag、正式 Plugin 发布、GitHub Release 和用户发布批准均未执行。
- Windows/Linux 原生验证、npm、standalone archive、DMG/PKG、Apple 签名/公证不属于 v1 发布路径。
