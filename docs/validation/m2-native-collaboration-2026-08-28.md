# M2 原生协作与文件化恢复验证

- 日期：2026-08-28
- 模式：Implementation → independent read-only audit → repair → fresh independent read-only audit
- 结论：PASS

## 实际工作流

本次不是提示词模拟。主任务先持久化 task/handoff，再启动一个没有继承对话历史的 Codex 原生实现代理。实现代理只从文件恢复任务，且只修改四个 ownership 路径。它停止写入后，另一个无对话继承的只读代理接收 task/handoff 并审计当前 diff。

第一轮审计真实返回 FAIL：Plugin 未分发恢复所需 Schema 闭包，且 ownership 路径规则没有覆盖目录、符号链接、大小写和 Unicode 别名。审计者未修复文件。ownership 随新的 task/handoff 返回原实现代理；实现代理完成三路径修复并再次停止写入。第三个全新只读代理从文件恢复第二轮审计，最终 PASS。

这条 trace 验证了：默认三代理上限、逐路径单一写 owner、实现者不自批、审计者不边修边批、失败 evidence 保留、所有权显式转移，以及不依赖隐藏对话状态的恢复。没有启动后台服务、隐藏规划器或自建模型路由。

## 产物与验证

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 初始恢复 | PASS | implementation agent 只读取 `task-implementation.json` 与 `handoff-to-implementation.json` 后完成限定变更 |
| 首轮独立审计 | FAIL（按设计保留） | [`audit-findings-1.md`](evidence/m2-native-collaboration-2026-08-28/audit-findings-1.md) 记录 P1/P2，审计零回写 |
| 有界修复 | PASS | [`implementation-repair.md`](evidence/m2-native-collaboration-2026-08-28/implementation-repair.md) 记录 18 项聚焦测试、Schema 验证、bundle build/native smoke 与摘要 |
| 第二轮独立审计 | PASS | [`audit-pass-2.md`](evidence/m2-native-collaboration-2026-08-28/audit-pass-2.md) 来自全新只读上下文，无 findings |
| 状态契约 | PASS | 13 份 task/handoff/evidence 快照均通过公共 Schema；三份 evidence payload 的 byte size 与 SHA-256 重新计算一致 |
| Bundle-only 恢复契约 | PASS | common/error/task/handoff/evidence 五份 Schema 固定打包；源码根全部不可用时仍能验证现有 bundle |
| ownership 负例 | PASS | 祖先/后代、符号链接、同对象、case-fold、Unicode NFC 碰撞均明确 `BLOCKED` |

本地候选 bundle：`.tools/plugin-bundles/codex-game-atelier-0.2.0-m2-native`（已忽略，不提交）。Bundle manifest SHA-256 为 `a35cc4ecaed8ab23cf4ab3d38975381942f84fbdc75c48844cac6af6909d6d53`，分发协作参考 SHA-256 为 `791ec689170e39a5732c516b2cd238b8838e88e531c39d214d84434746bb13e7`。

最终集成回归：Python Plugin/Template/Profile 共 32 项测试 PASS；Draft 2020-12 验证为 21 schemas、26 fixtures、11 份 M1 持久化记录、13 份协作 trace 记录及 31 个负例断言 PASS；Go `go test -count=1 ./...` 与 `go vet ./...` PASS。

## 未覆盖范围

- 路径冲突当前是分发给 Codex 原生代理的规范与打包回归，不是 CLI 内的 APFS/NTFS 可执行 ownership 引擎；v1 不建设通用任务数据库或常驻锁服务。
- 本切片未做真实 Plugin 安装生命周期，该项集中在 M3。
- Windows/Linux 原生运行仍未验证；这里只验证跨编译文件继续进入 bundle，不改变支持声明。
