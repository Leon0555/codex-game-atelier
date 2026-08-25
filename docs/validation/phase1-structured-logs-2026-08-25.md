# Phase 1 结构化 Logs 验证

日期：2026-08-25
范围：`logs --run-id`、单 run committed closure 读取、零自由文本投影、Schema 与只读安全边界

## 结论

ADR 0012 的最小生产薄切片已实现。`logs` 只读取用户显式指定的 committed validate/test run，在同一次 rooted、有界读取中验证 intent/result/payload/evidence 完整闭包，再输出不含 source 自由文本的结构事件。它不保存或返回 Godot stdout/stderr，不启动 Godot，也不写项目状态。

## 验证矩阵

| 项目 | 结果 | 证据 |
| --- | --- | --- |
| 参数与路径 | PASS | strict run ID；拒绝 `../`；project 参数输出规范化为 `.` |
| 单次闭包读取 | PASS | scanner 分类函数在同次 pinned read 中返回已验证 result/payload/record 内存快照；无二次按路径读取 |
| 完整性 | PASS | intent/result binding、canonical JSON、payload preflight、record path/kind/hash/size/producer/time 均复用 scanner 硬门禁 |
| 资源上限 | PASS | 4 文件；intent 256 KiB，其他各 4 MiB；总预算 12.25 MiB；64 KiB chunk 间响应 context |
| 零 source 文本/ID | PASS | 不输出 source summary、error code/text/details、report ID/summary、payload 原文或路径；高熵 fixture 文本和项目控制 ID 均不出现在 logs stdout |
| 结构事件 | PASS | check/test/error/result 事件按原顺序排列；ID 由 CLI 固定生成为 `check/test/error-NNNN`，只含 allow-list outcome/level/source/kind |
| 只读 | PASS | 命令前后 `.gameatelier` 全树快照一致；`evidence=[]` |
| 状态分类 | PASS | missing、incomplete、corrupt、future schema、symlink run 与预取消均有稳定错误/退出码 |
| Schema | PASS | Draft 2020-12：18 schemas、20 fixtures、24 negative assertions；含 logs 参数、零 evidence、事件 source/kind/id/outcome/level 组合、source/evidence/exit 绑定和非空 PASS 约束 |
| Go 测试/race/vet | PASS | 最终普通测试 app 63.019 秒；race app 69.626 秒；`go vet ./...` 退出 0 |
| 交叉构建 | PASS（构建） | public CLI、private runner、app test binary：Darwin arm64、Linux amd64、Windows amd64；另构建 Darwin amd64 CLI/runner |
| Universal 2 | PASS（结构） | public CLI 与 private runner 均由最新 arm64/amd64 产物合并，`lipo -archs` 为 `x86_64 arm64`；本段未重跑 Godot 实机 test |

## 真实 committed run 查询

用最新 arm64 CLI 查询既有 Universal 2 Apple Silicon test PASS run：

`atelier-20260825t075201.740213000z-e5d3a439280d`

- logs 命令：PASS/0，6 ms；自身 `evidence=[]`。
- source：`test`、PASS/0、5 个测试；输出 6 个连续事件（5 test + `command-finished`）。
- integrity：`test-report`，702 bytes，SHA-256 `7e85b6f316782f559bfaab0730c27dad86f66904f28ce6f4fe78d2fae672d423`。
- `raw_output_included=false`；输出没有原 test ID/summary、error code/text、payload path 或项目绝对路径。
- 后续 scanner 仍为 17 committed、0 incomplete、0 orphan、1 historical corrupt/protected、0 candidates，证明 logs 没有创建 run 或改变分类。

## 明确未覆盖

- 原始 stdout/stderr 捕获、脱敏、分片、查询与保留：NOT RUN；需要新的隐私/存储决策。
- `latest` 自动选择、分页、follow/tail、时间范围：NOT RUN。
- 未知 schema 迁移和 corrupt run 恢复：NOT RUN。
- Windows/Linux 原生文件系统与运行时复验：按用户决定延期；只做交叉构建。

本薄切片不等于 G-010 的完整 raw 日志能力，也不代表 v1.0 发布门禁完成。
