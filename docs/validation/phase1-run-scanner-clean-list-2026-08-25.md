# Phase 1 Run scanner 与只读 `clean --list` 验证

日期：2026-08-25
状态：PASS（最终独立只读审计：0 Blocker / 0 High / 0 Medium / 0 Low）

## 范围结论

生产 Go CLI 已新增 `clean --list [--project <dir>]`。它严格读取项目状态和有界 run store，完整验证已实现 `validate` run 的 intent/result/payload/evidence 闭包，只把有效 intent 无 result 的 `incomplete` 与 intent/result 都缺失的 `orphan` 列为预览候选。任何不能证明完整性的 run 都作为 `corrupt` 受保护。

本切片不删除、不恢复、不修复、不创建 `.run.lock`、不创建 evidence、不更新项目索引。活动 run 在 root→intent 窗口可能显示为 orphan，在 intent→result 窗口可能显示为 incomplete。未来删除必须单独获得用户确认；并且在接入前先让 writer/cleaner/recovery 实现同一 per-run 协调协议，再在锁内重新分类。

## 已通过验证

| 项目 | 结论 | 证据边界 |
| --- | --- | --- |
| 四态分类 | PASS | 单测构造 committed、incomplete、orphan、hash 被篡改的 corrupt；只有 incomplete/orphan 进入 candidates |
| Operation/commit 分离 | PASS | 已提交的 `FAIL` validation evidence 仍分类为 committed，不因命令失败进入清理候选 |
| 历史 revision | PASS | intent revision 0 在当前状态 revision 1 下仍验证为 committed；future revision 被拒绝 |
| 零写入 | PASS | 中文/空格项目路径下对 `.gameatelier` 做递归内容/模式快照；`clean --list` 前后完全相同，evidence 为空 |
| 空 run store | PASS | `runs` 不存在时返回 scanned=true、四类计数 0，不创建目录 |
| Unsafe fail-closed | PASS | 非法目录名、run 目录 symlink、超过 512 个目录均返回 `RUN_SCAN_UNSAFE`/7；scanned=false，计数/列表清零，不泄露部分候选 |
| 取消 | PASS | public dispatcher 贯穿调用方 context；pre-cancel、mid-scan cancel 与 64 KiB chunk 间 cancel 均返回/传播 `COMMAND_CANCELLED`/6 或 context error，不返回部分候选 |
| 累计工作预算 | PASS | 512 目录、2,048 文件、256 MiB 精确总读取上限；文件读取前用已打开 regular-file size 检查剩余预算，exact-limit 通过、limit-minus-one 在内容读取前失败；payload/evidence 不再重复读取 |
| JSON/文件安全 | PASS | duplicate key、尾随值、无效 JSON、intent symlink、bounded regular-file 读取均有回归覆盖 |
| Schema | PASS | Draft 2020-12：15 schemas、16 fixtures、8 negative assertions；新增 clean PASS/FAIL/cancelled data 和 candidate state/reason 绑定 |
| Go 单测 | PASS | Go 1.27.0：Low 修复后 `go test -count=1 ./...`，app package 26.720 秒 |
| Race | PASS | Low 修复后 `go test -count=1 -race ./...`，app package 30.693 秒 |
| 静态检查 | PASS | `go vet ./...`、`gofmt -d`、`git diff --check` |
| 三目标构建形状 | PASS | public CLI、private runner 和 app tests 均交叉编译为 Darwin arm64、Linux amd64、Windows amd64；不等于 Linux/Windows 原生运行 |
| Universal 2 | PASS | public CLI 与 runner 均生成 `x86_64 arm64` Mach-O；只在 Apple Silicon 原生执行 public Universal 2 的 `clean --list` |

## 真实参考项目复验

以临时构建的当前 public CLI 运行：

```text
codex-game-atelier clean --list --project examples/reference-game
```

结果通过 command-result schema，退出 0、outcome `PASS`、evidence 为空：

- committed：13
- incomplete：0
- orphan：0
- corrupt/protected：1
- cleanup candidates：0

扫描前后 `.gameatelier` 内所有 regular files 的 SHA-256 清单完全一致。

唯一 protected 历史 run 为 `atelier-20260825t025946.198285000z-e8bb17805e4b`，原因 `INTENT_INVALID`。只读核验表明该早期 Headless run 在 ADR 0009 外部写入声明落地前生成：command 已授权标准 `user://`，但 intent 缺少 `declared_external_writes=["godot:user-data:standard-os-location"]`。因此当前 scanner 按安全契约保护为 corrupt，不删除、不改写，也不把它作为候选。这是保留的早期开发证据，不影响其余 13 个当前闭包验证通过。

## 尚未覆盖

- 实际删除、`.run.lock`、锁内重验、恢复 finalize、索引和 schema migration 未实现。
- scanner 当前只接受已实现的 `validate` 闭包。未来 `test/build/export/release check` 接线前必须新增各自严格预检。
- 512 目录、2,048 文件或 256 MiB 内容以上当前 fail-closed；分页/派生索引尚未设计。
- Linux x64、Windows x64 只完成交叉编译形状，按用户决定未使用原生 runner/机器执行。
- macOS Intel 未实机 smoke；Universal 2 只在 Apple Silicon 验证，符合已批准支持声明。
- 本切片不构成 v1.0 生产级或发布声明。

## 首次独立审计与修复

首次只读终审结论为 0 Blocker / 1 High / 1 Medium / 0 Low，未接受当时的 staged index：

- High：scanner 丢弃 public signal context，只有单文件/目录数上限而没有累计 I/O 预算，最坏可读取数 GiB且不能响应正常 SIGINT/SIGTERM。
- Medium：ADR 只警告活动 run 显示为 incomplete，并假定未来 cleaner 单方面获取 `.run.lock` 即可；实际 writer 在 intent 前也会显示为 orphan，且当前 writer 不持有同一锁。

修复后 context 已贯穿并按 64 KiB 分块读取，增加全扫描字节/文件预算且取消/越界不返回部分决策；文档明确两种活动窗口和共同 writer/cleaner/recovery 协调协议前置条件。普通/race/vet/schema、三目标双二进制与测试包交叉编译、Universal 2 生成、Apple Silicon 真实参考项目零写入扫描均已重新通过。

第二次只读审计确认首次 High/Medium 已关闭，分级为 0 Blocker / 0 High / 0 Medium / 1 Low。Low 指出当剩余预算不足一个 64 KiB chunk 时，旧实现会先读 chunk 再拒绝，实际 I/O 可能超预算最多 64 KiB。修复后 scanner 在读取前以已打开文件的 size 同时检查单文件与全局剩余预算，只读取该 size 的精确字节数，并在读取后再次 `fstat` 检测缩短/增长；新增 exact-limit 与 limit-minus-one 回归。

最终定向复审确认 Low 已关闭且未引入新问题，分级为 0 Blocker / 0 High / 0 Medium / 0 Low。审计确认预算不足在任何内容读取前失败且 `readBytes/readFiles` 均为 0，exact-limit、short read/EOF、零进展、空文件、文件变化、context cancel 和整数减法边界均被实现或测试覆盖。
