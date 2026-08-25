# Phase 1 Godot Headless 与外部写入契约验证

日期：2026-08-25
状态：PASS（macOS Apple Silicon/APFS 参考项目薄切片）

## 范围结论

生产 Go CLI 的 `validate` 已新增显式 Headless 模式。默认静态 baseline 不变；`--headless` 只有在同时提供 `--allow-engine-user-data` 后才会启动 Godot。未授权调用会在进程启动前提交 `BLOCKED` evidence，授权调用会在 immutable intent 中记录 `godot:user-data:standard-os-location`，而不持久化用户绝对路径。

真实参考项目使用官方标准 Godot `4.7.2.stable.official.ed1daf0bf`，在 macOS Apple Silicon 上完成主场景一帧并退出 0。主场景预加载 `res://中文 资源/status_payload.gd`，因此本次运行同时经过中文、空格资源路径。该薄切片不等于完整 GDScript 测试、玩法、输入、UI 或全资源图验证。

## 实际结果

| 项目 | 结论 | 证据 |
| --- | --- | --- |
| 未授权门禁 | PASS | `ENGINE_USER_DATA_NOT_AUTHORIZED`、`BLOCKED`/4；fake Godot marker 未创建 |
| 固定引擎调用 | PASS | 公开 CLI 与 sibling 私有 runner 分离；runner 只接受 fd 3/4/5 与随机 nonce 控制消息，先 `fchdir(fd 3)`，再以同一 PID exec 固定的 version 或 `--headless --path . --quit-after 1 --no-header`；不接收任意 executable/引擎 argv |
| Pinned 执行身份 | PASS | runner 与 Godot 的已打开源 fd 均经过前后散列；runner 使用 64 MiB 有界 copy，Godot 使用同卷 APFS clone，两者均采用固定标识的本地 ad-hoc 重签名；两阶段各自的 runner/engine 快照散列必须一致且执行后不变；公共 runner 或 Godot 路径在阶段间替换不会重定向 scene |
| Pinned 项目身份 | PASS | 引擎前/后均核对公开路径与 pinned root；路径在启动前替换时不启动 Godot，运行中替换时丢弃 observation；Godot 只能在原目录 fd 上运行 |
| 版本门禁 | PASS | 进程必须自报 `4.7.2-stable` 官方标准版形状；不支持版本在场景前阻断。官方包身份另由下载散列、签名和环境盘点证明，不从版本文本推断 |
| 进程控制 | PASS | 默认总超时 30 秒；显式总超时、取消、非零退出、scene 已启动后的子进程组清理和 version/scene × stdout/stderr 的 256 KiB/stream 上限均有 Go 回归测试；取消、超时、截断、非零退出优先级已固定 |
| Godot 错误识别 | PASS | bounded stderr 中任一 `ERROR:` 即使进程退出 0 仍返回 `GODOT_REPORTED_ERRORS`/5 |
| 真实 Headless | PASS | timeout/symlink 修复后的 runner/engine 双固定最终复验 run `atelier-20260825t044302.152803000z-14ffb2b210e5`，duration 9,520 ms，8 项检查全部 PASS；获准访问标准 macOS `user://`；最终 run 内无 runner/engine 快照或 codesign 临时文件 |
| 外部写入声明 | PASS | intent 同时包含项目 run root 与 `declared_external_writes=["godot:user-data:standard-os-location"]` |
| Evidence 闭包 | PASS | intent、validation payload、SHA-256 evidence record、result 全部位于同一 immutable run；result 最后提交；任一 runner/engine snapshot 或 `.cstemp` 仍存在时 result 发布门禁拒绝提交 |
| Schema | PASS | Draft 2020-12：14 schemas、13 fixtures、6 negative assertions，并验证 headless command/result/report scope、outcome 和 check count 一致 |
| Go 单测/静态检查 | PASS | Go 1.27.0：`go test -count=1 ./...`、`go vet ./...` |

## 明确副作用

本次经用户批准的真实运行创建并使用 Godot 官方标准目录：

`~/Library/Application Support/Godot/app_userdata/Codex Game Atelier Thin Slice`

CLI 没有修改系统配置、用户 `HOME`、Godot 安装或全局 Codex 配置。该目录未被清理；删除属于独立操作，必须先列出并重新确认。

## 失败证据与修正

首次尝试把 `/dev/fd/3` 直接传给 Godot，run `atelier-20260825t033913.140224000z-1d446f299139` 按契约提交 `FAIL`/5。只读诊断确认 Godot 报告 `Invalid project path specified: "/dev/fd/3"`。实现随后改为私有 runner 先 `fchdir(fd 3)`、再以 `--path .` exec Godot。

独立审计随后发现 version/scene 重复解析公共 Godot/runner 路径的竞态，实现改为从各自已打开源 fd 创建阶段独立执行快照。macOS 实测表明未经重签名的 clone 会因复制后签名无效被系统终止；现在只对瞬时 Mach-O 快照执行固定的本地 ad-hoc 重签名。旧实现的超时试验 run `atelier-20260825t041814.681509000z-bc722ece95fd` 曾提交 `BLOCKED`/4 并遗留一个 79 MiB `.godot-scene-snapshot.cstemp`；该历史结果不代表最终契约，当前 timeout 统一为 `FAIL`/6，清理或瞬时文件存在会阻止 result 发布。用户后续明确批准定点删除该 `.cstemp`；删除后同一 run 的 intent、result 和 validation evidence 均保留。沙箱内试验 run `atelier-20260825t042005.079422000z-72e7a6794f64` 因标准 macOS user-data/log 目录不可写而提交 `FAIL`/5；获准访问声明目录后最终 run PASS。所有失败 result/evidence 均保留，没有通过删除证据获得 PASS。

## 尚未覆盖

- 原始 Godot stdout/stderr 尚未持久化；当前只保留严格脱敏 checks、阶段与稳定错误码。受控日志保留属于后续 `logs` 契约。
- Headless 会执行项目主场景 GDScript；参考项目代码已限定为预加载资源、打印结构化 marker 和退出。本薄切片不沙箱化任意项目脚本主动发起的网络或绝对路径操作。
- 当前安全边界阻止正常的公共路径替换、更新竞态和 version 阶段自修改影响 scene；不宣称抵御能够定位并主动篡改私有 run staging 的恶意同一用户并发进程。
- 项目运行期间的全内容快照/单文件内容并发修改检测尚未完成；当前能阻止公开根路径替换造成的执行重定向，但固定参考项目验证不等于任意活动项目的一致内容快照。
- Windows x64、Linux x64 原生 runner 按用户决定后移，未运行。
- 完整 GDScript 测试、输入、信号、UI、玩法、Debug/Release build 与 export 不属于本薄切片。
- 这不是 v1.0 生产级发布声明。

## 官方依据

- [Godot 4.7 数据路径](https://docs.godotengine.org/en/4.7/tutorials/io/data_paths.html)
- [Godot 4.7.2 `OS::get_user_data_dir`](https://github.com/godotengine/godot/blob/4.7.2-stable/core/os/os.cpp)
- [Godot 4.7.2 macOS 数据路径实现](https://github.com/godotengine/godot/blob/4.7.2-stable/platform/macos/os_macos.mm)
