# ADR 0013：预构建 Plugin Bundle、完整性与宿主声明

- 状态：Accepted（本地 bundle/lifecycle 已实证；分发入口与 Gatekeeper 门禁由 ADR 0023 取代）
- 日期：2026-08-26
- 决策范围：Plugin 平台布局、预构建 CLI/runner 配对、包清单、压缩包完整性与支持声明

## 背景

ADR 0004 已冻结 Go 与“用户不从源码构建”的方向，但没有冻结 Plugin 内平台文件布局，也没有区分“成功交叉构建”“静态形状已验证”“在目标宿主实际运行”和“已达到支持承诺”。如果打包器只相信文件名，任意文本都可能被错误声明为 Mach-O、ELF 或 PE；如果 Universal 2 的文件放进 `darwin-arm64`，也会把 artifact 架构与实机验证范围混为一谈。

## 提议决策

### Bundle 布局

单个多宿主 bundle 使用固定布局：

```text
codex-game-atelier/
├── .codex-plugin/plugin.json
├── BUNDLE-MANIFEST.json
├── LICENSE
├── NOTICE
├── THIRD_PARTY_NOTICES
├── starter-template/...
├── skills/develop-godot-game/...
└── bin/
    ├── darwin-universal2/{codex-game-atelier,codex-game-atelier-runner}
    ├── linux-amd64/{codex-game-atelier,codex-game-atelier-runner}
    └── windows-amd64/{codex-game-atelier.exe,codex-game-atelier-runner.exe}
```

仓库只跟踪 Plugin 文字源、打包器和测试，不跟踪生成的二进制、bundle 或 archive。根 `AGENTS.md`、Go 源码、构建工具、缓存和未知文件不得进入 bundle；Plugin 源文件采用显式 allowlist。

### 二进制与版本闭合

1. 公共 CLI 与同宿主 private runner 是不可拆分的 sibling 配对。
2. 打包器必须读取实际头部并验证 Universal 2 的 `x86_64 + arm64` slices、ELF amd64 与 PE32+ amd64，拒绝文本伪装、截断、错误架构、symlink、hardlink、特殊文件、空文件和超限文件。
3. Plugin manifest 版本必须通过 Go 链接参数注入公共 CLI；Apple Silicon 打包流水线必须在静态 `build` 后显式执行 `smoke-trusted-bundle`，由包内 `--version` 精确匹配。private runner 必须以固定拒绝契约退出，证明两个入口未互换。
4. Linux/Windows 在原生 runner 延后期间只能标记为 `cross-build artifact only` 与 `native_validation=NOT_RUN`，不能标记 PASS。
5. macOS artifact 声明为 Universal 2；支持措辞固定为“生成 Universal 2，但只验证 Apple Silicon”，Intel smoke 为 NOT RUN。artifact manifest 本身不冒充运行证据；运行 PASS 写入独立 validation record。

### 完整性与压缩包

1. `BUNDLE-MANIFEST.json` 记录固定 schema、Plugin 身份、无源码构建/无遥测标志、宿主声明及每文件相对路径、字节数、SHA-256 和 Unix mode；同时记录文件数和展开总大小。ADR 0021 进一步要求 clean Git revision、精确 Go 工具链和六文件/八架构 build metadata，并要求包含 Go `THIRD_PARTY_NOTICES`。
2. manifest 不写构建时间，排序固定。`.tar.gz` 统一 `uid/gid/mtime`，相同输入应生成逐字节相同的 archive。
3. archive 旁生成外部 `.sha256`。`verify` 与 `verify-archive` 必须永不执行被验证的代码：先核对外部 checksum，再限制成员数、压缩/展开大小、路径、大小写碰撞、类型和 mode，安全解包到临时目录后只重跑静态 bundle 验证。checksum 与 archive 可能来自同一攻击者，不能作为执行授权。
4. `build`、`verify`、`archive` 与 `verify-archive` 均只做静态处理。CLI/runner 执行闭合只能由名称明确的 `smoke-trusted-bundle` 触发，并且只接受同一可信本地构建流水线产物。它必须以 OS 级文件限制约束输出、限制时间、拒绝父进程退出后遗留的同组 child，并终止整个进程组；不得作为不可信下载的验证器。
5. 内部 manifest 不能保护自己；正式 Release 仍需可信发布渠道、外部 checksum、package provenance 与发布审计。

### 安装、升级和回滚边界

本 ADR 暂不批准用户级安装器或 Marketplace 写入。真实 Codex 安装缓存中的 Skill 相对路径、执行权限、带 quarantine 下载路径、升级失败不切换 active version、卸载不触碰游戏项目和回滚到上一已验证版本，必须在本 ADR 转为 Accepted 前取得证据。

ADR 0023 已冻结 v1 Plugin-only：Apple 公证不是默认门禁，真实远程 Plugin 在干净 Apple Silicon 环境无阻断安装才是门禁。只有该路径失败后，才能通过新决策把公证作为备选方案。

## 备选方案

### 每个平台一个 Plugin

暂不采用。Marketplace/客户端是否能可靠按宿主选择同版本 artifact 尚未实证，单一多宿主 bundle 更少产生错装组合。

### 安装阶段从源码构建

拒绝。违反零构建目标，并要求普通用户具备 Go/Node/源码仓库。

### 只依赖文件名和 checksum

拒绝。checksum 只能证明字节没有变化，不能证明字节属于声明的格式、架构或入口角色。

## 风险

- 多宿主 bundle 当前展开约 26 MiB、压缩约 11 MiB，后续功能增长需观察 Marketplace 限制和下载成本。
- 本地 Python 解包不能等价证明 Codex 客户端真实安装行为或 Finder/浏览器 quarantine 传播。
- Linux/Windows 只有交叉构建形状证据；原生运行、权限、取消和残留仍为 NOT RUN。
- Linux/Windows 的 public CLI 版本注入与 sibling 配对尚未在目标宿主闭合，当前只记录 artifact shape；不得从 macOS 版本 smoke 推断。
- 2026-08-30 的 Phase 1 candidate 来自有未提交修改的研发树，不是发布 artifact；ADR 0021 已将 dirty build 改为打包阻断条件，新候选必须重新构建。

## 迁移与回退

- 若官方分发要求分平台包，保持内部 CLI/runner sibling 契约和 manifest schema，再新增平台 package index；不得静默改变宿主支持状态。
- manifest schema 或布局变化需要新 ADR/版本并保持旧 bundle 可验证。
- 安装升级只在新 artifact 完整验证后切换；失败时保留并恢复上一已验证 bundle。

## 转为 Accepted 的最低证据

- Codex 实际安装缓存中的 Plugin 加载、Skill 定位、包内 CLI/runner 调用和卸载。
- 中文、空格和特殊字符安装路径。
- 受支持远程 Plugin 来源的干净安装、quarantine/Gatekeeper 观察和包内 CLI/runner 实际调用。
- archive 重现性、checksum 失败、安全解包、升级失败与回滚测试。
- Linux/Windows native runner 延后不阻碍本 ADR 接受，但二者必须继续明确为 NOT RUN，不能升级支持声明。
