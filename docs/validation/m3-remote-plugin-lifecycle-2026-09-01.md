# M3 远程 Plugin、Hosted CI 与真实生命周期验证

- 日期：2026-09-01
- 宿主：macOS Apple Silicon
- Codex CLI：`0.151.0-alpha.7.2`
- 源码仓库：`https://github.com/Leon0555/codex-game-atelier`
- 远程候选：`marketplace/v0.3.0-rc.1` @ `8bc58d3`
- 生命周期回滚基线：`marketplace/v0.3.0-rc.0` @ `dc91f5f`
- Hosted CI：`main` @ `930da52`，run `33515728377`

## 1. 结论

1. 从 GitHub Git-backed Codex Marketplace 向新的隔离 `CODEX_HOME` 安装 `0.3.0-rc.1`：PASS。
2. 安装缓存与已审计本地 Plugin bundle 逐文件一致，结构验证与 public CLI 实际执行：PASS。
3. macOS 安装缓存中的 CLI 只观察到 `com.apple.provenance`，没有 `com.apple.quarantine`；未执行 `xattr` 清除、系统设置放行、签名或公证。public CLI 实际返回 `0.3.0-rc.1`，private runner 按契约拒绝直接调用并返回 `125`。
4. 远程 Plugin 内 CLI 在中文、空格和 `#` 路径完成 `starter create → initialize → validate --headless → test`：PASS，GDScript `6/6`。
5. GitHub-hosted `macos-15` Apple Silicon CI：PASS；宿主架构断言、Go 1.24 最低工具链、Go format/vet/test、Schema/分发测试、本机 CLI/runner smoke 全部通过。
6. 真实用户级 `~/.codex` 生命周期：`rc.0` 安装 → `rc.1` 升级 → 无效候选拒绝且 active 仍为 `rc.1` → 回滚 `rc.0` → 卸载/清理：PASS。
7. 演练后用户级语义状态和 `config.toml` 字节级恢复到起点：PASS。

这些结论不等于 v1.0 已发布；新任务从远程安装态发现 Skill、branch protection 的 required check、受保护发布 tag/attestation、全新用户或机器复验与最终独立只读审计仍未完成。

## 2. 远程隔离安装

使用全新临时 `CODEX_HOME`，执行：

```text
codex plugin marketplace add Leon0555/codex-game-atelier --ref marketplace/v0.3.0-rc.1 --json
codex plugin add codex-game-atelier@codex-game-atelier --json
```

Codex 报告 Plugin `0.3.0-rc.1`，marketplace source 为 GitHub HTTPS Git URL。安装后的 cache 通过 `tools/package_plugin.py verify`，并与 `.tools/plugin-bundles/plugin-only-0.3.0-rc.1-969bef0` 逐文件比较无差异。

第一次 Godot Headless 尝试把临时项目放在系统盘 `/private/tmp`，而 Godot 在项目外置盘，按 ADR 0009 的同卷 APFS clone 要求正确返回 `GODOT_EXECUTABLE_SNAPSHOT_UNAVAILABLE`/4。保留该 BLOCKED 证据后，将临时 Starter 改到与 Godot 同卷的已忽略 `.tools` 路径：Headless validation 在 17.6 s 内 PASS，固定 GDScript 测试在 12.9 s 内 `6/6` PASS，engine version 为 `4.7.2.stable.official.ed1daf0bf`。

完成后从隔离 `CODEX_HOME` 卸载 Plugin 和 Marketplace，隔离已安装清单恢复为空。

## 3. Hosted CI

首次 run `33475137601` 在 Go 1.24 测试中失败。根因是一个测试把嵌套 `os.Root.Name()` 当成绝对路径；Go 1.24 返回 `.gameatelier`，导致测试从错误工作目录读取，并非生产 no-replace 逻辑删除了文件。

修复 `930da52` 让断言也通过已钉住的 `os.Root` 读取。本地 Go 全量测试、`go vet`、26 个 Schema/fixture 闭包与 52 个 Python 测试通过后推送。第二次 run `33515728377` 的全部 steps PASS：

- `Assert Apple Silicon host`
- `Set up minimum Go toolchain` (`1.24.0`)
- Go format/vet/test
- unsupported artifact-only Linux/Windows cross-build
- Schema and distribution tests
- native CLI/private runner build and smoke

可审计链接：`https://github.com/Leon0555/codex-game-atelier/actions/runs/33515728377`。该 workflow 已托管运行 PASS，但尚未配置 branch protection required check。

## 4. 真实用户级生命周期

操作前：

- 已安装 Plugin：13；Atelier：0。
- Marketplace：5；Atelier：0。
- Atelier Plugin cache 与 Marketplace snapshot：不存在。
- `config.toml` 私有快照权限 `0600`，SHA-256 为 `6a15ef7294c6b29b0cd57c1b882a2de3bb7c32d17f025f08a3d33d2c10f8b837`；没有输出正文。

正例顺序：

1. 从 `marketplace/v0.3.0-rc.0` 安装 `rc.0`，结构验证和实际 `--version` PASS。
2. 根据当前 Codex CLI 的实际契约，用 `marketplace remove → add(new ref) → plugin add` 升级到 `rc.1`；隔离 `CODEX_HOME` 先验证了该序列。
3. `rc.1` cache 与已审计候选逐文件一致，实际 CLI 返回 `rc.1`。
4. 安装一个不可解析 `plugin.json` 的独立远程升级候选：Marketplace clone PASS，Plugin install 明确返回 1；原 `rc.1` 始终为唯一 active Atelier 版本。
5. 移除无效测试 Marketplace，并删除远程临时分支 `test/invalid-upgrade-candidate`；本地完整 commit `e309ea0` 保留可恢复。
6. 切回 `marketplace/v0.3.0-rc.0`，实际 cache 逐文件比较和 CLI `--version` 证明回滚 PASS。
7. 显式卸载 Atelier Plugin 与 Marketplace。

负例还暴露了一个不能隐藏的客户端行为：仅删除 manifest 的 `version` 时，Codex 会以 `local` 版本安装，而不是拒绝。Atelier 自己的 bundle verifier 会拒绝该包；该测试条目已立即精确卸载，不作为失败升级 PASS 证据。随后使用不可解析 JSON 完成真正的拒绝路径。这说明发布门禁不能仅依赖 Codex 安装器，仍必须保留 Atelier 包完整性验证。

操作后：

- 已安装 Plugin 恢复为原 13 个，ID 清单与起点一致；Atelier：0。
- Marketplace 恢复为原 5 个；Atelier：0。
- `config.toml` SHA-256 与起点快照完全一致，未用快照覆盖用户文件。
- Codex 卸载后留下的两个 Atelier 空 cache 目录已用非递归 `rmdir` 精确移除；两个路径均已证实不存在。
- 完成一致性核对后，临时私有 `config.toml` 快照及其专用目录已精确删除；快照没有进入仓库或其他持久化产物。

## 5. 仍未覆盖

- 从远程 Plugin 安装态新建 Codex 任务并由主机自动发现 `develop-godot-game` Skill。
- 全新 macOS 用户或另一台 Apple Silicon 机器的独立复验。
- GitHub branch protection 将 hosted CI 设为 required check。
- 受保护版本 tag、远程 attestation/package provenance 和正式 Plugin 提交。
- 最终架构、安全、许可证和发布独立只读审计。
