# ADR 0004：CLI 运行时与零构建分发

- 状态：Accepted（分发方向与 Go 生产实现语言均已批准）
- 日期：2026-08-24；更新：2026-08-25
- 决策范围：CLI 运行时契约、Plugin/Template/npm/Release 分发与发布安全

## 背景

普通用户不应 clone 源码、执行 `npm build`、使用前沿 Node 或先学习大量命令。与此同时，CI 与高级用户需要稳定的确定性 CLI。单纯发布 TypeScript 源码或在安装阶段编译，会扩大环境差异和供应链风险。

## 决策

### 用户入口

1. Codex Plugin 与 Starter Template 是普通用户主要入口。
2. CLI 是二者的确定性底座，也是 CI/高级用户入口；普通用户不需要先学习 CLI 命令表。
3. 在已安装受支持 Godot 的前提下，目标首次使用不超过三个主要步骤。

### 分发契约

1. 用户取得的是**预构建或已打包**产物，永不要求从仓库源码构建。
2. 优先验证自包含原生 CLI：macOS arm64、Windows x64、Linux x64 各有确定版本、checksum 和 artifact manifest。
3. **v1 CLI 的生产实现语言冻结为 Go。** Phase 1 的 Rust/Go 对照显示，两者本机启动速度无决定性差异；Go 样品使用零第三方依赖，维护者工具链更小，并可在不增加 SDK 的情况下生成 Tier 1 x64 交叉产物。Rust 的 stripped binary 明显更小，但不足以抵消 v1 构建、依赖和跨平台维护成本。用户于 2026-08-25 明确批准 Go；详见 `docs/spikes/phase1-runtime-results.md`。
4. 若原生方案不满足 Plugin/维护成本，备选为发布已 bundle 的 JavaScript `dist`，支持保守的 LTS Node 下限；用户仍不执行 build。不得把最新 Node 当唯一要求。
5. npm 包是便利入口，不是普通用户唯一入口：可以是包含 JS launcher 和平台二进制 packages 的 meta package；安装脚本不得隐式编译或下载 Godot。
6. Plugin 内如何携带/定位平台 CLI 必须通过官方 Plugin 打包规则和当前客户端实测后再冻结，不能在 Phase 0 假设。

这里的 “Spike” 不是正式产品实现：Rust 与 Go 样品只用于比较二进制体积、启动速度、跨平台构建和进程控制。Go 的生产实现必须在正式包路径重新实现或正式审查，不直接复制未完成的 Spike；Rust 样品保留为研究证据，不进入生产 CLI、Plugin、Starter Template 或发布产物。Plugin 打包、升级/回滚和 evidence 写入仍需继续验证。

### 候选命名（2026-08-24 快照）

| 类型 | 候选 | 公开观察 | 建议 |
| --- | --- | --- | --- |
| npm unscoped | `codex-game-atelier` | 2026-08-24 registry 返回 E404 | 用户确认的首选；E404 不是保留，发布前复查 |
| npm scoped | `@codex-game-atelier/cli` | 取决于未来是否实际控制同名 scope | 仅在需要多包且取得 scope 后使用 |
| npm | 旧通用候选名 | registry 返回 404 | 不推荐，过泛且 GitHub 已有相近组织名称 |
| npm | 旧 Godot 专属候选名 | registry 返回 404 | 不作为总项目名 |
| GitHub repo | `Leon0555/codex-game-atelier` | 2026-08-24 GitHub Connector 搜索未发现同名仓库 | 用户确认的首选 slug；尚未创建或保留 |
| GitHub owner | 相近旧概念组织名 | 已存在的组织 | 不使用、不暗示关联 |

推荐远程仓库为 `Leon0555/codex-game-atelier`，CLI 保持 monorepo `packages/cli`。`codex-game-atelier` 同时作为首选 npm 包名和 GitHub repository slug；产品名称已确定为 Codex Game Atelier。现有本地目录只是 Phase 0 创建的工作区路径，不构成产品标识，也不要求在本次文档与实现变更中移动。名称中的 “Codex” 在公开发布前仍需完成品牌/商标审查。

### npm 与 GitHub 发布

1. 公开 npm 包优先采用 GitHub Actions + npm Trusted Publishing (OIDC)，避免长期发布 Token。
2. 发布 job 只授予 `contents: read` 和 `id-token: write`，由受保护 `v*` tag/environment 与审批触发；第三方 Actions 固定完整 commit SHA。
3. npm 账号启用 2FA；OIDC 稳定后设置 package 为要求 2FA 且不允许传统 token。紧急人工路径使用 npm 支持的 2FA 交互发布/`npm stage publish` 后审批；若必须改变 token policy，先作为独立安全决策审计并获用户批准，不能预设 token 后门。
4. 对公开 GitHub 仓库的公开 npm 包生成 package provenance；同时发布 checksum、artifact manifest、许可证和来源记录。Provenance 证明来源/构建链，不证明代码安全。
5. 外部 Release、npm publish、Marketplace 提交均为单独用户授权动作。Phase 0/实现阶段不登录、不创建 token、不发布。

## 备选方案

### A. TypeScript 源码 + 用户本地 build

拒绝。违反零构建目标并把工具链差异转嫁给用户。

### B. 只发布 npm 包

拒绝。会让普通用户依赖 Node/npm，也不匹配 Plugin/Starter Template 优先入口。

### C. 安装脚本下载最新二进制/引擎

拒绝。版本不确定、隐藏网络副作用和供应链风险高；任何下载必须显式、固定版本并校验。

### D. 只发布单一平台二进制

拒绝 v1.0 推荐矩阵。会与三宿主生产承诺冲突。

## 理由

自包含预构建 CLI 最接近“普通用户无 Node、无 build”，同时让 CI 固定版本。Plugin/Template 负责可发现性和工作流，CLI 保持小而确定。OIDC 和 provenance 减少长期凭据风险并提高来源可审计性。

## 风险

- Go 的可执行文件大于 Rust 对照样品；必须继续验证 Plugin 包体、下载与升级成本。
- Unix 进程组实现不能代表 Windows Job Object；Windows 进程树、取消和残留必须在目标宿主专项验证。
- 原生跨平台构建、代码签名和依赖更新成本高于单一 JS bundle。
- npm 平台包会增加发布原子性要求，任一缺包可能导致安装失败。
- Plugin 是否允许/适合携带各平台二进制仍需官方规则和实测。
- 包名/GitHub owner 的 404 会随时变化，也可能受私有资源行为影响。
- 项目名中的第三方商标需要发布前审查。

## 迁移与回退

- CLI 协议、状态 schema 和 artifact 名称保持实现语言无关，但 v1 生产代码使用 Go。未来改用 Rust、JS 或其他语言属于架构迁移，必须新 ADR、兼容性证据和用户批准。
- npm launcher 固定兼容的 CLI protocol；平台包缺失时给出明确手动下载指引，不隐式编译。
- 每个发布保留前一兼容 artifact 和 checksum，升级失败可回滚。
- 从 unscoped 迁移到 scoped 时提供 deprecation stub/迁移文档，不静默换包。

## Phase 1 Spike 验收

- 三宿主 hello/detect/doctor 二进制的体积、冷启动、JSON、进程取消和路径处理。
- Plugin/Starter Template 能否无源码构建地调用固定 CLI。
- npm meta package 在无编译器环境安装，且无隐藏下载/脚本副作用。
- 升级、回滚、checksum 失败和不支持平台诊断。
- 生成 SBOM/manifest/provenance 的可行性。

## 官方依据

- [npm：Unscoped public packages](https://docs.npmjs.com/creating-and-publishing-unscoped-public-packages/)
- [npm：Scoped public packages](https://docs.npmjs.com/creating-and-publishing-scoped-public-packages/)
- [npm：Trusted Publishing](https://docs.npmjs.com/trusted-publishers/)
- [npm：Generating provenance statements](https://docs.npmjs.com/generating-provenance-statements/)
- [npm：Two-factor authentication](https://docs.npmjs.com/about-two-factor-authentication/)
- [GitHub：Use GITHUB_TOKEN for authentication](https://docs.github.com/en/actions/tutorials/authenticate-with-github_token)
- [GitHub：Secure use reference](https://docs.github.com/en/actions/reference/security/secure-use)
- [OpenAI 官方：Build plugins](https://learn.chatgpt.com/docs/build-plugins)
