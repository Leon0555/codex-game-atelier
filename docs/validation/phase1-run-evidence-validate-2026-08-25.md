# Phase 1 静态 `validate` 与 run/evidence 事务验证

日期：2026-08-25  
状态：PASS（限定范围）

> 后续进展：本记录只描述当时的静态 baseline。ADR 0009 与 `phase1-godot-headless-2026-08-25.md` 已在其后加入显式 Headless 路径，ADR 0010 与 `phase1-run-scanner-clean-list-2026-08-25.md` 已加入只读 scanner；以下历史验证结论不作追溯改写。

## 范围结论

生产 Go CLI 已新增公开 `validate [--project <dir>]`。当前命令只做静态 baseline：严格 project state、pinned regular `project.godot`、Godot GDScript-only 边界，以及 run persistence readiness。它不启动 Godot，不代表 headless、场景、资源或脚本加载通过。

持久化范围限定为 macOS Apple Silicon/APFS。每次已开始的 validate 使用自包含 `.gameatelier/runs/<run-id>/`；`intent.json` 在 operation 前发布，payload/evidence 先完成并复验，`result.json` 最后 no-replace 发布并成为唯一逻辑提交点。

## 已通过验证

| 项目 | 结论 | 证据边界 |
| --- | --- | --- |
| Go 格式、单测、静态检查 | PASS | Go 1.27.0；`gofmt -d`、`go test -count=1 ./...`、`go vet ./...` |
| Race | PASS | `go test -count=1 -race ./...`；包含并发 unique runs 与全局生产版本冻结回归 |
| Schema | PASS | Draft 2020-12：14 个 schemas、9 个正向 fixtures；validate data、run intent、validation report 与 evidence closure 已覆盖 |
| Plugin/Skill validator | PASS | 官方 `validate_plugin.py` 与 `quick_validate.py`，使用项目本地 PyYAML/jsonschema；未执行用户级安装 |
| 三目标交叉编译 | PASS（形状） | Darwin arm64 Mach-O、Linux amd64 ELF、Windows amd64 PE；不等于 Linux/Windows 原生运行 |
| intent 时序 | PASS | begin 后、operation 前已有 immutable intent；operation-window `os.Exit(93)` 留下 intent 且无 result |
| 逻辑提交点 | PASS | result 前 create/write/fullsync/close/link 故障均无正式 result；result link 后故障保留权威 result 与完整 ref/hash/size 闭包 |
| stdout 一致性 | PASS | committed stdout 使用落盘的同一份 bytes；真实 partial writer 返回 8，不重写或重跑 committed run |
| begin 阶段分类 | PASS | 无 run root=`RUN_RECORDING_UNAVAILABLE`；无 intent orphan=`RUN_PREPARE_FAILED`；intent 后 incomplete=`RUN_COMMIT_FAILED` |
| 取消 | PASS | operation 前和 operation 中取消均提交一致的 `FAIL/6 COMMAND_CANCELLED` evidence |
| 路径/链接/替换 | PASS | 中文、空格、emoji；project/state pinned roots；拒绝 symlinked `project.godot`；begin 后路径 rename/replacement 不混合两个项目 |
| 资源边界 | PASS | project.godot 1 MiB；目录分批 256，最多 4096 entries/512 KiB names；JSON node/depth/string/key、payload/result 总量均有上限 |
| 文件系统 gate | PASS（注入） | state/runs/run/payload/evidence 每层要求 APFS 且同 FSID；注入 runs/run/payload identity 变化均无 result |
| 独立只读终审 | PASS | 0 Blocker、0 High、0 Medium、0 Low；接受为本限定 Phase 1 生产薄切片 |

## 尚未运行或不在本切片

- 真实断电测试未执行；当前依据 Go/Darwin `FULLFSYNC` 行为、APFS/FSID gate、故障注入与 hard-exit 测试。
- 真实嵌套非 APFS 或 network mount 演练未执行；当前用可注入 FSID 负例覆盖逻辑 gate。
- Linux x64、Windows x64 原生 durability/运行矩阵未执行，公开持久命令在这些宿主保持关闭。
- `go.mod` 声明 Go 1.24，但本轮只用项目本地 Go 1.27 编译；Go 1.24 原生工具链复验为 NOT RUN。
- run scanner、recovery、`.run.lock`、clean/index 未实现；当前 `status` 仍只读 `project.json`。
- Godot headless、场景、资源、GDScript 测试和实际引擎日志未覆盖。
- 项目源码不是锁定或内容寻址快照；未来 headless/build/export 必须另外处理并发内容变化、进程隔离与 artifact manifest。

本记录不扩大 Support Matrix，不允许声明 v1.0 已发布或 Godot 生产级验收完成。
