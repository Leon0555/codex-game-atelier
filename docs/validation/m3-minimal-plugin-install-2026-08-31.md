# M3 最小真实 Codex Plugin 安装闭环

- 日期：2026-08-31
- Codex CLI：`0.151.0-alpha.7.1`
- Plugin：`codex-game-atelier@codex-game-atelier-local`，版本 `0.2.0`
- 宿主：macOS Apple Silicon
- 结论：最小安装 → 新任务发现 → 包内 CLI 调用 → 卸载闭环 `PASS`；升级、失败升级与回滚 `NOT RUN`

## 授权与写入边界

用户明确批准“最小真实安装闭环”。允许的用户级写入仅包括专用本地 marketplace `codex-game-atelier-local`、测试 Plugin selector 及其 Codex cache。没有修改 personal marketplace、其他 Plugin、凭据、系统配置、远程仓库或外部发布状态；没有运行 Godot、初始化用户游戏项目或联网。

创建了一个 projectless 只读测试任务 `01a05577-f97e-7963-9129-e22d01bae3d4`，验证结束后已归档。任务没有创建/修改文件，也没有运行初始化、构建或导出。

## 实际结果

| 步骤 | 结果 | 关键证据 |
| --- | --- | --- |
| 起始快照 | PASS | 仓库 clean；Atelier config/cache 无匹配；用户配置 SHA-256 `10d7404e8fbfd4355451998feaa6b664dfc9781555cb3eb91fbd37b860755901` |
| marketplace 注册 | PASS | `marketplace add <A-path> --json` 返回 `codex-game-atelier-local`，`alreadyAdded=false` |
| Plugin 安装 | PASS | `plugin add codex-game-atelier@codex-game-atelier-local --json` 返回版本 `0.2.0` 和预期 cache 路径 |
| 安装态结构 | PASS | Atelier bundle verifier 与 Plugin Creator `validate_plugin.py` 均退出 0；安装缓存与候选 Plugin `diff -rq` 无差异 |
| 本机入口 | PASS | 包内 `codex-game-atelier --version` 退出 0，输出 `codex-game-atelier 0.2.0` |
| private runner 边界 | PASS | 直接调用退出 125，固定说明其为 internal fd-only component |
| 新任务 Skill 发现 | PASS | 全新任务自动发现 `skills/develop-godot-game/SKILL.md`，依据 Skill 定位包内 macOS CLI；`--version` 退出 0 |
| 卸载与移除 | PASS | `plugin remove` 与 `plugin marketplace remove` JSON 均只指向测试 selector；专用空 cache 根随后用 `rmdir` 删除 |
| 非测试 Plugin/marketplace | PASS | 前后均为原有 13 个已安装 Plugin、5 个 marketplace；名称、版本、来源和启用状态未变化 |
| 仓库状态 | PASS | 生命周期结束后仍无工作区改动；本记录写入前验证 |

记录写入后的仓库回归同样通过：项目本地 Python 环境执行 `python3 -m unittest discover -s tools/validators -p 'test_*.py' -v` 为 48/48 PASS；项目本地 Go 1.27.0 在 `packages/cli` 执行 `go test ./...` 退出 0。系统 `PATH` 不含 Go，因此测试明确使用已批准的项目本地工具链和项目内 cache，没有安装或修改全局 Go。

## 恢复差异

Atelier Plugin、marketplace 配置项和 cache 内容均已消失，其他 Plugin/marketplace 的语义状态与起始清单一致。Codex CLI 在 add/remove 过程中重写了用户级 `config.toml`：结束 SHA-256 为 `9ffd1af7937f3e7debd02125d730c86cddf6e3116755ca6117c59a71dc86c98b`，与起始摘要不同。CLI 没有留下可供字节级恢复的备份，因此本轮不手工重写整份用户配置，也不声称逐字节还原；只声明测试条目已完全移除、非测试 Plugin/marketplace 清单未变。

Codex CLI 两次列举时提示无法在当前受限进程中创建 PATH aliases，但命令继续成功并返回完整 JSON；该提示没有改变本次 Plugin 安装、发现或卸载结果。

## 未覆盖范围

- marketplace B 成功升级。
- 无效/失败升级保持 active version 的负例。
- 卸载后回装上一版本的回滚。
- 独立干净 macOS 用户账户或隔离 `CODEX_HOME`。
- 动态网络审计、外部 marketplace/Git 获取、npm 或 GitHub 发布。
- Starter Template 初始化与 Godot 工作流；这些不是本次用户级最小闭环的一部分。

以上项目继续保持 `NOT RUN`，留到最终候选的完整生命周期/发布审计，不影响本轮“Plugin 能被当前 Codex 真实安装、由新任务发现并安全卸载”的结论。
