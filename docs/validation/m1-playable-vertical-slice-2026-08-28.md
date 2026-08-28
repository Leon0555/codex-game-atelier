# M1 可玩垂直切片与特殊路径端到端验证

- 日期：2026-08-28
- 结论：**PASS（M1 macOS Apple Silicon 本地退出条件）**
- 范围：Godot `4.7.2-stable` standard/GDScript、Starter Template、参考游戏、中文/空格/`#` 项目根、Debug/Release macOS 技术导出
- 非结论：不是干净用户环境、Plugin 客户端生命周期、签名/公证、公开分发或 Windows/Linux 原生验证

## 可玩垂直切片

Starter Template 与参考游戏现在都包含 **Atelier Spark · 工坊火花**：用户点击按钮或按 Space/Enter 收集 5 个火花，达到目标后进入不可重复计分的胜利态，并可重置开始新一轮。这个小循环实际覆盖：

- Godot 标准输入 action。
- `score_changed` 与 `game_won` signals。
- 中文与空格路径中的预加载资源。
- Label、ProgressBar、Button 和完整 UI 场景实例化。
- 初始、进行中、胜利和重置后的确定性状态。
- 无外部素材、无第三方测试依赖的六项固定 GDScript 测试。

Starter Template 新增 `macOS Technical` preset：Universal 2、Debug/Release 共用、签名和公证关闭。示例 bundle identifier 明确要求用户在真实分发前替换。

## 特殊路径全链

模板复制到被忽略的 `<workspace>/.tools/M1 垂直切片 #20260828` 后运行完整流程。源模板没有生成 `.gameatelier`、`.godot` 或 `.uid`；运行状态和 artifact 只存在于副本。

| 步骤 | 结果 | 证据 |
| --- | --- | --- |
| `detect` | PASS | 中文、空格、`#` 路径与显式 Godot 均识别，退出 0 |
| `doctor --export` | PASS | Godot `4.7.2.stable.official.ed1daf0bf` 与 `4.7.2.stable` macOS templates 均通过 |
| 首次/重复 `initialize` | PASS | 首次创建 CSPRNG identity；重复执行 state 330 bytes、SHA-256 `56f3c6075459b8681032bb4385d7760f252af5f7fbd44bacb0053c85955c3126` 与 mtime 不变 |
| Headless `validate` | PASS | run `atelier-20260828t130252.694404000z-82e100ce228e`，8/8 checks，退出 0 |
| GDScript `test` | PASS | run `atelier-20260828t130248.479542000z-523363d1d676`，6/6 tests，退出 0 |
| Debug `build` | PASS | run `atelier-20260828t130330.285799000z-ad5745ce94b3`；64,329,953 bytes；SHA-256 `16add9b67a053cbb846093e6a4641bcd898b22c214eedcba99328b62f7fd0537` |
| Release `export` | PASS | run `atelier-20260828t130409.705786000z-d2410b2fe1e4`；59,655,625 bytes；SHA-256 `03fb441a54f83e8c7606a5d0bde229df6289c2eb63ffbbe3234d694bfca4a695` |
| Target smoke | PASS | 两个 ZIP 都先验证恰含 `x86_64 + arm64`，再在当前 Apple Silicon 宿主以固定 headless 一帧命令启动并退出 0 |
| `clean --list` | PASS | committed 4、incomplete 0、orphan 0、corrupt 0、candidate 0 |

两个 artifact manifest 都固定 `unsigned=true`、`not_notarized=true`、`public_distribution_ready=false`。本结果只证明 Apple Silicon 技术导出；不宣称 Intel 实机验证或公开分发就绪。

仓库内正式 `examples/reference-game` 随后独立复验：Headless run `atelier-20260828t131320.875250000z-d8b09234153f` 为 8/8 PASS，test run `atelier-20260828t131318.198880000z-7e3ffb008fb5` 为 6/6 PASS，Release export run `atelier-20260828t131352.668932000z-1d52a0c4835d` 完成 target smoke；artifact 59,656,013 bytes、SHA-256 `a2786063305c0e604ea0de14fb84fe84e7fcbbd0aef21b3487f5a1b7656101b2`。参考项目源未新增 `.uid` 或其他未声明文件。

## Template package 复验

当前 9 个模板源文件重新通过固定 allowlist、可玩循环、六项测试结构、导出 preset、无模型 ID/旧产品名、无 generated state、无 link/特殊文件等门禁。配套方案仍是 ADR 0014 的 A：Template 不嵌入 Plugin、Skill 或 CLI。

| 检查 | 结果 |
| --- | --- |
| 两次 archive 逐字节复现 | PASS |
| archive | 6,640 bytes；SHA-256 `30573faf9eb5df215a02b42ba910080340887a0dc569ee44814bfe984982e318` |
| manifest | 2,383 bytes；SHA-256 `141652bd29926e15b676e2d882d716f7fe7fdf440a92344e9d0ba1b19c2da542` |
| 内容 | 11 个非 manifest 文件，展开 13,450 bytes；Template/Plugin candidate 均为 `0.2.0` |
| 安全重验 | PASS；安全解包后独立重跑源契约 |

最终回归：Go `go test -count=1 ./...` PASS；Draft 2020-12 为 20 schemas、25 fixtures、11 个持久化 Template/M1 记录、31 个负例断言；Starter/Template package/Plugin 共 26 项 Python 测试 PASS。

当前证据位于 [`evidence/m1-vertical-slice-2026-08-28/`](evidence/m1-vertical-slice-2026-08-28/)，包含 11 个经 Schema 验证的产品记录、源哈希、Template manifest、archive 摘要和脱敏执行摘要。

## 环境观察

外置 APFS 工作区中的原始官方签名 Godot 执行文件在本次会话被 macOS 以 exit 137 终止，`spctl` 同时报告 Code Signing subsystem internal error；连 `--version` 都无法完成。为继续已授权的项目本地验证，使用同一安装来源的项目内 APFS 写时复制副本并做 ad-hoc 重签名，之后 Godot 自报官方 `4.7.2` build，全部流程通过。

这项处理没有修改系统 Gatekeeper、Keychain、用户级 Godot 安装或全局配置。ad-hoc 重签名只恢复项目本地可执行性，不是游戏签名、公证或发布身份，也不替代 M3 干净环境复验。

## M1 结论与剩余边界

三里程碑路线中的 M1 本地退出条件已满足：模板可玩、完整特殊路径流水线通过、Debug/Release 技术产物可在 Apple Silicon 自动启动并正常退出。以下事项继续由后续里程碑承担：

- M2：逻辑模型 Profile、一次真实有界协作、manual/standard/strict、`release check`、可选 hook 和最小 CI。
- M3：干净用户环境、Plugin 实际安装/发现、升级/卸载/回滚、供应链与最终独立审计。
- 延后：Windows/Linux 原生、Intel Mac smoke、Godot 游戏签名/公证和公开发布。
