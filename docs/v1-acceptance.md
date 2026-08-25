# Codex Game Atelier v1.0 验收草案

状态：Phase 1 验收基线；部分 macOS Apple Silicon 薄切片已实证，其余项目保持 `NOT RUN` 或明确限定状态
日期：2026-08-24

## 1. 结果词汇

- `PASS`：验收条件已实际通过并有证据。
- `FAIL`：已执行但未通过。
- `BLOCKED`：外部条件缺失，无法执行。
- `SKIPPED`：按冻结规则明确不执行。
- `NOT RUN`：尚未执行，不能视为通过。

每条 evidence 至少记录命令/操作、产品与 Godot 版本、宿主、时间、退出状态、结构化结果、关键日志、产物/hash、失败复现和未覆盖范围。

## 2. 发布阻断原则

- 任一冻结矩阵的必选项为 `FAIL`、`BLOCKED` 或 `NOT RUN` 时，不得宣称对应组合生产级稳定。
- `SKIPPED` 必须有已冻结规则和理由，不能用于隐藏缺口。
- 失败 evidence 不删除、不覆盖；修复后产生新的 run 并关联旧记录。

## 3. 功能验收

| ID | 验收条件 | 最低证据 | 当前状态 |
| --- | --- | --- | --- |
| G-001 | 检测冻结版本 Godot、路径、架构和项目 | JSON + 实际二进制版本 | NOT RUN |
| G-002 | `doctor` 诊断缺失/错误版本、export templates、平台限制 | 正反例矩阵与稳定错误码 | NOT RUN |
| G-003 | 项目初始化幂等，不覆盖用户文件 | 首次/重复/冲突运行 diff | NOT RUN |
| G-004 | Headless 启动并正常退出 | 进程、退出码、Godot 日志 | NOT RUN |
| G-005 | 场景和资源验证能发现损坏、缺失与循环问题 | 有效/无效 fixture 与定位 | NOT RUN |
| G-006 | GDScript 测试可运行并映射通过/失败/超时 | 测试报告、退出码、日志 | PASS（macOS 固定入口薄切片；三宿主与完整框架仍 NOT RUN） |
| G-007 | 输入、信号、资源、UI 与基础玩法工作流可在参考游戏验证 | 自动化/可复现操作证据 | NOT RUN |
| G-008 | `build --profile debug|release` 分别执行默认目标工作流并产生可辨识 runnable artifact；复用 export 时引用同一底层 evidence | 命令、调用链、产物 manifest/hash | NOT RUN |
| G-009 | 指定 preset/目标的直接导出成功并在目标平台实际启动；macOS 只要求未签名/未公证产物的 Apple Silicon 技术验证 | Godot export 命令 + target smoke + 分发就绪标记 | NOT RUN |
| G-010 | 日志结构化、secret redaction 和稳定退出码 | schema/fixture/端到端结果 | PASS（committed run 零自由文本结构投影薄切片；raw 捕获、脱敏、分片和保留仍 NOT RUN） |
| G-011 | 中文、空格和特殊路径无未声明失败 | 三宿主路径矩阵 | NOT RUN |
| G-012 | 超时、取消、异常退出、残留进程/锁和恢复可诊断 | 故障注入与恢复 evidence | NOT RUN |

## 4. Agent、Skill 与状态验收

| ID | 验收条件 | 最低证据 | 当前状态 |
| --- | --- | --- | --- |
| A-001 | 分发内容不含具体模型 ID | 全分发包扫描 | NOT RUN |
| A-002 | 逻辑 Profile 支持能力等级、会话继承和用户覆盖 | 解析测试矩阵 | NOT RUN |
| A-003 | 原生子代理有界并行，默认不超过三，且单一 owner | 多任务演练与状态文件 | NOT RUN |
| A-004 | 只读 Auditor 不能在同一步修改并自批 | 权限/流程负例 | NOT RUN |
| A-005 | 中断后可从 task/handoff/evidence 恢复 | 进程中断与新会话恢复 | NOT RUN |
| A-006 | 顶层 Agents/Skills 数量精简、职责无重叠 | 触发测试与评审 | NOT RUN |
| A-007 | 内部 `AGENTS.md` 不进入任何分发物或用户项目 | 包内容扫描 | NOT RUN |

## 5. 门禁验收

| ID | 验收条件 | 最低证据 | 当前状态 |
| --- | --- | --- | --- |
| E-001 | `manual` 仍执行安全、输入、目标和必需前置门禁 | 命令负例 | NOT RUN |
| E-002 | `standard` 的 build/export 自动执行相应验证子集 | 调用/证据链 | NOT RUN |
| E-003 | `strict` 强制完整测试、证据新鲜度和发布条件 | 绕过负例 | NOT RUN |
| E-004 | 未安装或绕过 Git hook 不能绕过 CLI/CI 门禁 | `--no-verify`/无 hook 演练 | NOT RUN |
| E-005 | Git hooks 只在用户显式安装后存在，并可完整卸载 | 安装前后精确路径清单 | NOT RUN |
| E-006 | 发布 CI 检查必选、最小权限、受保护来源触发 | workflow 审计与演练 | NOT RUN |

## 6. 分发与生命周期验收

| ID | 验收条件 | 最低证据 | 当前状态 |
| --- | --- | --- | --- |
| D-001 | 普通用户不 clone 源码、不执行项目构建即可使用 | 干净用户流程录像/日志 | NOT RUN |
| D-002 | 已有 Godot 前置条件时，安装与初始化最多三个主要步骤 | 可复现步骤与计数规则 | NOT RUN |
| D-003 | Plugin 在支持的 Codex 客户端安装、加载、调用、卸载 | 干净环境矩阵 | NOT RUN |
| D-004 | Starter Template 可独立取得、初始化和回滚 | 包内容与端到端证据 | NOT RUN |
| D-005 | CLI 预构建/已打包，不要求 Node 前沿版或本地编译 | 三宿主安装与版本证据 | PARTIAL：本地多宿主 bundle/archive 与 Apple Silicon 入口通过；实际安装及 Linux/Windows 原生 NOT RUN |
| D-006 | 安装、升级、卸载和回滚保留用户项目与凭据 | 生命周期 diff/evidence | NOT RUN |
| D-007 | 无默认遥测、隐藏网络请求或隐藏外部写入 | 网络/文件系统审计 | NOT RUN |
| D-008 | 发布物带 checksum、来源记录和许可文件 | artifact manifest | NOT RUN |
| D-009 | npm 发布采用 Trusted Publishing、2FA 策略和 provenance | 预发布演练，不含真实发布 | NOT RUN |

## 7. Support Matrix 与发布验收

| ID | 验收条件 | 最低证据 | 当前状态 |
| --- | --- | --- | --- |
| R-001 | Support Matrix 已经用户审阅冻结并公开 | 版本化文档/ADR | NOT RUN |
| R-002 | macOS Apple Silicon 宿主验证通过 | 干净机器 evidence | NOT RUN |
| R-003 | Windows x64 宿主验证通过 | 干净机器 evidence | NOT RUN |
| R-004 | Linux x64 宿主与 headless CI 通过 | runner evidence | NOT RUN |
| R-005 | 每个承诺导出目标均实际 export 并在目标平台 smoke run | target evidence | NOT RUN |
| R-006 | 参考游戏完成初始化到导出的端到端验证 | 完整 trace/evidence | NOT RUN |
| R-007 | 文档、命令、版本、错误和实际行为一致 | 文档测试/审计 | NOT RUN |
| R-008 | 无 blocker/high 安全问题，性能无未解释严重回退 | 问题清单与基线 | NOT RUN |
| R-009 | 最终架构、安全、许可证和发布审计独立只读通过 | 审计报告 | NOT RUN |
| R-010 | 用户明确批准正式发布 | 可审计批准记录 | NOT RUN |

## 8. 明确不纳入 v1.0 验收

Unity、Godot .NET/C#、移动端/主机/Web 导出、Windows ARM、Linux ARM、macOS Intel 开发宿主和运行验证、Godot 游戏产物的 macOS 签名/公证与公开分发就绪验证，以及未冻结的 Godot 预发布/旧版本，除非用户通过新决策扩大 Support Matrix。扩大范围不能通过修改宣传措辞暗中完成。
