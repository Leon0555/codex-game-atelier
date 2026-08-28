# M2 逻辑能力 Profile 验证

- 日期：2026-08-28
- 模式：Implementation + 本地验证
- 结论：PASS（仅 Profile 子项；V1-05 的原生协作恢复演练仍未运行）

## 实现边界

本切片在 Plugin Skill 内加入四个逻辑 Profile：`lead`、`implementation`、`fast-read`、`independent-audit`。目录只表达能力等级、工作类别、读写边界、独立性和降级策略，不含具体模型 ID。绑定顺序固定为任务显式覆盖、用户/团队映射、会话继承、主机默认；用户映射明确排除在 Plugin 分发之外，CLI 不参与选模。

Codex 当前支持用户级或项目级自定义 Agent 配置及会话继承，但 Plugin manifest 的正式组件边界是 Skills、MCP、hooks 和相关资源，并没有可移植的 Agent 映射字段。因此本项目不伪造 manifest 能力，也不自动写用户配置。参考：[Codex subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)、[Build plugins](https://developers.openai.com/plugins/build/plugins)。

## 验证结果

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| Profile Schema/fixture | PASS | Draft 2020-12：21 schemas、26 fixtures；分发 catalog 与 contract fixture 语义一致 |
| 解析优先级 | PASS | 9 项矩阵覆盖任务覆盖、用户映射、会话继承、主机默认、未知 Profile 与无绑定 |
| 审计不可静默降级 | PASS | critical 能力不足或没有独立只读上下文均稳定返回 `BLOCKED` |
| 分发模型中立 | PASS | 扫描 Plugin、Starter Template、参考游戏和 CLI runtime 源码；无具体模型 ID |
| task/handoff 记录 | PASS | `owner.logical_profile` 为向后兼容可选字段，既有省略形式仍有效 |
| Plugin 固定白名单 | PASS | bundle 必须包含 catalog；目录/字段/语义篡改均被拒绝 |
| Apple Silicon 原生入口 | PASS | 新 bundle 结构验证后，public CLI 与 sibling runner smoke 均通过 |

关键本地产物（`.tools/` 已忽略，不作为源码提交）：

- bundle：`.tools/plugin-bundles/codex-game-atelier-0.2.0-m2-profiles`
- catalog SHA-256：`5a37ebe7143d37608c80e389b8dae3c513a8dea3391a367559667338200f6a9e`
- bundle manifest SHA-256：`73232e6c5742c773fb7746d63f5763011dd43c8e628dc192f9e5a8c01dfc063c`

## 未完成

- 尚未进行一次真实 Codex 子代理的 owner → read-only audit → handoff → 恢复 trace；该项属于 M2 下一切片。
- 尚未写入或演练任何用户级自定义 Agent 映射；这不是 Plugin 安装副作用，将来只作为用户显式选择的集成方式。
- V1-05 因恢复 trace 未完成，当前只能标记 `PARTIAL`，不能整体 PASS。
