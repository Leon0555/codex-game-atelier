# M2 门禁运行时、可选 Hook 与最小 CI 验证

- 日期：2026-08-29
- 宿主：macOS Apple Silicon
- CLI：Go `0.1.0-dev`，生产基线最低 Go 版本 `1.24.x`
- Godot：`4.7.2.stable.official.ed1daf0bf` standard/GDScript
- 结论：本地实现与回归 PASS；GitHub-hosted CI 和修复后的独立终审仍 `NOT RUN`

## 1. 完成范围

- `build/export --mode manual|standard|strict` 真实消费门禁策略，显式覆盖不改写 project state。
- standard 在同一 operational deadline 内依次执行生产 Headless、固定 GDScript tests、项目快照、Godot 导出、artifact 复验/复制和 Apple Silicon target smoke。
- strict 先执行 standard 子集，再对 M3 未实现项明确 `BLOCKED`；不启动真正导出。
- `release check` 保持只读，standard 复验最新当前 revision evidence 与 Release ZIP；strict 的 source/Plugin/Starter/license/CI 项保持 `NOT_RUN`。
- hooks 命令显式管理一个轻量 `pre-commit`，不自动安装、不覆盖、不依赖 hook 执行 CLI/CI 门禁。
- 最小 CI 使用一个 macOS Apple Silicon job，只读权限、完整 action SHA、Go 1.24.x、Python/Schema/分发测试和 native CLI pair smoke。

## 2. 自动化回归

| 验证 | 结果 | 证据 |
| --- | --- | --- |
| Go 全量单测 | PASS | `go test -count=1 ./...`；app package 50.990 秒 |
| Go 竞争检测 | PASS | `go test -race -count=1 ./internal/app -run 'TestHooks|TestRelease|TestRunIntentRejectsExportModeThatConflictsWithPolicySnapshot'`；3.591 秒 |
| Go 静态/格式 | PASS | `go vet ./...` 退出 0；`gofmt -l .` 无输出 |
| Python validators | PASS | 39 项，含 CI workflow、Plugin、Starter、Profiles 和 gate policy |
| Draft 2020-12 | PASS | 24 schemas、29 fixtures、11 份 Starter evidence、13 份协作记录、31 个负例断言 |
| CLI pair | PASS | public CLI `--version` 退出 0；private runner 直接调用固定退出 125 |
| 分发模型策略 | PASS | 分发文本具体模型 ID 扫描仍通过 |

## 3. 三模式与真实 Godot

更新后的当前 CLI 在既有隔离项目上执行 standard Release：

- outer export run：`atelier-20260829t154237.594112000z-f8c4bc9b50c4`
- 结果：PASS/0，60.735 秒。
- artifact：59,655,625 bytes。
- SHA-256：`cdd935cab3191e4a9e15e89363ccb4eca93353f720ad0ba7d216107ba41d7a82`。
- 形状：unsigned、not notarized、Universal 2；Apple Silicon one-frame target smoke PASS。
- standard `release check`：PASS/0，6/6；`release_ready=false`，没有把 standard 冒充严格发布就绪。
- strict `release check`：BLOCKED/4；6 项 PASS、5 项 `NOT_RUN`，包括尚无托管结果的 required CI。

此前门禁负例继续由 Go tests 覆盖：固定 tests 失败会停止真正导出；strict 不会越过 deferred gates；manual 覆盖只改变本次 intent；artifact 篡改会阻断 release check。新增 scanner 还拒绝 command `mode` 与 intent `policy_mode` 不一致的闭包。

## 4. Hook 生命周期

真实当前 CLI 在临时 Git + Starter 项目完成：

1. `initialize` PASS。
2. `hooks plan` PASS，目录字节保持不变。
3. `hooks install` PASS，只新增固定 hook 与 manifest。
4. `hooks status` 返回 `installed`。
5. 实际执行 `pre-commit`，内部 manual `release check` PASS/0，且 `release_ready=false`。
6. `hooks uninstall` PASS，只删除两份 owned 文件。

自动化负例覆盖已有用户 hook、partial/conflict、旧 CLI 路径 `stale`、有效 `core.hooksPath`、取消、中文/空格/单引号路径与 shell metacharacter quoting。没有给本源码仓库安装 hook。

## 5. CI 边界

`.github/workflows/ci.yml` 已通过本地 YAML/contract 测试：

- 单 job `verify-macos-arm64`，固定 `macos-15`。
- workflow `contents: read`，checkout 不持久化凭据。
- checkout/setup-go/setup-python 使用完整 40 字符 commit SHA。
- 不使用 `pull_request_target`，不调用本地 Git hook，不安装 Godot，不发布 artifact/package。
- 本地等价命令全部 PASS。

[GitHub-hosted runners 官方参考](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)当前将 `macos-15` 列为 arm64 Apple Silicon 标准 runner；但项目尚无远程仓库，workflow 没有 GitHub-hosted run ID，因此 required-CI evidence 必须保持 `NOT RUN`，不能据本地 YAML 声称远程 CI PASS。

## 6. 审计状态

独立只读审计在额度中断前完成全量 Go 回归并报告：未发现 Blocker/High；提出两个 Medium：文档状态漂移，以及 command mode/intent mode 与 total-timeout 覆盖需要强化。本切片已分别修复：

- README/architecture/roadmap/acceptance 与实现状态重新对齐。
- run preflight 强制 mode/policy snapshot 绑定；total operational deadline 扩展到快照和 artifact I/O。

由于独立审计上下文随后触发用量限制，修复后的全新独立复审尚未完成，状态为 `NOT RUN`，不是 PASS。该项在提交或进入 M3 前必须补做；当前主实现者的自检不能替代它。

## 7. 剩余边界

- strict 的 clean source、Plugin/Starter lifecycle、license/provenance 与 required hosted CI 仍属 M3。
- Windows/Linux 原生 runner 仍按用户决定延后。
- CI 当前不下载 Godot；Godot 干净 runner 矩阵属于 M3 发布验证。
- 无远程创建、push、npm 登录/发布、签名、公证或系统配置修改。
