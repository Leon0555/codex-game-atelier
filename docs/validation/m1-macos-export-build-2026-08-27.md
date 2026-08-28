# M1 macOS Build/Export 验证记录

日期：2026-08-27
宿主：macOS Apple Silicon
引擎：Godot `4.7.2.stable.official.ed1daf0bf` standard/GDScript
范围：`build`/`export` 共用执行链、Debug/Release Universal 2 ZIP、项目源写入隔离、evidence/Schema/回归

## 最终结果

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| Debug `build` 实机 | PASS | run `atelier-20260827t142727.123077000z-25080887e532`，34,063 ms，artifact 64,327,238 bytes，SHA-256 `5d69f3ad9856608f9b6525f2aed188c0f2fcc8732d75fabaaa69540b52d1ecb9` |
| Release `export` 实机 | PASS | run `atelier-20260827t142810.524566000z-2e6b32346bf2`，26,862 ms，artifact 59,652,867 bytes，SHA-256 `fa1c08df862aacc9ae070ada826415e5a0486ea3eceb970bebcefc23ace257ef` |
| Godot/preset/templates | PASS | 两条命令均固定 `macOS Technical`、`macos-universal2`、匹配 `4.7.2.stable` 的 `macos.zip`/`icudt_godot.dat`；`build` 不接受 preset 参数 |
| Universal 2 真实性 | PASS | CLI 有界展开 ZIP，要求唯一 app 主可执行文件；fat Mach-O 恰含 `x86_64`/`arm64`，校验 slice 范围、重叠、64-bit magic 与 thin/fat CPU type；单架构负例返回 `EXPORT_ARTIFACT_INVALID`/5 |
| Apple Silicon target smoke | PASS | 两个最终命令都从刚生成的 ZIP 有界安全解包 `.app`，固定执行 `--headless --quit-after 1 --no-header`；退出 0、输出未截断、无 `ERROR:`，manifest 记录 `macos/arm64/headless-one-frame` |
| 项目源隔离 | PASS | 最终两次 Godot 都从 run 内有界项目快照运行；运行前后 `git status` 无新增源文件，参考项目只保留原有两个 tracked `.uid`；run 内无 `.godot-project-snapshot` 或 `.godot-export-runtime` 残留 |
| 写入声明 | PASS | intent 只声明对应 run root、artifact root 与显式获准的 `godot:user-data:standard-os-location`；ZIP 先写快照内部固定路径，再复制到声明 artifact root |
| Artifact readiness | PASS | manifest 明确 `unsigned=true`、`not_notarized=true`、`public_distribution_ready=false`；本结果不是公开分发就绪声明 |
| Evidence 闭包 | PASS | 两个最终 run 均含 immutable intent、严格 export-artifact payload、SHA-256 evidence record 和最后提交的 result；`clean --list` 将新 run 判为 committed |
| Go | PASS | Go 1.27.0：`go test -count=1 ./...`、`go vet ./...`、`go test -race -count=1 ./...` |
| Schema | PASS | Draft 2020-12：20 schemas、25 fixtures、7 个持久化 Starter Template records、31 个负例断言；含 build/export result、intent、manifest/target-smoke 交叉语义 |
| Plugin/Starter 回归 | PASS | Python `unittest` 26 项通过 |

## 真实命令

两条最终命令都使用仓库内预构建 CLI/runner、项目内已批准的 Godot 4.7.2 与 export templates，并显式传入 `--allow-engine-user-data`。公开参数没有任意 Godot argv 或自由输出路径：

```text
codex-game-atelier build --project <reference-game> --profile debug --godot <Godot> --timeout-ms 180000 --allow-engine-user-data
codex-game-atelier export --project <reference-game> --profile release --preset "macOS Technical" --godot <Godot> --timeout-ms 180000 --allow-engine-user-data
```

stdout 各只有一个 command-result JSON，结果均为 `PASS`/0。绝对项目、Godot 和用户目录路径不写入持久化 intent/result/payload。

## 发现并修复的隐藏源写入

最初直接在源项目运行的 Release export `atelier-20260827t140240.145719000z-3780fe5c6747` 虽然返回 PASS，却生成了未声明的 `gameplay_state.gd.uid` 与 `tests/atelier_test_runner.gd.uid`。这证明“只声明 `.gameatelier` 写入、但让 Godot 在源项目运行”的方案不成立。两个文件确认是本次运行新建的未跟踪副产物后已从工作树移除；原有 tracked UID 未修改，失败/历史 run 与 artifact evidence 保留。

修复后，导出先把项目复制到 run 内快照，排除 `.git`、`.godot`、`.gameatelier`，拒绝 symlink/特殊文件和超限内容。测试中的 fake Godot 会主动创建 `generated.gd.uid`，最终只出现在瞬时快照并随快照清理；真实 Debug/Release 复验也未改变源工作树。

项目快照是正常相对写入隔离，不是恶意代码沙箱。项目代码或 editor plugin 主动发起的网络/绝对路径写入仍可能发生，因此命令只适用于用户拥有或已审阅的项目。

## 未覆盖

- 参考游戏当前还是薄切片，尚未完成 M1 约定的小而完整玩法扩展和全链 E2E。
- Intel slice 只做静态 Universal 2 验证，不做 Intel 实机 smoke。
- 游戏产物签名、公证和公开分发 readiness 按已批准范围不属于 v1.0。
- Windows/Linux 原生 runner/机器验证延期，不从 macOS 或交叉构建结果推断支持。
