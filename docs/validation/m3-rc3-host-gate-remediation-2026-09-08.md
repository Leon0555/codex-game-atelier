# rc.3 宿主门禁 High 修复

- 日期：2026-09-08（Asia/Shanghai）
- 触发：[`m3-rc3-final-readonly-audit-2026-09-07.md`](m3-rc3-final-readonly-audit-2026-09-07.md) 的 1 High
- 范围：公共 CLI 顶层分发、非支持宿主无写入回归、契约与状态文档
- 候选语义：这是 rc.4 源码修复，不修改或重新批准 rc.3 artifact

## 实现

`app.Run` 在全部 13 个公开命名命令的专属参数解析、项目读取、Godot 调用或写入之前调用统一宿主门禁。唯一支持组合为 `darwin/arm64`；其他组合固定输出一个结构化 `BLOCKED` result、错误 `HOST_UNSUPPORTED`、退出码 `4`。

未知命令和未声明别名继续返回 `INVALID_ARGUMENT`/2，不会因为宿主门禁被误认为真实命令。只有精确单参数 `--version` 可绕过命名命令门禁；它只向 stdout 写版本标识，不读取项目、运行工作流或写入状态。

## 回归

- 逐个覆盖 `build`、`clean`、`detect`、`doctor`、`export`、`hooks`、`initialize`、`logs`、`release`、`starter`、`status`、`test`、`validate`，模拟 Linux x64 并验证统一拒绝。
- 比较所有命令前后的临时文件树，确认无写入。
- 模拟 Windows x64，验证 `hooks`/`release`/`starter` 的畸形参数仍先被宿主门禁拒绝。
- 验证 `rel`、`Build`/unknown 类未声明入口仍为 usage error；精确 `--version` 成功，`--version extra` 为 usage error。
- `gofmt`、`go vet ./...`、定向测试与 `go test -count=1 ./...` PASS。
- Draft 2020-12 schema 全量、52 项 Python validators、Plugin policy/Skill 打包回归 PASS。
- Linux/Windows 新源码交叉构建成功；只验证 artifact 形状，不冒充原生执行。

## 文档与安全入口

ADR 0025、公共 contract、CLI README 与分发 Skill 已统一说明全局门禁和 `--version` 的非操作性例外。rc.3 的 strict PASS、最终审计 FAIL 与 rc.4 重建要求分别保留，没有回写历史证据。

仓库新增 `SECURITY.md`，明确当前没有已发布/受支持版本，并禁止把敏感漏洞细节放入公开 Issue。GitHub Private Vulnerability Reporting 的只读检查结果为 disabled；启用仍是正式发布前需维护者批准的远程设置动作，本修复没有暗中开启。

## 后续

修复必须通过受保护 PR 和 required CI，合并到新的 clean `main` 后再构建 rc.4。rc.4 仍需重新执行 candidate 绑定、远程 Plugin、Godot E2E、用户级生命周期、strict 聚合和最终独立审计。

## 独立修复复审

同一只读审计责任在新的复审轮中检查了工作区修复，没有参与修改。首轮复审报告两个 Low：缺少畸形/别名/`--version` 路由测试，以及 CLI README 保留旧 initialize 错误码。实现 owner 补齐后再次 spot-check，最终结果为 **Blocker 0 / High 0 / Medium 0 / Low 0**；针对性 race test、格式和 diff 检查均 PASS。该结论只批准修复进入 rc.4 源码 PR，不批准尚未生成的 rc.4 artifact。
