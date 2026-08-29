# Codex Game Atelier v1.0 验收基线

状态：三里程碑发布门禁；M1 macOS Apple Silicon 本地闭环已实证
更新日期：2026-08-28

## 1. 结果词汇

- `PASS`：验收条件已实际通过并有证据。
- `FAIL`：已执行但未通过。
- `BLOCKED`：外部条件缺失，无法执行。
- `SKIPPED`：按冻结规则明确不执行。
- `NOT RUN`：尚未执行，不能视为通过。

每条 evidence 至少记录命令/操作、产品与 Godot 版本、宿主、时间、退出状态、结构化结果、关键日志、产物/hash、失败复现和未覆盖范围。

任一冻结矩阵必选门禁为 `FAIL`、`BLOCKED` 或 `NOT RUN` 时，不得宣称对应组合生产级稳定。失败 evidence 不删除、不覆盖；修复后产生新 run 并关联旧记录。

## 2. 十二个发布门禁

以下十二项是 v1.0 对外发布的唯一顶层阻断清单。实现与测试仍可保留更细编号，但不得为同一行为制造重复审批或重复演练。

| 门禁 | 里程碑 | 验收结果 | 最低证据 | 当前状态 |
| --- | --- | --- | --- | --- |
| V1-01 环境与项目 | M1 | 正确检测 Godot/宿主/项目；`doctor` 覆盖版本、export templates 和限制；初始化幂等且不覆盖用户文件；Headless 可正常退出 | 正反例 JSON、稳定错误码、首次/重复 diff、实际进程 | PASS（macOS Apple Silicon；特殊路径完整链与幂等状态证据） |
| V1-02 验证、测试与玩法 | M1 | Godot 实际加载场景/资源并定位损坏；GDScript 通过/失败/超时可映射；参考游戏覆盖输入、信号、资源、UI 和基础玩法 | 有效/无效 fixture、测试报告、参考游戏操作证据 | PASS（macOS Apple Silicon；Atelier Spark 六项测试 6/6，既有失败/超时映射回归保留） |
| V1-03 构建与导出 | M1 | Debug/Release `build` 与指定 preset `export` 复用同一执行链，生成有 manifest/hash 的 runnable artifact，并完成目标 smoke | Godot 命令、evidence 关联、产物清单/hash、启动退出结果 | PASS（macOS Apple Silicon；Debug build 与 Release export 均自动完成 Universal 2、manifest/hash 和 headless 一帧 target smoke） |
| V1-04 路径、日志与恢复 | M1 | 中文/空格/特殊路径可用；结构化日志与稳定退出码可诊断；超时、取消、异常退出和残留进程/锁有明确恢复结果 | 路径矩阵、故障注入、进程/锁检查、结构化日志 | PASS（macOS Apple Silicon；中文/空格/`#` 全链、既有故障注入、进程组与 run closure 证据） |
| V1-05 模型、Agent 与状态 | M2 | 分发无具体模型 ID；逻辑 Profile 支持能力等级、继承和覆盖；原生子代理默认不超过三、单一 owner、只读审计不可自批；可由 task/handoff/evidence 恢复 | 分发扫描、Profile 解析矩阵、一次中断/恢复协作 trace | PASS（Profile 目录、九项解析矩阵、分发扫描、task/handoff 逻辑引用，以及无对话继承的实现 → 审计 FAIL → 修复 → 全新只读审计 PASS trace） |
| V1-06 模式、Hooks 与 CI | M2 | `manual` 保留安全门禁，`standard` 自动执行生产子集，`strict` 聚合发布条件；hook 只能显式安装并可卸载；无 hook/`--no-verify` 不能绕过 CLI/CI | 三模式正反例、hook 路径 diff、CI workflow 审计 | NOT RUN |
| V1-07 零构建分发入口 | M3 | 普通用户无需 clone/npm build；已有 Godot 时最多三个主要步骤；Plugin 可安装/加载/调用，Starter Template 可取得/初始化，CLI 为预构建产物 | 干净用户路径及步骤计数、包内容、实际调用 | NOT RUN（Plugin/Template bundle、archive 与 Apple Silicon 入口已有候选证据） |
| V1-08 生命周期与供应链 | M3 | 安装、升级、卸载、回滚保留用户项目和凭据；无默认遥测/隐藏网络或外部写入；发布物包含 checksum、来源和许可；npm 方案使用 Trusted Publishing/2FA/provenance | 生命周期前后 diff、网络/文件审计、artifact manifest、预发布配置审计 | NOT RUN |
| V1-09 macOS 生产证据 | M1/M3 | Godot 4.7.2 standard/GDScript 在 macOS Apple Silicon 完成干净环境全流程；生成 Universal 2 但只声明 Apple Silicon 技术验证；不要求签名/公证 | 干净环境完整 evidence 与 Apple Silicon target smoke | NOT RUN（Debug/Release 技术产物、双架构静态验证与 Apple Silicon target smoke 已 PASS；干净环境全流程未完成） |
| V1-10 Support Matrix 诚实性 | M3 | 版本、宿主和导出目标已冻结公开；每个生产级元组都有原生证据；交叉构建不冒充原生支持 | 版本化矩阵、宿主/目标 evidence 索引 | NOT RUN（Windows/Linux 原生范围在 M2 结束时决策） |
| V1-11 参考游戏与独立审计 | M3 | 参考游戏从初始化到导出完成 E2E；文档与行为一致；无 Blocker/High 安全问题或未解释严重性能回退；架构/安全/许可/发布只读终审通过 | 完整 trace、审计报告、问题清单与基线 | NOT RUN |
| V1-12 用户发布批准 | M3 | 用户在其余门禁全部通过后明确批准正式外部发布 | 可审计批准记录 | NOT RUN |

## 3. 原验收覆盖映射

三里程碑方案只收敛流程，不删除原始要求。2026-08-24 验收草案的细项映射如下：

| 原细项 | 顶层门禁 |
| --- | --- |
| G-001 至 G-004 | V1-01 |
| G-005 至 G-007 | V1-02 |
| G-008 至 G-009 | V1-03 |
| G-010 至 G-012 | V1-04 |
| A-001 至 A-007 | V1-05；其中内部 `AGENTS.md` 分发扫描同时属于 V1-07 |
| E-001 至 E-006 | V1-06 |
| D-001 至 D-005 | V1-07 |
| D-006 至 D-009 | V1-08 |
| R-002、R-005 | V1-09 |
| R-001、R-003、R-004 | V1-10 |
| R-006 至 R-009 | V1-11 |
| R-010 | V1-12 |

详细 fixture、平台组合和失败案例进入测试/evidence 清单，不再作为第二套发布审批表。

## 4. 当前延期项的验收语义

- Windows x64 与 Linux x64 原生 runner 当前 `NOT RUN`。到 M2 结束时决定补齐 Tier 1 证据或正式调整 Support Matrix；在此之前不能 PASS V1-10。
- npm 只在候选版本阶段做预发布配置审计；任何真实登录、组织创建、Token、publish 或 GitHub Release 都需要单独用户授权。
- Plugin 生命周期只在 M3 候选包稳定后集中演练一次；已有本地 bundle 不等于安装/升级/卸载已经通过。
- `clean` 实际删除、通用 schema migration、派生索引、raw 日志平台和第三方测试框架不是当前发布门禁，除非实现过程中出现没有它们就无法满足上述门禁的真实用例。

## 5. 明确不纳入 v1.0 验收

Unity、Godot .NET/C#、移动端/主机/Web 导出、Windows ARM、Linux ARM、macOS Intel 开发宿主和运行验证、Godot 游戏产物的 macOS 签名/公证与公开分发就绪验证，以及未冻结的 Godot 预发布/旧版本，除非用户通过新决策扩大 Support Matrix。
