# ADR 0026：绑定式外部发布证据

- 状态：Accepted
- 日期：2026-09-01
- 决策范围：strict `release check` 如何消费远程 Plugin 与 required CI 事实

## 背景

`release check --mode strict --distribution-candidate` 已能只读验证本地单 Plugin candidate，但 `remote-plugin-install` 与 `required-ci` 必须保持 `NOT_RUN`。M3 已取得真实 Git-backed Plugin 安装、Godot E2E、用户级生命周期和 GitHub-hosted CI 证据；若仍依赖人工阅读文档，strict 命令永远不能形成机器可读闭包。

外部证据不能通过让公共 CLI 隐式联网、读取长期 Token 或调用 GitHub/Plugin 安装器解决。candidate 自述、人工 JSON 和同源 checksum 也不能单独证明发布者身份。

## 决策

1. 公开命令新增 `--release-evidence <file>`，只允许与显式 `--mode strict` 和 `--distribution-candidate <directory>` 同时使用。命令结果只记录 `release_evidence: provided`，不回显路径或外部自由文本。
2. release evidence 使用固定 `1.0.0` JSON Schema，记录 release 版本、源码 revision、candidate manifest/archive SHA-256、远程 Marketplace 安装观察和 GitHub Actions required CI 观察。
3. CLI 有界只读解析证据，不联网、不启动外部进程、不写项目/candidate/evidence。它要求：
   - release version、source revision、distribution manifest 摘要和 Plugin archive 摘要与已验证 candidate 一致；
   - 远程安装与 CI repository 一致，远程安装版本与 candidate 一致；
   - CI head SHA 与 candidate source revision 一致，固定 workflow/job、push/main、成功结论和 required branch protection 均明确记录；
   - macOS arm64、Godot 4.7.2 standard/GDScript、无 Gatekeeper/system-settings/quarantine 绕过、CLI/runner/Starter/Headless/6-test 和远程安装态新任务 Skill 发现均为 PASS；
   - 最终候选自身完成从上一候选升级、失败升级不切换 active、回滚上一候选、卸载和用户状态恢复。
4. 未提供 evidence 时两项仍为 `NOT_RUN`；提供但任一字段、摘要或绑定失败时两项均 `BLOCKED`；完整通过时两项为 PASS，strict 才可能返回 `release_ready: true`。
5. GitHub `main` 必须另行配置 hosted macOS Apple Silicon CI 为 required check。CLI 验证的是证据闭包与 candidate 的一致性；branch protection 才负责远程合并不可绕过。
6. v1 不要求 OIDC、签名或密码学 attestation。正式发布前仍需固定/保护版本 ref 或 tag、只读终审和用户批准；该输入不自证发布者身份。

## 备选方案

### CLI 直接调用 GitHub API 与 Codex Plugin 安装器

拒绝。它会把网络、账号、Token、外部写入和客户端版本行为引入核心只读门禁。

### 两个独立参数分别接受 CI 与 Plugin 证据

拒绝。它增加命令面和错配组合；单一 manifest 更容易绑定同一 version、revision、candidate 和 repository。

### 只依赖 branch protection，不接入 strict

拒绝。远程强制仍需要，但这样公共发布聚合器无法机器化表达已经完成的外部门禁。

### v1 强制 GitHub artifact attestation

延期。它能增强发布者身份保证，但不是当前 Plugin-only 无阻断安装闭环的必要条件；引入时必须新 ADR，不得静默改变 v1 安装路径。

## 风险与边界

- 本地 evidence 文件可以被伪造；CLI 只证明固定结构、摘要与跨输入绑定，不证明 GitHub 身份。required branch protection、受保护 release ref、远程只读复核和最终审计共同承担信任边界。
- `recorded_at` 只验证 RFC 3339 语法，不设 TTL；稳定的 version/revision/hash 不会因时间自动失效，但离线 CLI 也无法发现 branch protection 后来被撤销。每次正式发布仍必须现场只读复核远程保护状态。
- repository 允许合法 GitHub fork，不在分发代码中硬编码 `Leon0555`；官方发布身份由发布配置、受保护 ref 和最终审计确认。`marketplace_revision` 是固定格式的远程观察值，离线 CLI 不会假装能解析 Git ref 或向 GitHub 反查它。
- candidate 与 evidence 是显式本地路径；验证期间的恶意并发替换不在 v1 文件系统威胁模型内。正式检查应在隔离、不可变工作区执行。
- 该证据只覆盖 macOS Apple Silicon。Windows/Linux 交叉构建不得因此升级为原生支持。

## 回退

- 省略 `--release-evidence` 即恢复两项 `NOT_RUN`，不迁移或改写项目 state。
- evidence 验证失败只返回保守阻断，不删除、修复或覆盖任何输入。
- 若未来需要密码学身份或其他 CI provider，新增 schema/ADR 版本；`1.0.0` 不宽松接受未知字段。

## 验证

- 完整绑定正例使 strict 的 12 项门禁全部 PASS、`release_ready=true`；缺少新任务 Skill 发现或最终候选生命周期任一项均阻断。
- version、source revision、manifest/archive SHA、repository、Marketplace ref、CI URL/head/job、required 状态、macOS/Godot/测试结论的逐项篡改负例。
- 缺失、未知字段、超大文件、symlink、非 strict/无 candidate 参数组合和取消负例。
- 检查前后 project、candidate 和 evidence tree 完全一致；stdout 不含任一输入绝对路径。
