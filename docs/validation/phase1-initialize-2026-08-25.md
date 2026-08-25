# Phase 1 `initialize` 原子状态切片验证

日期：2026-08-25（Asia/Shanghai）
原生验证宿主：macOS Apple Silicon / APFS
范围：已有 Godot/GDScript 项目的首次 Atelier 状态、并发、幂等、路径 containment、恢复与目标宿主 gate

## 结论

`initialize [--project <dir>]` 已成为第二个 Phase 1 生产薄切片。它只为已存在且通过前置检查的 Godot/GDScript 项目创建 `.gameatelier/project.json`；不创建或修改 `project.godot`，不运行 Godot，不写 evidence，不安装 hook，也不修复/迁移/覆盖既有异常状态。

本结论只接受 macOS Apple Silicon/APFS 的原生运行行为。Linux x64 与 Windows x64 仍属于 v1.0 Tier 1 目标，但当前命令显式返回 `INITIALIZE_HOST_NOT_VERIFIED`，直到各自原生文件系统、锁、故障与恢复矩阵通过。三目标交叉构建不等于原生运行支持。

## 验证结果

| 检查 | 结果 | 证据与限制 |
| --- | --- | --- |
| Go 格式与静态检查 | PASS | Go 1.27.0；`gofmt`、`go vet ./...` 退出 0 |
| 单元与竞态检查 | PASS | `go test ./...`、`go test -race ./...` 退出 0 |
| 三目标构建形状 | PASS（有限） | darwin/arm64、linux/amd64、windows/amd64 均可构建；后两者未原生执行 |
| 初次初始化 | PASS | APFS 上产生 schema 1.0.0、revision 0、standard mode、Godot 4.7.2-stable/GDScript 与 128-bit CSPRNG project ID |
| 幂等重跑 | PASS | `created:false`；前后 SHA-256、inode、mtime、size、mode、link count 相同 |
| 文件权限 | PASS | `project.json` 与 advisory lock 均为 `0600` |
| 中文/空格/emoji 路径 | PASS | 项目目录 `中文 项目 🚀` 上 initialize → status → initialize 均 exit 0 |
| 并发初始化 | PASS | 16 并发 writer 只产生一个 `created:true` 和一个 project ID；其余只会读取同一 final 或返回可重试锁冲突 |
| 不覆盖发布 | PASS | 同目录临时文件经 file sync/close 后用 handle-relative hard-link no-replace 发布；已有 final 保留 |
| 同根读取与 TOCTOU 缩减 | PASS（代码/测试） | state root 由 `os.Root` 固定；`project.json` 的 `Lstat`、打开、`fstat` 与 `os.SameFile` 在同一 root 下完成，并限制为 1 MiB |
| 无效/未来/symlink state | PASS | 返回 `STATE_CONFLICT`/7，不覆盖 state；前置冲突不创建 lock |
| 锁冲突与恢复 | PASS | advisory exclusive lock 返回可重试 `STATE_LOCKED`；持有进程退出后可恢复 |
| crash temp 边界 | PASS | 只清理当前调用的精确 temp；外部/旧 temp 不被 glob 删除 |
| durability 确认 | PASS（API 行为） | 首次发布 sync 文件并 sync 目录；合法重跑也重新确认 file/directory sync。未宣称断电、网络盘或全部文件系统持久性 |
| evidence/state 索引 | NOT RUN | 本切片刻意保持 `evidence: []`；不创建 runs/evidence/index 或 `last_command_result_ref`。后续默认持久化政策已获用户批准，但多文件事务尚未实现 |
| Linux x64 原生 | NOT RUN | 命令被 gate；需原生 ext4/runner 测试后启用 |
| Windows x64 原生 | NOT RUN | 命令被 gate；需安全的 handle-relative no-replace 方案及 NTFS/reparse/LockFileEx 原生测试后启用 |
| 独立只读终审 | PASS（范围限定） | Blocker 0、High 0、Medium 0，无未解决的可操作代码缺陷；接受为 macOS Apple Silicon/APFS 首个有写入行为的生产薄切片，不代表 v1.0 发布或其他宿主支持 |
| Draft 2020-12 完整语义验证 | PASS | 项目本地 `jsonschema` 4.25.1 验证 11 个 schemas、6 个正向 fixtures 和 3 个 initialize 负向断言 |
| Plugin/Skill 官方 validator | PASS | 项目本地 PyYAML 6.0.3；`quick_validate.py` 与 `validate_plugin.py` 均退出 0；未进行用户级安装 |

## 已验证的不变量

- 所有 Godot 项目、宿主与 .NET 前置检查在任何 `.gameatelier` 写入前完成。
- project ID 不由路径、目录、时间、机器或账号派生；初始化结果不把绝对项目路径写入 command arguments。
- 合法既有 v1 state 是字节级 no-op；未知 schema 不自动迁移，异常 state 不自动 repair/force。
- 发布不使用可覆盖目标的 rename，也不把不支持 no-replace 的文件系统静默降级为弱语义。
- final 完整可见但目录同步/精确 temp 清理无法确认时返回 `STATE_DURABILITY_UNCONFIRMED`，不谎报 PASS；重跑可重新确认合法 final 的持久性。
- Windows 不采用由可变路径字符串拼接出的 `CreateHardLink` 降级，避免 reparse/rename 竞态逃离已固定 state root。

## 仍未证明

- Linux/Windows 原生锁、目录同步、文件系统错误映射、故障注入与干净环境行为。
- 网络盘、SMB、FAT/exFAT、可移动盘和断电后的持久性。
- 多文件 artifact/evidence/run/state 事务；这必须作为独立薄切片冻结提交点与恢复规则。
- Plugin 的用户级安装、升级、回滚与卸载；本次没有进行任何用户级或外部写入。

## 审计结论与后续门禁

独立只读审计已确认本轮 TOCTOU 缩减、close 生命周期、原子发布、恢复边界、平台 gate 与文档一致，未发现 blocker/high/medium 或剩余可操作 low 缺陷。

1. 分别取得 Linux x64、Windows x64 原生 runner evidence 后才解锁对应宿主的 `initialize`。
2. 多文件 evidence 写入必须先形成独立契约，不从单文件初始化的成功自动推导事务安全。
