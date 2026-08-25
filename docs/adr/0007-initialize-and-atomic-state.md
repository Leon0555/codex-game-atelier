# ADR 0007：初始化与原子项目状态

- 状态：Accepted（Phase 1 生产基线，尚非 v1 跨宿主稳定承诺）
- 日期：2026-08-25
- 决策范围：`initialize`、项目身份、独占锁、不覆盖发布、幂等与恢复边界

## 背景

首个生产 CLI 切片只能读取 `.gameatelier/project.json`。Phase 1 下一步需要建立真实状态，但不能因崩溃、并发初始化、symlink 或普通 rename 的覆盖语义产生半文件或破坏已有状态。同时，ADR 0005 的多文件 evidence 提交顺序尚未经过恢复验证，不能把它暗中塞入首次初始化。

## 决策

1. 公共命令为 `codex-game-atelier initialize [--project <dir>]`。它只初始化已有 Godot/GDScript 项目的 Atelier 状态，不创建或修改 `project.godot`，不要求 Godot executable，不运行引擎，也不写项目外。
2. 前置检查全部通过前零写入：目标必须存在 `project.godot`、不属于 Godot .NET/C#，并运行在已完成本命令原生事务验证的宿主上。当前只启用 macOS Apple Silicon；Linux x64 与 Windows x64 虽属于 v1.0 Tier 1 目标，但在各自原生文件系统/锁测试通过前显式返回 `INITIALIZE_HOST_NOT_VERIFIED`。
3. 首次状态固定为：schema `1.0.0`、revision `0`、mode `standard`、Godot `4.7.2-stable`/GDScript、空 task/run refs、省略 `last_command_result_ref`、UTC canonical 时间戳。
4. `project_id` 使用 CSPRNG 生成 `project-` 加 128-bit 小写 hex，不由路径、目录名、时间、机器或账号派生。复制已初始化目录会保留身份；创建新身份留给未来显式 clone/reinitialize 决策，Starter Template 不携带生成后的 state。
5. 合法既有 v1 state 返回 `PASS`/0、`created:false`，不得改字节、mtime、revision 或时间。无效、未知 schema、symlink、特殊文件或不安全状态返回 `FAIL`/7，不覆盖、修复、迁移或 force。
6. 写入方使用固定 `.project-state.lock` 的 OS advisory exclusive lock；锁由进程退出自动释放，锁文件本身可以保留，不用 PID/时间猜测 stale lock，也不自动删除。
7. `project.json` 通过同目录随机临时文件写全、file sync、close，再用同目录 hard-link/no-replace 原语发布；已存在目标绝不覆盖。发布后删除本次精确 temp，并在支持的平台同步 state 目录。
8. 若文件系统不支持 hard link/no-replace，返回 `STATE_ATOMIC_WRITE_UNSUPPORTED`，不降级为覆盖式 rename。若 final 已完整可见但目录同步/清理无法确认，返回 `STATE_DURABILITY_UNCONFIRMED`；重跑通过固定 state root 读取并重新 sync 合法 state 后幂等通过，不回滚 final。合法 final 的 fast path 不创建或获取 lock，避免把只读幂等确认变成持久写入。
9. 本切片成功结果的 `evidence` 仍为空，不创建 runs/evidence/index，也不更新 `last_command_result_ref`。多文件 evidence 事务作为独立薄切片按 `artifact -> evidence record -> run result -> optional state index` 验证。

## 原子承诺分层

- 当前本机已验证目标：协作 CLI 观察不到半个 `project.json`；并发 initialize 不覆盖既有 final；合法重跑不修改 state。
- macOS 本机可以验证 file sync、hard-link publish 和 directory sync 的运行行为。
- Windows/Linux 在原生宿主执行锁、文件系统和故障注入前，只能声明交叉编译形状；`initialize` 明确关闭，不声明 NTFS/ext4 运行通过。
- 网络盘、FAT/exFAT、SMB、可移动盘和断电持久性不从 API 成功自动推导；缺少 evidence 时保持 NOT RUN 或不支持。

## 错误语义

- `STATE_LOCKED`：`FAIL`/7，可重试。
- `STATE_CONFLICT`：`FAIL`/7，不自动修改现有状态。
- `STATE_WRITE_FAILED`：`FAIL`/7，安全创建、锁或写入失败。
- `STATE_ATOMIC_WRITE_UNSUPPORTED`：`FAIL`/7，不允许静默降级。
- `STATE_DURABILITY_UNCONFIRMED`：`FAIL`/7，final 完整可见但持久性确认失败，可通过重跑恢复确认。
- `PROJECT_ID_GENERATION_FAILED`：`FAIL`/8，不使用低质量 fallback。

错误 details 只输出有界、归类后的原因，不回显绝对路径或原始 OS 错误文本。

## 备选方案

### 初始化同时写 evidence/run/index

拒绝。它会把首次状态提交变成多文件部分事务，并在恢复语义冻结前产生循环引用或悬空引用。

### 只用 `os.Rename`

拒绝。Unix 可以覆盖目标，Go 也不承诺所有非 Unix 文件系统上的同等原子语义。

### 用目录名或绝对路径派生 project ID

拒绝。移动项目会改变身份，且持久状态会泄露或关联本机路径。

### 自动删除 stale lock/temp 或 force 覆盖

拒绝。advisory lock 不需要猜测 stale；清理只针对当前调用创建的精确 temp，不对模糊 glob 做破坏性操作。

## 风险与回退

- Windows reparse point、LockFileEx、handle-relative no-replace publication 和目录 durability 仍需原生实证；当前不使用基于可变绝对路径的 `CreateHardLink` 降级。
- Linux advisory lock、handle-relative `linkat`、目录 sync 和故障注入仍需原生实证。
- 拥有同一用户权限的恶意进程仍可能竞争替换 mount/inode；当前边界不宣传对同权限恶意 actor 的绝对防护。
- `.gameatelier` 和 lock 可能在 project state 发布前已建立；崩溃后重跑会重新加锁并继续，不把目录存在等同于已初始化。

回退时只移除尚未发布的 `initialize` 命令实现；不得自动删除用户已经生成的 `.gameatelier/project.json`。任何卸载/清理命令必须另行定义预览与确认语义。
