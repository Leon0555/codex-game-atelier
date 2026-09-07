# M3 rc.3 绑定式发布证据

- 日期：2026-09-07（Asia/Shanghai）
- 候选版本：`0.3.0-rc.3`
- 源码提交：`89a80a88b47088373176afb3d1fda274ac875144`
- Marketplace ref：`marketplace/v0.3.0-rc.3`
- Marketplace revision：`40d8e90a297f43ab245c93cce4a894145869ca95`
- 绑定 evidence：[`evidence/m3-release-evidence-2026-09-07/release-evidence.json`](evidence/m3-release-evidence-2026-09-07/release-evidence.json)
- 当前结论：本地候选、远程 Plugin、用户级生命周期、Godot E2E、required CI 与 strict 聚合均已 PASS；随后独立最终审计发现统一宿主门禁缺失这一 High，因此 **rc.3 已拒绝**，不得晋升。独立 macOS 用户或第二台 Apple Silicon 机器复验、受保护版本 ref 和用户正式发布批准仍未完成。

## 1. 候选与绑定

`0.3.0-rc.3` 从受保护 `main` 的 clean source revision 构建两次。Plugin bundle A/B 与 distribution candidate A/B 均逐字节一致。

| 对象 | SHA-256 |
| --- | --- |
| `DISTRIBUTION-MANIFEST.json` | `f1dd108a879d5f1609d83364349056141ba25538c9a0a25c0b1a57fa09a9ab38` |
| Plugin archive | `a759c7c850b42501b157f924da616525df024830265e936b818abe56eeac050d` |

包内公开 CLI 和 private runner 均为 Universal 2；只有 Apple Silicon slice 完成原生执行验证。Windows/Linux 文件仍只是 artifact inventory，运行时明确报告为 v1 unsupported。

## 2. 远程 Plugin 与 Godot 闭环

从公开 GitHub Marketplace ref 安装到隔离 `CODEX_HOME` 后完成：

| 检查 | 结果 |
| --- | --- |
| Git-backed Marketplace 与 Plugin install | PASS |
| 安装 cache 与 bundle A 逐文件比较 | PASS；无差异 |
| public CLI `--version` | PASS；`0.3.0-rc.3` |
| private runner 直接调用拒绝 | PASS；退出 `125` |
| 特殊路径 embedded Starter 创建与初始化 | PASS |
| Godot 4.7.2 Headless validation | PASS；8/8 checks |
| 固定 GDScript 测试 | PASS；6/6 |
| standard Debug build 与 Release export | PASS；均完成 Apple Silicon target smoke |
| 下载属性与系统绕过 | 无 quarantine；未移除属性，未修改系统设置 |

隔离项目最初位于 `/private/tmp`，而 Godot 工具链位于外接卷；macOS 的 clonefile 同卷约束使该布局明确 BLOCKED。把全新 Starter 放到同一外接卷后，第一次非提权执行又被 Codex 工具沙箱阻止写入 Godot `user://logs` 和系统 CA 路径；项目自身日志已显示 PASS。随后仅放宽本次工具执行权限、不修改系统设置或 Gatekeeper，再次运行获得 Headless 8/8、GDScript 6/6、Debug build 和 Release export 全部 PASS。失败证据均保留。

Marketplace 获取期间还出现过一次 Git `curl 18` partial-file 失败；带 HTTP/1.1 的浅克隆随后约三秒完成，证明远程 ref 可取得。用户级候选重装时，一次 Codex CLI 调用在 `marketplace_add` 内部等待、尚未启动 Git；安全中断后相同命令重试成功。这些客户端/网络异常不改写为候选 PASS，但最终安装路径无阻断完成。

## 3. 用户级生命周期与恢复

操作前快照只记录权限、大小与 SHA-256，不输出配置正文。起始状态为 23 个已安装 Plugin、5 个 Marketplace、0 个 Atelier 条目；`config.toml` 权限 `0600`、大小 `5955`、SHA-256 `cbc4faaece632f785d8d3441216fbf2a39f85c493372217eaba9815f39bfe262`。

1. 远程安装 previous version `0.3.0-rc.2`，实际 CLI 与缓存均核对成功。
2. `rc.2 → rc.3` 升级 PASS；`rc.3` 为唯一 active 版本，缓存与候选逐文件一致。
3. 与正式 Marketplace 并存的独立无效 Marketplace 提供不可解析 `plugin.json`；安装明确退出 `1`，`rc.3` 始终保持 active。无效远程分支随后删除。
4. `rc.3 → rc.2` 回滚 PASS；实际 CLI 和已审计 rc.2 bundle 一致。
5. 重新安装 `rc.3`，用于新任务 Skill 发现；一次 Codex CLI 内部等待被中断，受控重试成功。
6. 显式卸载 Plugin 和 Marketplace，二者均退出 `0`。
7. Codex CLI 会重写 `config.toml`；用演练前私有快照恢复该文件后，mode、size、SHA-256 和逐字节比较全部与起点一致。只删除了本次卸载留下的精确空 Atelier cache 目录。
8. Marketplace 清单现场复核为起点 5 项、相同匿名哈希、0 个 Atelier；Atelier cache 和 Marketplace 目录均不存在。

恢复后的远程全局 Plugin catalog 查询返回 EOF，因此该次即时响应只列出 14 个可本地解析 Plugin，不能作为 23 项起点清单的重新枚举。生命周期没有写远程账号状态；机器本地的完整配置已逐字节恢复，且 Atelier 条目/目录为零。绑定 evidence 的前后状态保留起点清单哈希，并在本记录中明确披露 catalog 复核限制。

## 4. 新任务 Skill 发现

用户级 `rc.3` 激活后，新建只读任务 `01a079ed-cdda-7e61-8340-132bab015833`。该任务自动看见 `codex-game-atelier:develop-godot-game`，并定位和读取实际安装文件：

`/Users/leon/.codex/plugins/cache/codex-game-atelier/codex-game-atelier/0.3.0-rc.3/skills/develop-godot-game/SKILL.md`

任务能力清单的路径元数据仍显示旧 `rc.2`；该旧目录已不存在，磁盘定位得到的是唯一 `rc.3`。Skill 名称和入口相对路径发现 PASS，但这个 Codex 客户端元数据陈旧现象作为已知外部限制保留。任务没有执行 CLI/Godot 或修改文件，验证后已归档。

## 5. Required CI 与分支保护

GitHub Actions run [`33965337817`](https://github.com/Leon0555/codex-game-atelier/actions/runs/33965337817) 在候选源码提交上完成，event 为 `push`、branch 为 `main`，job `verify-macos-arm64` 结论为 `success`。

2026-09-07T03:37:47Z 的现场只读 API 复核确认：

- required status check 为 `verify-macos-arm64`，`strict=true`。
- `enforce_admins=true`。
- force push 与 branch deletion 均关闭。
- required signatures 关闭；它与不属于 v1 的 Apple 公证是不同机制。

## 6. Strict 聚合

输入为 fresh rc.3 特殊路径项目、distribution candidate A 与本记录对应的 1.1.0 evidence。实际命令退出 `0`：

| 结果 | 数量 |
| --- | ---: |
| PASS | 12 |
| BLOCKED | 0 |
| NOT_RUN | 0 |
| `release_ready` | `true` |

命令只读完成，输出只把 candidate/evidence 参数记录为 `provided`，没有回显绝对路径。完整回归同时通过：

- Draft 2020-12：27 个 schema、33 个 fixture、11 个持久化 Starter 记录、13 个持久化协作记录和 44 个负向断言。
- Python validators：52/52 PASS。
- Go：`gofmt` 无差异、`go vet ./...` PASS、`go test -count=1 ./...` PASS。
- Plugin bundle 与 distribution candidate 静态复验 PASS。
- 分发源码具体模型 ID、Unity 与旧产品名扫描均为零结果。

## 7. 尚未完成

- 独立架构、安全、许可与发布最终只读审计已完成，但因 1 High 拒绝 rc.3；详见 [`m3-rc3-final-readonly-audit-2026-09-07.md`](m3-rc3-final-readonly-audit-2026-09-07.md)。修复后必须构建并重新绑定 rc.4。
- 按用户决定，独立 macOS 用户或第二台 Apple Silicon 机器复验延后到最终 RC，当前为 `NOT RUN`。
- 受保护版本 ref/tag、正式 Plugin 发布和用户发布批准均未执行。
- Windows/Linux 原生验证、npm、standalone archive、DMG/PKG、Apple 签名/公证不属于 v1 发布路径。

## 8. 最终审计结果

审计计数为 Blocker 0 / High 1 / Medium 1 / Low 1。High 指向目标提交缺少全局 pre-dispatch host gate：`release check` 和 `hooks` 等命令在 Windows/Linux artifact 上仍可进入执行/写入路径，与 ADR 0025 冲突。Medium 是已披露的 Codex Skill 路径元数据仍指向 rc.2；Low 是缺少 `SECURITY.md`。因此 rc.3 状态为 `FAIL`，此前 strict 12/12 只是已验证事实，不等于候选可晋升。
