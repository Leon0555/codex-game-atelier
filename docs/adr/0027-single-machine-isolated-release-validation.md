# ADR 0027：v1 接受单机隔离式最终复验

- 状态：Accepted（用户于 2026-09-08 选择方案 B）
- 日期：2026-09-08
- 决策范围：V1-09 的干净环境与 macOS 生产证据
- 细化：ADR 0025 的 macOS Apple Silicon 发布门禁

## 背景

rc.6 已在当前 Apple Silicon Mac 上完成固定源码的可复现双构建、全新隔离 `CODEX_HOME` 的 GitHub 远程 Plugin 安装、安装 cache 与候选逐文件比较、真实用户级升级/失败保护/回滚/卸载、全新特殊路径项目、Godot 4.7.2 Headless、固定 GDScript tests、Debug/Release 技术导出、Apple Silicon target smoke、required CI 和独立只读终审。

原 V1-09 还要求在独立 macOS 用户或第二台 Apple Silicon 实机重复完整流程。当前机器只有一个普通本地账户；创建一次性账户会引入系统级账号、密码、目录所有权和清理操作。用户明确决定不创建该账户，并选择接受当前机器的隔离式证据。

## 决策

1. v1.0 不再要求第二 macOS UID 或第二台实体 Apple Silicon Mac 才能关闭 V1-09。
2. V1-09 的“干净环境”定义为以下组合全部成立：
   - 候选来自固定、干净的源码 revision，并完成两次可复现构建；
   - 使用未复制现有配置或凭据的全新隔离 `CODEX_HOME`，从固定 GitHub Marketplace ref 远程取得并安装 Plugin；
   - 安装 cache 与已验证候选 Plugin 树逐文件一致；
   - 使用远程安装 cache 中的 CLI，在此前不存在的中文、空格和特殊字符项目路径创建 Starter 并初始化；
   - 在支持宿主实际运行 Godot 4.7.2 Headless、固定 GDScript tests、Debug build、Release export 与 Apple Silicon target smoke；
   - 另行完成真实用户级安装、升级、损坏升级拒绝、回滚、重装、卸载和起止状态精确恢复；
   - 没有 Gatekeeper/System Settings 绕过、quarantine 移除、凭据复制、自动依赖安装或隐藏外部写入；
   - required CI、绑定 strict evidence 和独立只读终审通过。
3. GitHub 托管 `macos-15` Apple Silicon CI 继续提供独立机器上的源码构建、Go、Schema、分发契约和原生 CLI pair smoke；它不冒充完整 Godot/Plugin 生命周期重复。
4. rc.6 已持久化的隔离安装、真实生命周期和 Godot E2E 组合满足上述定义，因此 V1-09 可改为 PASS。
5. 对外只声明“在 macOS Apple Silicon 上完成技术验证”。不得宣传多用户、多机器、macOS Intel、签名/公证或普通消费者直接下载兼容性已经验证。
6. 独立账户或第二台 Apple Silicon Mac 的完整复验降为后续增强项，不再阻断 v1.0；若出现权限、TCC、文件系统或机器相关缺陷，必须恢复为发布 blocker 并重新评估本决策。

## 备选方案

### 同一台 Mac 创建一次性标准账户

不采用。它能额外验证不同 UID、Home 目录所有权和空用户配置，但需要系统级账户操作，用户明确拒绝。

### 第二台 Apple Silicon Mac 完整复验

本次不采用。它能覆盖更多机器差异，但准备与维护成本更高；现有托管 CI 已提供第二机器上的构建层验证，完整生命周期重复保留为后续增强。

### 只保留当前账户的普通重复测试

不采用。没有隔离 `CODEX_HOME`、远程固定 ref、逐文件候选绑定和状态恢复的重复运行不能称为干净环境。

## 风险与缓解

- 单一实体机器可能隐藏 TCC、文件系统、系统版本或本机缓存差异。缓解：隔离 Home 状态、固定远程来源、逐文件绑定、全新项目路径、项目/用户两层恢复证据，以及独立托管 Apple Silicon CI。
- 当前真实用户级生命周期与远程取得分开执行。缓解：两者的 Plugin 树均与同一 bundle A 逐文件一致，文档必须继续披露这一拆分。
- 本决策降低的是环境重复次数，不降低候选完整性、Godot 运行、目标 smoke、CI、审计或发布授权要求。

## 回退

若后续远程用户报告只在新账户/新机器出现的阻断，或审计认为当前隔离不能覆盖关键权限边界，则把 V1-09 恢复为 `NOT RUN/BLOCKED`，并新增独立 UID 或第二机器复验。该回退不改变已有 rc.6 历史证据。

## 验收

- Support Matrix、项目简报、路线图、README 与 V1-09 使用同一“单机隔离式干净环境”定义。
- 历史 rc.2–rc.5 文档保持当时结论，不回写为 PASS。
- rc.6 绑定 evidence、strict 12/12 和原最终审计仍保持原始事实；本次范围变更另行接受独立只读政策审计。
- V1-12、受保护版本 ref/tag 和正式发布继续保持未执行，不能由本 ADR 自动批准。
