# 来源与借鉴记录

状态：Phase 0 来源审计已通过；Phase 1 持续更新
审计日期：2026-08-24

## 1. 记录原则

- 优先 clean-room 风格的架构借鉴：研究思想、能力边界和缺陷，不默认复制源码、提示词、脚本、模板、资源或大段文档。
- 每次实质性复用必须记录仓库、commit、源文件、版权、许可证、复制/改写方式和进入的本项目文件。
- 若包含第三方源码或实质部分，必须在分发物中保留相应版权和许可证，并更新 NOTICE。
- 本记录是工程来源审计，不替代法律意见。

## 2. merlinhu1/codex-game-studio

- 仓库：[merlinhu1/codex-game-studio](https://github.com/merlinhu1/codex-game-studio)
- 审计 commit：[`f3c50a0552f562eafc8178973dc48c29cd0d50fa`](https://github.com/merlinhu1/codex-game-studio/tree/f3c50a0552f562eafc8178973dc48c29cd0d50fa)
- 许可证：[原始 LICENSE](https://raw.githubusercontent.com/merlinhu1/codex-game-studio/f3c50a0552f562eafc8178973dc48c29cd0d50fa/LICENSE)，MIT，`Copyright (c) 2026 MerlinH`。
- Phase 0 复制状态：**未复制**源码、提示词、模板、脚本、资源或实质性文档。

### 研究过的思想

- 文件可审查的状态、审批、锁和运行元数据。
- 确定性 prompt/run 输入与 dry-run 可见性。
- CLI 验证和角色化协作边界。

### 独立重设方式

- 本项目把智能编排交给 Codex 原生子代理，CLI 只保存确定性状态/证据并调用 Godot。
- 采用逻辑 Profile 和用户/会话解析，不复制模型白名单或回退表。
- 从 Godot-only 的最小闭环起步，不复制其大规模 Agents/Skills/Workflows 矩阵。
- 本项目自行定义门禁、schema、错误码和证据格式。

### 核验到的局限与差异

- [`src/prompt-surface-metadata.ts`](https://github.com/merlinhu1/codex-game-studio/blob/f3c50a0552f562eafc8178973dc48c29cd0d50fa/src/prompt-surface-metadata.ts) 存在具体模型白名单/回退映射。
- [README](https://github.com/merlinhu1/codex-game-studio/blob/f3c50a0552f562eafc8178973dc48c29cd0d50fa/README.md) 的初始化要求 Node ≥24、Codex CLI，并描述大规模角色/技能/工作流面。
- 对“`run <role>` 只是提示词组装”需精确表述：README 把它描述为 bounded prompt packet 组装，`--dry-run/--print-prompt` 可以只查看；默认路径还会通过 [`src/runner.ts`](https://github.com/merlinhu1/codex-game-studio/blob/f3c50a0552f562eafc8178973dc48c29cd0d50fa/src/runner.ts) 与 [`src/codex-runtime.ts`](https://github.com/merlinhu1/codex-game-studio/blob/f3c50a0552f562eafc8178973dc48c29cd0d50fa/src/codex-runtime.ts) 启动 Codex 并保存 run metadata。因此局限不是“完全不执行”，而是以外部单轮 prompt/run 为核心，没有采用本项目所要求的原生可检查子代理线程、文件化所有权/交接和独立只读审计模型。

## 3. Donchitos/Claude-Code-Game-Studios

- 仓库：[Donchitos/Claude-Code-Game-Studios](https://github.com/Donchitos/Claude-Code-Game-Studios)
- 审计 commit：[`984023ddac0d5e27624f2baacde6105e45de375f`](https://github.com/Donchitos/Claude-Code-Game-Studios/tree/984023ddac0d5e27624f2baacde6105e45de375f)
- 许可证：[原始 LICENSE](https://raw.githubusercontent.com/Donchitos/Claude-Code-Game-Studios/984023ddac0d5e27624f2baacde6105e45de375f/LICENSE)，MIT，`Copyright (c) 2026 Donchitos`。
- Phase 0 复制状态：**未复制**源码、提示词、模板、hooks、脚本、资源或实质性文档。

### 研究过的思想

- 角色职责、升级路径和可选 review 强度。
- 路径规则、生命周期检查和用户作最终选择的人类控制模型。

### 独立重设方式

- 将关键检查实现为 CLI 命令内建门禁和发布 CI，而非复制 Claude hooks。
- Git hooks 始终显式可选、可列出、可卸载，不自动写入。
- 只保留少量由 Godot 垂直闭环证明必要的 Agents/Skills。

### 核验到的局限与差异

- [协调规则](https://github.com/Donchitos/Claude-Code-Game-Studios/blob/984023ddac0d5e27624f2baacde6105e45de375f/.claude/docs/coordination-rules.md) 绑定具体 Claude 模型/等级。
- [README 平台说明](https://github.com/Donchitos/Claude-Code-Game-Studios/blob/984023ddac0d5e27624f2baacde6105e45de375f/README.md#platform-support) 表明主要在 Windows 10 + Git Bash 测试，macOS/Linux 尚在测试。
- Hooks 的强制程度不一致：[`settings.json`](https://github.com/Donchitos/Claude-Code-Game-Studios/blob/984023ddac0d5e27624f2baacde6105e45de375f/.claude/settings.json) 配置生命周期 hooks；[`validate-commit.sh`](https://github.com/Donchitos/Claude-Code-Game-Studios/blob/984023ddac0d5e27624f2baacde6105e45de375f/.claude/hooks/validate-commit.sh) 的部分检查告警后返回成功，而 [`validate-assets.sh`](https://github.com/Donchitos/Claude-Code-Game-Studios/blob/984023ddac0d5e27624f2baacde6105e45de375f/.claude/hooks/validate-assets.sh) 对部分无效数据失败。

## 4. NOTICE 处理

当前 Phase 0 只有思想研究，没有复制两仓库的软件或实质部分，因此 MIT 的“随复制保留版权/许可”条件尚未触发到本项目分发内容；NOTICE 不列入 MerlinH 或 Donchitos 的版权，以免虚构包含关系。

若未来复用任何可识别文件、大段提示/模板、脚本、资源或实质性改写：

1. 在本文件新增逐项记录。
2. 在复制内容附近及/或分发 NOTICE 保留对应版权与完整 MIT 条款。
3. 重新做逐文件许可证审计后再发布。

## 5. Phase 1 本地生成工具

| 产物 | 工具 | 复用情况 | 处理 |
| --- | --- | --- | --- |
| `plugin/codex-game-atelier/.codex-plugin/plugin.json` | Codex bundled `plugin-creator` scaffold | 仅使用清单形状和校验默认值；产品文案、作者与能力已重写 | 不复制工具脚本，不把开发 Skill 打入分发包 |
| `plugin/codex-game-atelier/skills/develop-godot-game/` | Codex bundled `skill-creator` initializer | 仅使用目录/frontmatter/UI metadata 骨架；运行说明由本项目原创 | 未创建用户级 Skill 或 Marketplace 条目 |

这些工具属于当前 Codex 开发环境，不是两个参考游戏工作室仓库，也不成为用户运行时依赖。
