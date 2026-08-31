# M3 供应链修复与本地候选复审

- 日期：2026-08-31
- 实现提交：`ea3da8ec66c75504d8b2c857b956f59e904b8f56`
- 候选：`.tools/distributions/codex-game-atelier-0.2.0-m3-clean-ea3da8e-a/`
- 对照候选：`.tools/distributions/codex-game-atelier-0.2.0-m3-clean-ea3da8e-b/`
- 范围：修复 SC-001/SC-002，收紧本地 provenance 与 notice 契约，并只读复查剩余发布阻断
- 结论：clean-build provenance 与 Go notice 两项 High 已在本地候选关闭；framework Gatekeeper、三宿主 Tier 1、远程 CI/发布链、升级/回滚和最终独立审计仍未完成

本记录不是最终独立审计。实现与复查由同一主任务完成，因此只证明修复已产生可重复的本地证据，不能替代发布前独立只读终审。

## 1. 已完成修复

### SC-001 dirty build provenance：RESOLVED（本地候选）

- Plugin 打包器在创建输出前要求 clean Git worktree 和完整 40 位 revision。
- 调用方必须显式传入实际 Go tool；manifest 记录精确 `go1.27.0`。
- public CLI/private runner 的 macOS `amd64 + arm64`、Linux amd64、Windows amd64 共六个文件、八个 build record 均实际检查。
- 八条记录一致包含 `-trimpath=true`、`CGO_ENABLED=0`、正确 module/package/GOOS/GOARCH、`vcs.revision=ea3da8e...` 与 `vcs.modified=false`。
- 二进制复制进 Plugin 后再次检查包内字节的 build metadata；来源变化不能沿用复制前 provenance。
- Plugin 与 distribution manifest schema 升至 `1.1.0`；distribution 必须逐字段等于 Plugin provenance。
- 旧 `1.0.0` candidate 使用新验证器复验时退出 `1`，不会被静默升级成新契约 PASS。

### SC-002 Go notice：RESOLVED（本地候选）

- 仓库新增完整 `THIRD_PARTY_NOTICES`，包含 Go Authors copyright、二进制再分发条件和免责声明。
- `NOTICE` 与 `docs/provenance.md` 明确预构建 CLI/runner 的 Go 标准库关系。
- Plugin bundle 与总 candidate 固定携带并逐字节核对仓库 notice；缺失、篡改或未知内容均失败。
- Starter Template 不包含 Go 二进制，因此不虚构同一运行时依赖。

## 2. 重现性与候选摘要

同一 clean commit 独立执行 A/B 两套构建：

- 8 个 Go 薄架构入口逐文件 `cmp`：全部相同。
- 2 个 Universal 2 合并入口逐文件 `cmp`：全部相同。
- 两套 Plugin trusted native smoke：均 PASS；CLI 返回 `0.2.0`，private runner 固定退出 `125`。
- 两套 Starter Package 与两套 Distribution verifier：均 PASS。
- A/B candidate `diff -rq`：无输出。

候选 manifest 记录：

| 字段 | 值 |
| --- | --- |
| source revision | `ea3da8ec66c75504d8b2c857b956f59e904b8f56` |
| source clean | `true` |
| Go | `go1.27.0` |
| trimpath / CGO | `true` / `false` |
| binary files / build records | `6` / `8` |
| candidate files | `7`（不含 manifest 自身） |
| candidate expanded bytes | `25,276,432` |
| Distribution manifest SHA-256 | `81033229f73e4bb79ac90bffb85ace889daa986bc3e1dcf0957143856b2fab69` |
| Plugin archive SHA-256 | `db088d3b4788f876b0c3ab6847a2613a1bbc4154aeaed12d4da035c88d912947` |
| Starter archive SHA-256 | `14837f802739c4d705bce625eecfdf132681ddd14616bcca52810a120afd8d3e` |
| third-party notice SHA-256 | `7055aea9a5203534e38b838951401c19bede9a34c7515b6966a111019178f2e6` |

## 3. 本轮验证

| 验证 | 结果 |
| --- | --- |
| Python validators | PASS，51/51 |
| Draft 2020-12 schemas | PASS，25 schemas、30 fixtures、31 negative assertions |
| Go formatting | PASS，无差异 |
| Go vet | PASS |
| Go tests | PASS；`internal/app` 50.627 秒 |
| Go module closure | PASS；`go list -m all` 只有生产 module，非标准依赖只有仓库四个 package |
| runtime network/telemetry implementation scan | PASS（静态）；产品代码未命中 `net/http`、HTTP client、telemetry、analytics 或 upload |
| actual package metadata | PASS；8/8 records 为 clean revision |
| Plugin/Starter/Distribution A/B reproducibility | PASS |
| old candidate rejection | PASS；旧 schema 验证退出 1 |
| strict release check | BLOCKED/4；5 PASS、1 BLOCKED、5 NOT_RUN，与未接线状态一致 |

## 4. 仍未关闭的发布条件

1. **Framework Gatekeeper**：新 Universal 2 CLI 可在本机 trusted smoke，但 `codesign --verify --strict` 报告未签名，`spctl` 退出 1。后续本地 Spike 已证明 quarantine 可由 archive 传播到解包 Plugin，public/private 入口会阻塞且不能完成无人值守执行；此项不是 `NOT_EVALUATED`，而是当前候选明确 `FAIL`。它不等于 Godot 游戏技术导出签名要求，详见 [`m3-framework-gatekeeper-spike-2026-08-31.md`](m3-framework-gatekeeper-spike-2026-08-31.md)。
2. **Windows/Linux Tier 1**：只完成 reproducible cross-build 和格式/build-metadata 验证；原生执行仍 `NOT RUN`。现有三宿主 Tier 1 声明与延期决定仍需最终范围决策。
3. **Required CI / Python 下载哈希**：CI 的最低 Go 从浮动 `1.24.x` 收紧为精确 `1.24.0`；Python validator 虽固定版本，但本机没有可验证的 CI Python 3.13 wheel 集，未在禁止远程读取的本轮伪造 `--require-hashes`。首次 GitHub-hosted run、release workflow、OIDC attestation、SBOM 仍 `NOT RUN`。
4. **Lifecycle/clean environment**：真实升级、失败升级、回滚和最终干净用户环境复验按用户决定留到最终候选。
5. **Final audit/publication**：尚未执行独立只读终审、remote/tag/GitHub Release/npm publish/Marketplace 发布或用户正式发布批准。

## 5. 下一顺序

ADR 0022 已把 Plugin/Starter/license provenance 作为显式只读输入接入 strict release gate，并保持 required CI 为 `NOT_RUN`，实证见 [`m3-strict-distribution-gate-2026-08-31.md`](m3-strict-distribution-gate-2026-08-31.md)。fresh Starter 本地 E2E 也已闭合，但 framework quarantine Spike 已把 Gatekeeper 从未知收敛为当前候选失败。后续只剩干净环境、framework 分发策略、Support Matrix 决策及远程/生命周期门禁；到需要 Developer ID/notarization、真实下载、升级/回滚、GitHub-hosted CI 或其他远程/账号操作时，先向用户说明作用域并取得授权。
