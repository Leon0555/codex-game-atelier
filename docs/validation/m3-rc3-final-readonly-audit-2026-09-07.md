# M3 rc.3 最终只读审计

- 日期：2026-09-07（Asia/Shanghai）
- 候选：`0.3.0-rc.3`
- 源码提交：`89a80a88b47088373176afb3d1fda274ac875144`
- 审计方式：独立 Sol/xhigh 只读上下文；审计代理未修改文件
- 结论：**FAIL；拒绝 rc.3，V1-11 不能 PASS**
- 发现计数：Blocker 0 / High 1 / Medium 1 / Low 1

## High：非支持宿主仍能执行部分公开命令

目标提交的顶层分发器在宿主门禁前分发全部 13 个公开命令：

- `packages/cli/internal/app/app.go:20` 没有统一的 pre-dispatch host gate。
- `release.go` 没有宿主门禁，并能直接把 `support-scope` 记为 PASS，因此 Windows/Linux artifact 理论上可返回 `release_ready=true`。
- `hooks.go` 没有宿主门禁，`install`/`uninstall` 在非支持宿主可到达 `.git/hooks` 写入或删除路径。
- 目标提交的测试只验证 `isSupportedHost` 布尔矩阵，没有逐命令的拒绝与无写入测试。

这违反 ADR 0025 的 `artifact-only / unsupported in v1` 契约，也与 Skill 和 V1-10 的运行时声明冲突。rc.3 已绑定的二进制不能被后续源码修改挽救，必须拒绝并构建新候选。

## Medium：Codex Skill 路径元数据陈旧

新任务能看见 `codex-game-atelier:develop-godot-game`，但能力清单路径仍指向已不存在的 rc.2 目录；实际 rc.3 Skill 存在且候选/远程树 hash 正确。独立审计复现了标准字面路径加载失败。

这是 Codex 客户端元数据问题，不单独构成候选完整性 Blocker/High。最终 RC 的 V1-09 必须在新鲜客户端/用户环境复验标准解析路径，不能只用手工查找 rc.3 文件代替。

## Low：缺少私密漏洞报告入口

仓库没有根级 `SECURITY.md`。项目会写项目状态，并可由用户显式安装 Git hook；正式公开发布前应提供支持范围和不会泄露漏洞细节的报告路径。该项不阻断当前源码修复，但应在新候选前补齐文档并确认可用的私密入口。

## 已独立确认

- Candidate、Plugin bundle、archive 的静态验证与摘要绑定 PASS。
- Schema/fixtures、52 项 Python validators、目标提交 Go test/vet/gofmt、strict 12/12 均 PASS。
- Marketplace revision `40d8e90a297f43ab245c93cce4a894145869ca95` 的 Plugin 树与候选逐文件一致。
- GitHub `main`、required CI run `33965337817` 和 branch protection 与绑定 evidence 一致。
- MIT、NOTICE、Go 标准库第三方声明、build revision/clean/trimpath/CGO metadata 未发现缺陷。
- catalog EOF 已披露；本地配置 mode/size/hash、Atelier 条目和目录恢复证据足以支持本次目标生命周期恢复，不单独阻断。
- V1-09、V1-12 保持 `NOT RUN`。

## 后续处置

High 发现触发新候选，不回写 rc.3 artifact：

1. 在所有公开命令分发前增加统一 host gate，非 `darwin/arm64` 在解析项目或产生写入前返回 `HOST_UNSUPPORTED`。
2. 增加模拟 Linux/Windows 的逐命令负向测试，并证明文件树无变化。
3. 全量回归、受保护 PR 与 required CI 通过后，从新的 clean `main` 构建 rc.4。
4. rc.4 必须重新执行 candidate 绑定、远程 Plugin、Godot E2E、生命周期、strict 与独立只读终审；不得沿用 rc.3 PASS 声明。
