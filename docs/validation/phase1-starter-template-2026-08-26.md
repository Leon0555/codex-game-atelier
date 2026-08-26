# Phase 1 Starter Template 本地验证记录

- 日期：2026-08-26
- 结论：**PASS（仅限模板项目本体、项目本地复制与 Apple Silicon 当前 CLI 工作流）**
- 非结论：不是独立分发/安装、Plugin 客户端发现、生命周期、Linux/Windows 原生或 v1 发布通过
- 宿主：macOS 26.6.2 / Darwin 25.6.0 / arm64
- Godot：`4.7.2.stable.official.ed1daf0bf` standard build
- CLI/runner：本地 Plugin finalcheck bundle `0.2.0`

## 静态内容

模板固定 8 个非空文字文件与 3 个目录：README、`.gitignore`、Godot project/scene、两个 scripts、固定 test runner 和中文/空格资源。没有 `.gameatelier`、`.godot`、UID/cache、export artifact、`AGENTS.md`、可执行文件、具体模型 ID 或 .NET 配置。

`validate_starter_template.py` 与 6 项回归测试均 PASS，覆盖真实模板、生成状态/未知内容、.NET/缺失 runner、root/内部 symlink 与 hardlink、常见具体模型家族 ID/旧名变体、注释掉的结构标记、必需 ignore 规则、三步内容，以及持久化证据与当前模板哈希的一致性。静态 validator 只证明固定 allowlist 与已知模式门禁，不声称识别所有未来模型名称；GDScript 语法与可运行性由下节真实 Godot 结果证明。

## 复制后端到端结果

方案 A 冻结后的最终验证副本路径：`.tools/Starter 模板验证 #3 方案A`（包含空格、中文与 `#`）。该目录被仓库 `.gitignore` 排除，未进入 Git；早期 `#1`/`#2` 副本保留实现中验证历史，不作为当前源快照。

| 步骤 | 状态 | 证据 |
| --- | --- | --- |
| 未显式 Godot 的 detect | BLOCKED（预期，早期 `#1`） | 项目识别成功；隔离 Godot 不在 PATH，`GODOT_NOT_FOUND`/4；零写入 |
| 显式 Godot detect | PASS | project/Godot/host 均 detected，退出 0 |
| 首次 initialize | PASS | 新 `project_id`，created=true，revision 0，退出 0 |
| 重复 initialize | PASS | created=false；当前 `project.json` SHA-256 `4e0bbbd6dae66eb90a81f81e2fdc6cb5e8d7fb1bf49646a9947f66a73d9326e7`、330-byte size 与 mtime 前后完全相同 |
| Headless validate | PASS | run `atelier-20260826t015452.322710000z-5be5443b9bd7`，8 checks，退出 0 |
| 固定 GDScript test | PASS | run `atelier-20260826t015509.471623000z-45fb9bf3a65d`，5/5，退出 0 |
| 源/副本差异 | PASS（限定） | `diff -qr` 只发现副本新增 `.gameatelier`；源模板未生成身份或缓存 |

Headless/test 按既有授权在沙箱外允许 Godot 写标准 `user://`；CLI intent 只记录符号化 `standard-os-location`。

## 可复核证据

跟踪证据位于 [`evidence/phase1-starter-template-2026-08-26/`](evidence/phase1-starter-template-2026-08-26/)，不包含本机绝对路径、Godot `user://` 实体位置或凭据。

| 文件 | 用途 |
| --- | --- |
| `execution.json` | 宿主、Godot/包版本、CLI/runner SHA-256、脱敏命令和 initialize 前后元数据 |
| `initialize-first.json` / `initialize-repeat.json` | 首次与幂等 initialize 原始结构化结果 |
| `project-state.json` | 当时持久化项目状态；重建字节的 SHA-256 与 execution 记录一致 |
| `validate-result.json` / `validation-report.json` | Headless validate 结果与 8 项检查 payload |
| `test-result.json` / `test-report.json` | 固定 GDScript test 结果与 5 个 case payload |
| `source-files.sha256` | 运行时候选模板的 8 个源文件哈希；回归测试直接与当前源匹配 |

执行顺序为：

1. `codex-game-atelier detect --project <copy> --godot <godot-4.7.2-standard>`
2. `codex-game-atelier initialize --project <copy>`
3. `codex-game-atelier validate --project <copy> --headless --godot <godot-4.7.2-standard> --timeout-ms 30000 --allow-engine-user-data`
4. `codex-game-atelier test --project <copy> --godot <godot-4.7.2-standard> --timeout-ms 30000 --allow-engine-user-data`
5. `codex-game-atelier initialize --project <copy>`

这些文件是 Phase 1 本地 candidate 证据，不替代后续干净环境、安装/卸载或发布门禁。

## 尚未运行

- Plugin 配套的 Codex 客户端真实安装/发现；ADR 0014 已 Accepted 方案 A，但客户端生命周期证据仍未完成。
- Codex 客户端新任务中的发现、三步可用性录制和真实用户安装。
- Template archive/checksum 已在本地 candidate PASS；真实取得、Plugin 升级/卸载/失败回滚与干净 Codex 环境复验未运行。
- `.gameatelier` 的最终版本控制政策。
- Linux/Windows 原生与 Intel Mac smoke。
