# ADR 0024：Plugin 内含 Starter 的确定性创建命令

- 状态：Accepted（实现 ADR 0023 已批准的 Plugin-only 初始化路径）
- 日期：2026-09-01
- 决策范围：公开 CLI 命令、目标目录写入、Plugin 资产定位、失败与恢复语义

## 背景

ADR 0023 已将 Starter Template 收进唯一的 Plugin 分发闭包，但仅把文件放入 Plugin 还不能形成用户入口。若 Skill 直接用 shell 递归复制，版本、allowlist、符号链接、并发覆盖、失败清理和结构化结果都会依赖提示词，违反“CLI 负责确定性副作用”的边界。

现有 `initialize` 只为已存在的 Godot 项目创建 `.gameatelier` 状态，不应暗中扩大为模板安装器或覆盖已有项目。

## 决策

1. 新增公开命令 `starter create --project <new-directory>`。
2. 命令只读取当前 CLI 所属 Plugin 根内的固定 `starter-template/`，验证 `TEMPLATE-MANIFEST.json`、Plugin 版本配对、embedded 标志、固定文件/目录 allowlist、hash、大小、mode、Godot 4.7.2 standard/GDScript 边界及无具体模型 ID。
3. 目标必须尚不存在；父目录必须已存在且可安全解析。命令不合并、不修复、不覆盖任何已有文件或目录。
4. 创建先写入父目录中的私有 staging 目录，逐文件同步，再以平台验证过的原子 no-replace rename 发布；失败时只清理该 staging 目录。无法提供原子发布的平台返回明确阻断，不降级为可见的半成品目录。
5. 创建的游戏项目包含 Starter 项目文件以及随模板提供的 `LICENSE`/`NOTICE`，不复制包级 `TEMPLATE-MANIFEST.json`，不复制 Plugin、CLI、runner、Skills、内部 `AGENTS.md`、缓存、`.gameatelier` 或历史 evidence。
6. 命令不启动 Godot、不联网、不安装依赖、不写用户级 Codex 状态，也不自动调用 `initialize`。Plugin Skill 将“创建 Starter + initialize”作为一次用户可见初始化工作流依次调用两个确定性命令。
7. stdout 继续只输出一个 command-result JSON；成功结果明确 `created=true`、`initialized=false`、模板版本、文件数和字节数。路径在结构化输出中只记录为 `provided`，不回显用户绝对路径。

## 备选方案

### Skill 直接复制目录

拒绝。提示词无法稳定承担 no-replace、来源闭包、并发和恢复契约。

### 把 Starter 编译进 Go 二进制

暂不采用。它会在 Plugin 中同时保存可审计资产和二进制内副本，增加体积与来源闭包；Plugin-only 已提供固定相对资产位置。

### 扩大 `initialize` 让它在缺少项目时自动生成

拒绝。现有 `initialize` 的安全承诺是只对已有项目创建状态；把“创建目录”和“初始化状态”混入同一命令会改变幂等与覆盖语义。

## 风险与回退

- CLI 只有在完整 Plugin 布局中才能自动定位 Starter；源码开发二进制需要测试注入或显式构建 Plugin，不能把源码 checkout 当作用户后备路径。
- Windows/Linux 原子目录发布在原生 runner 验证完成前保持 BLOCKED；不得由交叉编译推定支持。
- 如命令实现回退，Plugin 仍可保留 embedded Starter，但 Skill 必须阻断新项目创建，不能静默恢复 shell 递归复制。

## 验收

- 中文、空格和特殊字符的新目录创建 PASS，并能随后由 `initialize` 接受。
- 目标已存在、父目录缺失、资产缺失/篡改、manifest 版本或 embedded 标志错误、未知文件、symlink、mode/hash 不符均零目标写入。
- 失败不遗留 staging；成功不包含 `TEMPLATE-MANIFEST.json`、Plugin/CLI/runner、`.gameatelier` 或内部 `AGENTS.md`。
- command-result Schema、Go 单元测试、Plugin 打包测试和分发扫描全部 PASS。
