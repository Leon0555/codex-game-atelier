# ADR 0021：干净构建 Provenance 与第三方 Notice

- 状态：Accepted（M3 本地候选发布前门禁）
- 日期：2026-08-31
- 决策范围：预构建 CLI/runner 的源码归属、Go 工具链身份与二进制许可证材料

## 背景

M3 只读审计确认，`0.2.0` 旧候选的六个 Go 二进制均记录 `vcs.modified=true`，不能精确对应一个干净提交。候选还只携带项目 MIT，没有满足本地 Go `LICENSE` 对二进制再分发 notice 的明确要求。逐字节可复现不能替代源码 provenance 或第三方许可证义务。

## 决策

1. Plugin 打包器在复制二进制前必须确认仓库工作树 clean，并取得完整 40 位 Git revision；dirty、无 Git、revision 不一致均阻断。
2. 打包器必须使用调用方显式传入的 Go tool，记录其精确 `goX.Y.Z` 版本，并用 `go version -m` 检查所有 public/private、macOS 两个 slice、Linux 和 Windows 构建记录。
3. 每个记录必须使用同一 revision、`vcs.modified=false`、`-trimpath=true`、`CGO_ENABLED=0`，并匹配固定 module/package、GOOS 和 GOARCH。Plugin manifest 记录 source revision、Go 版本、6 个二进制文件和 8 个架构记录，不记录时间或绝对路径。
4. Distribution manifest 必须逐字复制 Plugin 的 verified-clean provenance；没有该字段的旧 bundle/candidate 自动验证失败。
5. 根 `THIRD_PARTY_NOTICES` 保存完整 Go copyright、条件和免责声明。Plugin 与总分发 candidate 必须携带并逐字节核对；Starter Template 不含 Go 二进制，不复制该文件。
6. 该 provenance 证明本地构建输入闭合，不证明发布者身份。最终外部发布仍需要受保护 tag、required CI 和可信渠道/attestation。

## 备选方案

### 只在文档写“从 clean commit 构建”

拒绝。旧候选已经证明人工说明和实际 build metadata 可以不一致。

### 删除 Go VCS metadata

拒绝。`-buildvcs=false` 会隐藏 dirty 输入而不是解决 provenance；最终候选保留并验证 metadata。

### 把 Go notice 只放仓库网页

拒绝。离线 Plugin/archive 必须自带适用的二进制再分发材料。

### 给 Starter 也复制 Go notice

拒绝。Starter 是源码型 Godot 项目且不嵌入 CLI；无差别复制会虚构其内容关系。

## 风险

- `go version -m` 是维护端 Go tool 的行为；最终 release toolchain 变化时需重验解析契约。
- Universal 2 必须分别检查两个 slice；只检查 fat 文件返回的第一条记录可能漏掉架构不一致。
- clean local provenance 仍不等于签名、公证、Gatekeeper 或远程 package provenance。

## 回退

- 旧 manifest 不迁移、不补写；按旧候选处理并保持非发布资格。
- 若 provenance 收集失败，打包器在创建输出前阻断，不留下半成品。
- 若第三方 notice 缺失或篡改，Plugin/distribution verifier 必须失败。

## 验证

- 六个文件、八个架构记录的 clean revision/Go/toolchain/target 正例。
- dirty Git、`vcs.modified=true`、revision/target/toolchain 不一致负例。
- Plugin 和 distribution 缺少/篡改 `THIRD_PARTY_NOTICES` 负例。
- 从同一 clean commit 独立构建两次并逐字节比较二进制、Plugin archive 与 distribution candidate。
