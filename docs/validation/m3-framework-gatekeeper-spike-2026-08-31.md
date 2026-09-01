# M3 Framework quarantine / Gatekeeper Spike

> 后续决策（2026-08-31）：ADR 0023 已把本记录中的手动 archive/quarantine 路径排除出 v1 支持渠道。Apple 公证不再是默认门禁；当前门禁是干净 Apple Silicon 上真实远程 Plugin 的无阻断安装。以下内容保留为历史失败 evidence，不代表当前实施计划要求预先签名或公证。

- 日期：2026-08-31
- 候选源码：`de3f7420d346222a515b3ffc07b9a76b01807fe2`
- 记录基线：`4f14093`
- 候选：`.tools/distributions/codex-game-atelier-0.2.0-m3-strict-de3f742-a/`
- 宿主：macOS Apple Silicon
- 范围：仅在项目 `.tools/` 可丢弃副本上设置/移除 quarantine 并执行只读 Gatekeeper 检查；未修改候选、系统策略、Keychain、证书、用户级 Codex 或远程状态
- 结论：当前 framework CLI/runner 在可信本地路径可运行，但当前 archive 经带 quarantine 的下载等价路径解包后不能无人值守运行；SC-003 仍为发布 High，不能把本地 Plugin 安装 PASS 推广成公开下载 PASS

本 Spike 没有做 Developer ID 签名、公证或账号操作，也没有把 ad-hoc 签名实验写入生产打包链。Godot 游戏技术导出免签决定与本结论无关。

## 1. 当前候选基线

候选 Plugin 内 `darwin-universal2` public CLI 与 private runner 都是实际 `x86_64 + arm64` Mach-O。public CLI 的当前观察：

| 检查 | 结果 |
| --- | --- |
| 原候选路径 `--version` | PASS；`codex-game-atelier 0.2.0` |
| `codesign -dvv` | 显示 `Identifier=a.out`、`adhoc,linker-signed`、无 TeamIdentifier |
| `codesign --verify --strict --verbose=4` | FAIL/1；`code object is not signed at all` |
| `spctl --assess --type execute --verbose=4` | FAIL/1；Code Signing subsystem internal error |
| 原候选 xattr | 只有本地 provenance，无 quarantine |

这说明 Go 薄 slice 的 linker signature 观察不等于 `lipo` 后 Universal 2 文件拥有可验证的整体发布签名。

## 2. 显式 quarantine 二进制

将 public CLI 与 private runner 复制到唯一临时目录并分别设置 `com.apple.quarantine` 后：

- strict codesign 仍失败；
- `spctl` 仍退出 1；
- public CLI `--version` 与 private runner 均超过 30 秒没有返回，不适合作为非交互 Plugin/CI 路径；
- 只终止本 Spike 启动的精确 PID `18881`、`18930`，随后 `pgrep` 确认没有残留。

本记录把“无返回”当作失败，不把潜在系统提示或用户手工放行视为自动安装成功。

## 3. Quarantined archive 解包

复制 Plugin archive 后只在副本上设置 quarantine，再用 macOS `/usr/bin/tar` 解包：

- archive 保留 quarantine；
- 解包后的 Plugin root、子目录、public/private binary 和文字文件都获得 quarantine；
- public CLI 的 `spctl` 退出 1；
- 用 5 秒 hard alarm 执行 `--version`，以 signal/alarm 退出 `142`，无版本输出。

因此不能假设“quarantine 只在压缩包上，不会传播到 CLI”。不同 Plugin/包管理器的真实解包行为仍需在对应远程渠道复验，但至少系统 tar 路径已给出明确失败反例。

尝试只对临时解包树移除 quarantine 后，同一路径仍受既有系统 assessment 影响而超时；本 Spike 不把 `xattr -d` 宣传成可靠修复。另一个从可信本地 candidate 新建、从未带 quarantine 的普通副本可在 5 秒内返回 `0.2.0`，证明二进制功能本身没有损坏。

## 4. 系统设置人工允许复验

用户随后明确在 macOS 系统设置中允许了被阻断项目。本 Spike 对同一 quarantined 路径再次执行：

| 检查 | 结果 |
| --- | --- |
| public CLI `--version`，10 秒 hard alarm | FAIL；退出 `142`，无版本输出 |
| private runner，10 秒 hard alarm | FAIL；退出 `142` |
| public CLI `spctl` | FAIL/1；Code Signing subsystem internal error |
| private runner `spctl` | FAIL/1；Code Signing subsystem internal error |
| 两个文件 quarantine xattr | 均仍存在 |

这不证明用户操作错误；macOS 的人工放行可能仍要求后续图形确认，而且 public/private 是两个独立 executable。它证明的是：该路径不能作为无提示的 Plugin、CLI 或 CI 安装闭环，也不能要求普通用户逐个放行包内隐藏二进制。

## 5. 结论与产品影响

1. **本地生命周期证据继续有效但范围有限**：专用本地 marketplace 安装、发现、调用和卸载 PASS，证明本地可信来源闭环，不证明下载场景。
2. **当前 public archive 不能声明 macOS 公开分发就绪**：完整性、clean provenance 和 checksum 都不能替代发布者身份与 Gatekeeper acceptance。
3. **不能自动清除 quarantine**：分发器或 Plugin 自行运行 `xattr -d` 会修改 macOS 安全元数据，既不应隐藏执行，也不能作为三步上手的默认可靠方案。
4. **需要冻结 framework 分发策略**：最终候选必须选择并实证 Developer ID/notarization 路径，或把 v1 安装渠道限制为已证明不会留下 quarantine 阻断的受控 Plugin/package-manager 获取路径；后者仍需真实远程渠道证据，不能由本地 tar 试验推定。

## 6. 未完成

- 未使用 Developer ID、Apple 公证、证书或账号。
- 未修改生产打包器，未建立整体签名或 notarization workflow。
- 未通过真实浏览器/GitHub/npm/Codex Marketplace 下载。
- 未验证 Codex 远程 Plugin 安装器是否传播、移除或绕过 quarantine。
- 未解决 Windows/Linux Tier 1、required hosted CI、升级/回滚或最终独立审计。
