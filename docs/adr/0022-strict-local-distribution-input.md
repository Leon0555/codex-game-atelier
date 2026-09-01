# ADR 0022：Strict Release Check 的本地分发输入

- 状态：Accepted（M3 本地发布工作区门禁）
- 日期：2026-08-31
- 决策范围：`release check --mode strict` 如何消费 Plugin/embedded-Starter/license/provenance 事实
- 后续变更：ADR 0023 将 candidate 从双 archive 升级为 `1.2.0` 单 Plugin archive，并增加独立 `remote-plugin-install` 门禁；本 ADR 的只读、零执行和保守阻断语义保持不变。

## 背景

M3 clean candidate 已能由维护端 Python 工具完整验证，但公共 Go CLI 的 strict release check 仍把 `clean-source-policy`、`plugin-bundle`、`starter-package` 与 `license-and-provenance` 固定为 `NOT_RUN`。这会让发布门禁依赖人工把打包日志解释成 PASS，也无法在 CI 中用一个只读命令聚合项目 evidence 与分发 evidence。

CLI 不能启动 Python、npm、shell 或任意验证脚本，也不能因为增加分发检查而修改项目、candidate、用户级 Codex 状态或网络。candidate manifest/checksum 同源，仍不能自证发布者身份。

## 决策

1. `release check` 新增可选 `--distribution-candidate <directory>`，只允许与显式 `--mode strict` 一起使用。manual/standard 不读取该输入，也不能把它误报为 release ready。
2. 未提供 candidate 时保持现有四项 `NOT_RUN`。提供后，CLI 只读验证固定 `1.2.0` distribution manifest、顶层 allowlist、文件 hash/size/mode、外部 checksum、单 Plugin archive 的有界安全结构、包内 Starter manifest/inventory、版本闭合、项目/第三方 notices 和 clean Go build metadata。
3. CLI 不执行 archive 内代码、不解包到磁盘、不启动外部进程。Go build info 直接从有界内存读取；Universal 2 两个 slice 分别验证。
4. 完整闭包通过时，`clean-source-policy`、`plugin-bundle`、`starter-package`、`license-and-provenance` 为 PASS。任一步失败时四项均保守标记 BLOCKED；stdout 不回显用户绝对路径或不可信自由文本。
5. `remote-plugin-install` 与 `required-ci` 继续为 `NOT_RUN`，直到分别存在干净 Apple Silicon 远程 Plugin 无阻断安装证据和受信 GitHub-hosted required check 输入契约。仅有本地 candidate 不能令 `release_ready=true`。
6. 该检查证明当前本地字节符合固定闭包，不证明 Publisher 身份、远程 Plugin/Gatekeeper 行为、Windows/Linux 原生运行、升级/回滚或远程 package provenance。

## 备选方案

### CLI 调用 Python packager verifier

拒绝。它会让普通预构建 CLI 依赖 Python/源码 checkout，并把外部进程执行引入核心发布门禁。

### 只相信 distribution manifest 中的 PASS 字段

拒绝。自述字段不能证明 archive、notice 或 Go binary 与 manifest 一致。

### 把 candidate 路径写入项目 state/evidence

拒绝。release check 保持零写入；本地绝对路径也不进入持久证据或结构化 stdout。

## 风险

- Go verifier 与 Python packager 是两份实现；manifest/schema 变化必须同步测试并通过独立审计。
- 本地 candidate 目录不对恶意并发修改提供不可变快照保证；正式 CI 应在隔离、不可变 workspace 运行。
- 同源 checksum 和 manifest 不能替代可信发布渠道、tag protection 或 attestation。

## 回退

- 省略 `--distribution-candidate` 即回到四项 `NOT_RUN`，不迁移项目 state。
- candidate 验证失败不会删除、修复或改写任何输入。
- 公开参数或 manifest 语义变化必须新 ADR；旧 candidate 继续由对应旧工具验证。

## 验证

- clean `0.2.0` A/B candidate 正例，四项本地分发 gate PASS、required CI NOT_RUN。
- manifest/archive/checksum/notice/provenance/Go target/unknown path/symlink/size/metadata 负例。
- manual/standard 携带 candidate 参数的用法负例。
- 检查前后 project 与 candidate tree snapshot 相同。
