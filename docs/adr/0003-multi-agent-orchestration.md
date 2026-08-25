# ADR 0003：Codex 原生有界子代理编排

- 状态：Accepted（2026-08-24 Phase 0 审阅通过）
- 日期：2026-08-24
- 决策范围：智能协作、任务所有权、评审与恢复

## 背景

把 `run <role>` 实现为 CLI 中的 prompt packet 组装/外部单轮执行，无法充分利用当前 Codex 原生子代理线程、并行状态和可检查活动。CLI 也不适合成为常驻智能协调服务。项目需要真正的有界子代理，同时保证所有权、证据、恢复和用户权限边界可审计。

## 决策

1. 智能编排由当前 Codex 会话的原生子代理能力承担；CLI 不实现模型循环、prompt runner 或后台 daemon。
2. 默认最多同时运行三个子代理，只并行具有独立输入/输出和不重叠写范围的工作。
3. 每个跨文件/跨模块任务只有一个 owner；其他代理只能做只读探索、测试或评审，除非 Lead 显式移交所有权。
4. 核心责任角色保持精简：Lead、Godot Engineer、QA/Recovery Engineer、Read-only Auditor。
5. 架构、安全、兼容性和发布审计默认只读。Auditor 发现问题后由 owner 修复，再启动新的独立评审；不在同一步自修自批。
6. 任务与交接落盘，候选字段包括：task id、scope、acceptance、owner、allowed paths、status、dependencies、evidence refs、handoff summary、blocker 和 schema version。
7. 状态机候选：`planned → ready → active → review → verified → done`，并允许 `blocked`、`failed`、`cancelled`。状态变化由确定性 CLI/schema 校验，判断和任务拆分由 Codex 完成。
8. 子代理只返回结论、必要证据引用和待决问题；长日志保存在 evidence，不灌入主线程。
9. 中断后新会话读取 task/handoff/evidence 恢复。隐藏对话记忆不是唯一事实来源。
10. 外部发布、破坏性操作、安装、账号或范围扩张仍由用户批准；子代理不能继承超出主任务的权限。

## 备选方案

### A. CLI `run <role>` 生成并执行单轮 prompt

拒绝作为核心编排。可保留只读 prompt/plan 预览思想，但不能替代原生子代理线程、所有权和交接。

### B. 常驻后台多代理服务

拒绝。增加部署、状态一致性、隐私、恢复和安全面，与 Codex 原生能力重复。

### C. 不落盘，只依赖会话

拒绝。会话中断、压缩或换主机后无法可靠恢复，也难做发布审计。

### D. 大规模固定角色矩阵

拒绝。触发重叠、维护成本和上手门槛高；角色按真实 Godot 垂直闭环再拆分。

## 理由

Codex 官方当前支持并行子代理、线程可检查和结果聚合。把智能留给宿主可避免重造 agent runtime；文件化状态和确定性 CLI 则提供跨会话恢复与审计。

## 风险

- 不同 Codex 客户端/版本的子代理和自定义 Agent 能力可能不同。
- 多代理会增加 token/时间成本；并行只用于真实独立工作。
- 文件锁/owner 与真实进程状态可能漂移，需要 lease、超时和人工恢复语义。
- 过度结构化任务会重现高门槛问题，必须保持最小必需字段。

## 迁移与回退

- 无子代理能力时允许 Lead 串行执行同一任务状态机，并明确记录 `degraded: serial`；不回退到隐藏后台服务。
- 任务 schema 版本化并支持只读旧版本、显式迁移和备份。
- 失效 lease 只能通过可审计 recovery 命令释放，不能静默抢占活跃 owner。

## 验证

- 三个独立只读任务并行并由 Lead 汇总。
- 两个写任务尝试声明同一路径时被阻止。
- Auditor 只读权限负例和修复后重新评审。
- 主进程中断后从文件化 handoff/evidence 恢复。
- 串行降级路径在无子代理能力时保持同一验收语义。

## 官方依据

- [OpenAI 官方：Subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)
