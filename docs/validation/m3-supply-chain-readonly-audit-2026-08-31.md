# M3 供应链与发布前只读审计

> 后续状态：SC-001/SC-002 已由后续供应链修复关闭；SC-003 的“最终候选至少先签名”建议已由 ADR 0023 取代。v1 先验证真实远程 Plugin 无阻断安装，不预先实施 Apple 公证。本文保留审计当时的原始发现，不能单独作为当前发布计划。

- 日期：2026-08-31
- 审计基线：`48fab50`
- 候选：`.tools/distributions/codex-game-atelier-0.2.0-m3-final/`
- 候选版本：`0.2.0`，`local-candidate`
- 宿主：macOS Apple Silicon
- 审计方式：只读检查源码、候选、展开 Plugin、二进制 metadata、许可证、CI 与本地 release gate；未修改产品实现
- 结论：本地候选完整性和现有回归 `PASS`；公开发布就绪 `FAIL`，存在四项 release blocker

本结论不撤销最小真实 Plugin 安装闭环：当前候选确实能被本机 Codex 安装、由新任务发现并卸载。它只说明该开发候选不能原样升级为公开发布物。

## 1. 结果总览

| 范围 | 结果 | 证据摘要 |
| --- | --- | --- |
| candidate/archive 完整性 | PASS | distribution、Plugin、Starter 三个 verifier 均退出 0；外部 checksum 与实际 SHA-256 一致；两份 final candidate `diff -rq` 无输出 |
| archive 确定性与路径安全 | PASS | owner/group 固定为 0、mtime 固定为 Unix epoch、mode 固定；无 symlink/hardlink/special file/未知根内容；解包预算和 traversal 负例仍通过 |
| 运行时第三方模块 | PASS（有限） | 生产 `go.mod` 无 `require`；`go list -deps` 的非标准包只有本仓库四个 package；动态链接只见 macOS 系统 `libSystem`/`libresolv` |
| 凭据与敏感持久化 | PASS（静态） | tracked source 与展开候选未命中高置信度 token/private-key 模式；唯一宽泛命中是代码自身的敏感字段拒绝表，运行时会拒绝 token/secret/password/credential/authorization 等键和值 |
| 网络与遥测 | PASS（静态） | 分发源没有 `net/http`、下载器、遥测/分析 SDK 或上传代码；Skill 明确排除 telemetry/publication；动态网络 trace 未执行 |
| 进程执行边界 | PASS（当前契约） | 无 `sh -c`/`eval`；doctor 只执行所选 Godot 的 `--version`，Headless/test/export 通过 sibling runner、fd identity、nonce、固定 stage 与固定 argv；test 只允许固定项目脚本 |
| 写入边界 | PASS（静态） | 产品写入限定为显式 `.gameatelier`、导出 artifact、隔离 snapshot、获授权的 Godot `user://` 和用户显式安装的 owned Git hook；`release check`/`clean --list` 为零写入 |
| CI 最小权限 | PASS（实现） | workflow 顶层仅 `contents: read`，checkout 不持久化凭据，三个外部 Action 使用完整 commit SHA；GitHub-hosted 首次运行仍 NOT RUN |
| 许可证与 NOTICE | FAIL | 项目 MIT 一致，但包含 Go 标准库的预构建二进制没有随附 Go BSD notice |
| 源码到二进制 provenance | FAIL | 六个分发二进制全部记录 `vcs.revision=98ffa65...` 与 `vcs.modified=true`，不能映射到一个干净提交 |
| macOS framework 分发 | FAIL | Universal 2 可本机执行，但严格 codesign 验证失败，`spctl` 退出 1；真实下载/quarantine/Gatekeeper 路径未通过 |
| Support Matrix | BLOCKED | 文档仍把 Windows/Linux 设为 v1 Tier 1，但二者只有交叉构建形状、原生矩阵已延期 |
| strict release check | BLOCKED | 5 PASS、1 BLOCKED、5 NOT_RUN，`release_ready=false`、退出 4 |
| 外部发布链 | NOT RUN | 无 remote/tag/release workflow/npm package；没有 OIDC attestation、package provenance、SBOM 或外部 required CI evidence |

## 2. Release blockers

### SC-001 High：分发二进制来自 dirty worktree

六个 public/private、macOS/Linux/Windows 二进制的 Go build metadata 全部显示：

- Go `1.27.0`
- `-trimpath=true`
- `vcs.revision=98ffa650448f66169dd75a8513ba42c04e087c6a`
- `vcs.modified=true`

`98ffa65..7d47c05` 实际包含生产 Go 文件和分发契约变化，所以这不是“只改了无关文档”的可忽略 dirty 标记。当前 archive 可逐字节复现，只证明同一未提交输入能产生相同结果；不能证明发布物对应一个可审查的提交或 tag。

最终候选必须从 clean、不可变、已记录 revision 构建；构建器应拒绝 dirty source，并在 manifest/attestation 中记录 source revision、精确 Go 版本与构建参数。修复后必须重建全部二进制、Plugin、Starter 配对和 distribution candidate，不能只编辑 manifest。

### SC-002 High：缺少 Go 二进制再分发 notice

本地 Go 1.27.0 `LICENSE` 明确要求：二进制再分发必须在随附文档或材料中重现版权、条件和免责声明。当前 candidate、Plugin 与仓库 `NOTICE` 完全相同，只包含 Leon0555 项目说明，并声明没有第三方版权归属；没有 `Copyright 2009 The Go Authors` 或完整 Go BSD 条款。

由于 Plugin archive 实际携带静态编译 Go 二进制，这是发布阻断。最终修复应新增可机器检查的第三方 notices（可采用 `THIRD_PARTY_NOTICES` 或等价文件），更新 provenance/NOTICE 与所有包含二进制的 manifest/allowlist；Starter 若不包含 Go 二进制，不需要虚构同一依赖关系。本记录是工程许可证审计，不是法律意见。

### SC-003 High：framework Universal 2 没有有效整体签名/Gatekeeper 证据

当前 fat Mach-O 能在本机 Apple Silicon 直接运行，但：

- `codesign --verify --strict --verbose=4` 报告 code object 未签名并出现 Code Signing subsystem internal error；
- `spctl --assess --type execute` 退出 1；
- 没有 TeamIdentifier；当前文件只有本地产物的 provenance xattr，没有远程下载产生的 quarantine 证据。

先前“不要求签名/公证”的决定只针对 Godot 游戏技术导出，不自动覆盖框架 CLI。根据 `codesign -dv` 对 slice 的 linker-signed 观察与 fat 文件严格验证失败，推断 `lipo` 合并后没有形成可验证的整体签名；仍需在修复阶段用实际构建链确认原因。

最终候选至少要先建立有效整体签名并完成隔离/quarantine/Gatekeeper 实测。是否进一步要求 Developer ID 和 notarization 是最终候选阶段的产品分发决策，本轮不替用户选择。

### SC-004 High：Tier 1 承诺与已延期证据不一致

`docs/support-matrix.md` 仍冻结 macOS Apple Silicon、Windows x64、Linux x64 三个 Tier 1，并定义“每个 release candidate 必须在该宿主完成端到端流程”。当前 Linux/Windows 只有交叉编译和文件格式证据，Skill 正确拒绝在二者执行，原生 runner/机器已按用户决定延期。

最终候选前必须二选一：补齐 Windows/Linux 原生矩阵，或形成范围决策，把 v1.0 生产承诺限定为 macOS Apple Silicon，并把另外两个平台明确降级为 artifact-only/preview 或延期。交叉构建不能满足现有 Tier 1 定义。

## 3. 其他缺口

### SC-005 Medium：发布构建/验证工具链尚未冻结

- production `go.mod` 最低版本为 Go 1.24.0，candidate 实际由 Go 1.27.0 构建，CI 使用浮动 `1.24.x`；没有 final release builder 锁定一个精确 toolchain。
- Python validator 依赖固定了版本，但 CI 的 `pip install` 没有 `--require-hashes`，只能算版本固定，不能算下载内容固定。
- 没有 release workflow、tag policy、artifact attestation、SBOM 或 npm package；因此 ADR 0004 的 Trusted Publishing/OIDC/provenance 目前只是设计。

这些配置在没有 remote/publish 时不会泄露凭据，但不能支持最终 provenance 声明。

### SC-006 Medium：本地 strict gate 仍明确不完整

对 `examples/reference-game` 的真实 strict `release check` 返回退出 4：5 PASS、1 BLOCKED、5 NOT_RUN。阻断包括：

- run store 含 6 个被保护的 corrupt 历史记录；`clean --list` 只读确认 25 committed、0 incomplete、0 orphan、6 corrupt，且没有可删除 candidate；本轮没有删除 evidence。
- `clean-source-policy`、`plugin-bundle`、`starter-package`、`license-and-provenance`、`required-ci` 仍为 NOT_RUN。

最终干净环境应从新项目/新 run store 复验，不应通过删除失败 evidence 制造通过。M3 还需要把已完成的分发验证以可验证输入接入 strict gate，而不是手工把 NOT_RUN 改成 PASS。

### SC-007 Low：公开项目信息仍不完整或过期

- 根 README 仍称真实 Codex installation/lifecycle `NOT RUN`，与 2026-08-31 的最小闭环证据不一致。
- 仓库没有 `SECURITY.md` 或等价漏洞报告渠道。
- 项目名称包含第三方商标；ADR 已要求公开发布前复核，但本轮按“无远程操作”边界未刷新官方要求。

## 4. 明确通过的安全边界

- candidate 只含固定七个顶层文件（含 manifest）；Plugin manifest 盘点 19 个文件，Starter manifest 盘点 11 个内容文件（archive 另含其 manifest）；没有根 `AGENTS.md`、开发 Skill、源码、构建器、包管理脚本或用户级配置。
- Plugin/Starter/candidate 的 LICENSE、NOTICE 和 manifest hash/size/mode 一致；外部 checksum 格式与实际 archive digest 一致。
- archive 清单使用固定 owner/group、epoch mtime 和受控 mode；路径、Unicode case-fold、traversal、symlink/hardlink、metadata bomb 和拼接 gzip 负例由测试覆盖。
- 生产 Go module 没有第三方 module 依赖；维护端 Python 依赖明确标注为非产品运行时。
- 分发内容没有具体模型 ID；能力 Profile 只使用逻辑名称。
- Plugin Skill 不搜索源码 checkout、不依赖 PATH/npm/Go，不安装 Godot、依赖或 hook，不登录、不发布、不调用任意脚本。
- CI 不持久化 checkout credential，权限只读，Action 固定 SHA；当前不存在带 `id-token: write` 的发布 job。

## 5. 本轮回归

| 验证 | 结果 |
| --- | --- |
| Go `test -count=1 ./...` | PASS；`internal/app` 61.599 秒 |
| Go vet | PASS |
| gofmt | PASS；无输出 |
| Draft 2020-12 Schema | PASS；25 schemas、30 fixtures、31 negative assertions 及持久 evidence |
| Python validators | PASS；48/48 |
| distribution/plugin/starter verifier | PASS |
| candidate 两次构建 diff | PASS；无差异 |
| 高置信度 secret scan | PASS；无命中 |
| static network/telemetry scan | PASS；无实现命中 |

## 6. 未运行与后续顺序

本轮未运行：任何远程读取/写入、GitHub-hosted CI、npm name refresh/login/publish、Trusted Publishing/OIDC、远程 artifact attestation、动态网络/文件系统 trace、真实下载 quarantine、Developer ID/notarization、Windows/Linux 原生执行、升级/失败升级/回滚、最终独立审计。

建议按以下顺序进入修复阶段：

1. 补齐 Go third-party notice 与机器化包内容检查。
2. 增加 clean-source/精确 toolchain/provenance 门禁，从当前 clean commit 重新构建候选。
3. 为 fat Mach-O 建立可验证整体签名并做本地隔离/Gatekeeper Spike；到最终候选再决定 Developer ID/notarization 要求。
4. 收紧 CI validator 下载哈希，设计但不启用 release workflow/attestation。
5. 更新 README、SECURITY 与发布文档一致性。
6. 最终候选时请求用户决定三宿主 Tier 1 或 macOS-only v1，并在获准后执行相应原生/远程门禁。
7. 最后执行升级/失败升级/回滚、干净环境、required CI 和独立只读终审。

修复必须发生在后续 Implementation 步骤；本审计自身不修改被审对象，也不构成最终独立审计通过。
