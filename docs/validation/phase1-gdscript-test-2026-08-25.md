# Phase 1 固定 GDScript Test 验证

日期：2026-08-25
范围：公开 `test` 命令、固定 Godot/GDScript 测试入口、结果映射、原子 evidence、scanner 闭包与 macOS Apple Silicon 实机薄切片

## 结论

ADR 0011 的第一版生产薄切片已实现。CLI 只运行固定 `res://tests/atelier_test_runner.gd`，不接受任意脚本路径、Godot argv 或 shell 命令；在显式授权标准 `user://` 后，复用 version/test 两阶段 pinned runner/engine 快照、总超时、取消、输出上限和进程组回收。严格测试 marker 被重新编码为一份 `test-report`，`result.json` 仍最后发布。

官方标准 Godot `4.7.2.stable.official.ed1daf0bf` 在 macOS Apple Silicon 上完成参考游戏 5 项真实 GDScript 测试：中文/空格资源、信号、输入、UI 和基础玩法状态均 PASS。原生 arm64 CLI 与 Universal 2 CLI 在 Apple Silicon 上各通过一次；按已冻结范围不做 Intel Mac smoke。

## 验证矩阵

| 项目 | 结果 | 证据 |
| --- | --- | --- |
| 固定命令参数 | PASS | 私有 runner 只接受 `version/scene/test` allow-list；test 固定为 `--headless --path . --script res://tests/atelier_test_runner.gd --no-header` |
| 未授权门禁 | PASS | `ENGINE_USER_DATA_NOT_AUTHORIZED`、`BLOCKED`/4；fake Godot 未启动，仍提交可验证 report closure |
| 固定入口缺失 | PASS | `GDSCRIPT_TEST_RUNNER_NOT_FOUND`、`BLOCKED`/4；不启动 Godot |
| 报告协议 | PASS | 唯一 marker、strict JSON、duplicate key/未知字段/重复 ID/矛盾 outcome/空 PASS/多 marker/非法 UTF-8/控制字符和纯空白摘要负例均拒绝；Unicode 摘要按字符计数 |
| 断言失败 | PASS | fake 与真实 Godot GDScript 负例均以有效 FAIL report + exit 1 映射 `GDSCRIPT_TESTS_FAILED`、`FAIL`/3，逐项失败计数持久化 |
| 无效报告/引擎失败 | PASS | exit 0 但无有效 marker 映射 `GDSCRIPT_TEST_REPORT_INVALID`、`FAIL`/5；Godot ERROR、异常退出和截断沿用稳定 engine errors |
| 超时与取消 | PASS | public test 的 fake process timeout 为 `GODOT_TIMEOUT`/6；预取消为 `COMMAND_CANCELLED`/6，二者均提交失败 evidence |
| 原子 evidence | PASS | test intent/result/test-report 的 command、scope、outcome、counts、hash/size 和 external-write declaration 由 preflight 共同验证 |
| Scanner | PASS | 修正原 validation-report 文件名假设后，真实失败/成功 test run 均为 committed；非法/未知闭包保持 corrupt/protected |
| Schema | PASS | Draft 2020-12：17 schemas、19 fixtures、15 negative assertions，包含 FAIL 聚合闭包与 test result/report/intent 跨 fixture 语义 |
| Go 单元测试 | PASS | `go test -count=1 ./...`，串行 app 34.272 秒；与 race 并发复验 app 49.930 秒 |
| Go race | PASS | `go test -count=1 -race ./...`，串行 app 35.716 秒；与普通测试并发复验 app 51.586 秒 |
| Go vet | PASS | `go vet ./...` |
| 交叉构建 | PASS（构建） | public CLI、private runner、app test binary：Darwin arm64、Linux amd64、Windows amd64；不等于目标宿主原生运行 |
| Universal 2 | PASS（Apple Silicon） | CLI 与 runner 都为 `x86_64 arm64`；Universal 2 bundle 在 Apple Silicon 真实 test 5/5 PASS |

## 真实运行

原生 arm64 最终 PASS run：

`atelier-20260825t074407.795469000z-1ae781734cd3`

- outcome/exit：`PASS`/0
- duration：9,883 ms
- engine：`4.7.2.stable.official.ed1daf0bf`
- tests：5 total、5 passed、0 failed
- evidence：`.gameatelier/runs/<run-id>/evidence/0001-test-report.json`

Universal 2 Apple Silicon PASS run：

`atelier-20260825t075201.740213000z-e5d3a439280d`

- outcome/exit：`PASS`/0
- duration：16,105 ms
- tests：5 total、5 passed、0 failed

原生 arm64 真实断言失败 run：

`atelier-20260825t075446.049416000z-96ab2def59b2`

- outcome/exit：`FAIL`/3
- error：`GDSCRIPT_TESTS_FAILED`
- duration：9,212 ms
- tests：5 total、4 passed、1 failed
- 方法：用补丁临时把 gameplay 期望改为必然失败，运行后立即用补丁恢复源文件；失败 evidence 保留

首次真实 attempt `atelier-20260825t074259.026104000z-8bf782134278` 提交为 `FAIL`/5。沙箱内只读复现同时发现参考断言期望字符串与实际 fixture 不一致，以及 sandbox 阻止已声明的标准 macOS user-data/log 路径。修正 fixture 期望，并在已批准的标准 `user://` 范围外运行后得到上述 PASS；首次失败 result/evidence 保留，没有删除或覆盖。

最终只读 scanner：17 committed、0 incomplete、0 orphan、1 corrupt/protected、0 candidates。唯一 protected 仍是 ADR 0009 前生成且缺失 external-write declaration 的历史 Headless run；四条本轮 test run（首次受限环境失败、真实断言失败、原生成功、Universal 2 成功）均通过新闭包验证。

首次同时运行普通测试和 race 时，测试夹具的 2 秒成功路径预算在双重编译/进程负载下触发超时，结果保留为一次失败复验。将成功路径及进程启动观察的测试专用预算提高到 5 秒后，普通测试与 race 再次并发均 PASS；产品默认/用户指定超时契约及 100 ms 超时负例没有改变。

最终独立只读审计：Blocker 0、High 0、Medium 0、Low 0。审计者独立复验了普通测试、race、vet、Schema、Linux/Windows 交叉构建、格式和 diff 检查；官方 Godot 参考游戏与 Universal 2 实机结论复用本记录中的既有运行证据。

## 安全边界

- `test` 是显式执行用户项目 GDScript 的操作，不是通用 eval 或 shell。用户只应对拥有或已审阅的项目运行。
- CLI 固定启动路径、引擎/runner 身份、进程 containment 和结果协议，但不隔离项目代码发起的网络、绝对路径或其他副作用。
- intent 中的 `godot:user-data:standard-os-location` 记录 CLI 可预期的引擎标准写入，不是对任意项目代码能力的沙箱证明。
- 原始 stdout/stderr 尚不持久化；本切片只保存严格派生 test report。日志脱敏、分片和查询留给后续 `logs`。
- 测试入口前后 hash 可发现该入口变化，但项目全树不是不可变快照；完整内容 manifest 与干净环境复验留给 build/export/release 门禁。

## 未运行/剩余范围

- Windows x64、Linux x64 原生 runner 与 Godot test：按用户决定延期；只有交叉构建。
- Intel Mac smoke：明确不要求；Universal 2 仅验证 Apple Silicon。
- 第三方测试框架适配、过滤、异步/fixture 隔离、覆盖率和性能测试：NOT RUN。
- 原始日志持久化与 secret redaction：NOT RUN。
- 完整场景/资源图、Debug/Release build、export、Plugin/Starter Template 安装闭环：NOT RUN。

本记录不扩大 Support Matrix，不代表 v1.0 已达到生产发布门禁。
