# M3 Strict 本地分发门禁

- 日期：2026-08-31
- 实现提交：`de3f7420d346222a515b3ffc07b9a76b01807fe2`
- 候选 A：`.tools/distributions/codex-game-atelier-0.2.0-m3-strict-de3f742-a/`
- 候选 B：`.tools/distributions/codex-game-atelier-0.2.0-m3-strict-de3f742-b/`
- 执行入口：候选 A Plugin 内的 macOS Universal 2 public CLI
- 结论：本地 clean-source、Plugin、Starter、license/provenance 四项 strict gate 已机器化 PASS；required CI 仍为 `NOT_RUN`，当前参考项目 run store 仍 `BLOCKED`

## 1. 实现边界

`release check --mode strict --distribution-candidate <directory>` 只读完成：

- distribution manifest `1.1.0`、固定八个顶层成员、文件 hash/size/mode 与外部 checksum；
- gzip 单流、tar root/path/type/owner/time/mode/member/展开大小边界，不解包到磁盘；
- Plugin/Starter 固定 allowlist、包内 manifest、inventory、版本配对与 no-embedded-plugin；
- 根/包内 MIT、NOTICE、Go `THIRD_PARTY_NOTICES` 一致性和必要条款；
- public/private、macOS `amd64 + arm64`、Linux/Windows amd64 共六文件八条 Go build info；
- clean revision、精确 Go、module/package、GOOS/GOARCH、`-trimpath`、`CGO_ENABLED=0`、`vcs.modified=false`；
- 分发文本无具体模型 ID，内部 `AGENTS.md` 文件不在固定路径集合。

CLI 不执行包内代码、不调用 Python/shell/npm、不联网、不安装、不写 project/candidate，也不把 candidate 绝对路径或 archive 自由文本放进 stdout。未提供 candidate 时四项为 `NOT_RUN`；验证失败时四项保守为 `BLOCKED`。

## 2. Clean candidate 与重现性

- A/B 均从 clean `de3f742...` 使用 Go `1.27.0` 构建。
- 两套 Plugin trusted native smoke PASS；A/B distribution `diff -rq` 无输出。
- manifest provenance：`source_clean=true`、`source_revision=de3f742...`、`trimpath=true`、`cgo_enabled=false`、`binary_file_count=6`、`binary_build_record_count=8`。

| 产物 | SHA-256 |
| --- | --- |
| Distribution manifest | `309a4535cfc64ccfca282d391c572079a0356b726b44530d4b7dce9875046283` |
| Plugin archive | `ae5edabcb3bddb326235ab21a2b0d93b9c4864b70a7530290b89ebf78e29d830` |
| Starter archive | `14837f802739c4d705bce625eecfdf132681ddd14616bcca52810a120afd8d3e` |

## 3. 实际 strict 结果

候选 A 内 CLI 对 `examples/reference-game` 与候选 A 执行，退出 `4`、整体 `BLOCKED`，不是伪 PASS：

| 门禁组 | 结果 |
| --- | --- |
| project-state / support-scope | 2 PASS |
| run-store-integrity | 1 BLOCKED（保留的历史 corrupt records） |
| latest headless/test/release export | 3 PASS |
| clean-source/plugin/starter/license-provenance | 4 PASS |
| required-ci | 1 NOT_RUN |
| 合计 | 9 PASS / 1 BLOCKED / 1 NOT_RUN；`release_ready=false` |

命令前后对 `.gameatelier` 和 candidate 全部 regular files 计算 SHA-256，快照完全相同。candidate 参数在 command-result 中只记录 `distribution_candidate=provided`。

## 4. 回归与负例

| 验证 | 结果 |
| --- | --- |
| Go vet | PASS |
| Go tests | PASS；`internal/app` 44.792 秒 |
| Python validators | PASS，51/51 |
| Draft 2020-12 schemas | PASS，25 schemas、30 fixtures、31 negative assertions |
| 本地真实 candidate Go verifier | PASS |
| 拼接 gzip | 拒绝 |
| symlink tar member | 拒绝 |
| 未知 archive member | 拒绝 |
| 缺失显式 `false` provenance 字段 | 拒绝 |
| candidate 用于非显式 strict | usage 退出 2 |
| verifier 失败 | 四项本地分发 gate 均 BLOCKED |
| verified candidate 但 required CI 缺失 | 保持 BLOCKED；不能 `release_ready=true` |

## 5. 未完成

- 这不是最终独立审计；实现者没有自批最终发布。
- 当前参考项目的历史 corrupt run 记录未删除；不能为获得绿色结果而清理失败 evidence。
- GitHub-hosted required CI、Python wheel hashes、release attestation/SBOM 仍未运行。
- Framework Gatekeeper、Windows/Linux 原生证据、成功/失败升级与回滚、最终干净用户环境仍未完成。
- 没有 remote、tag、Release、npm/Marketplace 发布或用户发布批准。

## 6. Fresh Starter 后续复验

同一候选随后在新解包的中文、空格与 `#` 路径 Starter 上完成 initialize、Headless、6/6 tests、Debug/Release build、直接 Release export 与 Apple Silicon target smoke。standard 聚合为 6 PASS；strict 聚合为 10 PASS、1 `NOT_RUN`，只剩 required hosted CI。完整命令、run ID、产物摘要、受限沙箱负例和只读树摘要见 [`m3-fresh-starter-candidate-e2e-2026-08-31.md`](m3-fresh-starter-candidate-e2e-2026-08-31.md)。
