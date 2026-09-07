# M3 rc.4 Plugin manifest 实测与修复

- 日期：2026-09-08（Asia/Shanghai）
- 候选版本：`0.3.0-rc.4`
- 源码提交：`f43ec6370446879b226d0eaa05ce62935c1fd369`
- Marketplace ref：`marketplace/v0.3.0-rc.4`
- Marketplace revision：`ddd88322e1043e299d95891f8988b742dfcb1428`
- 结论：**rc.4 拒绝；修复后重建 rc.5**

## 已通过证据

- 两次独立 Go 构建、Plugin bundle A/B 和 distribution candidate A/B 逐字节一致。
- `DISTRIBUTION-MANIFEST.json` SHA-256：`f8b09f3542ac7f355249ddf8b9beb3dd685be81b83868b834671b9db07401b89`。
- Plugin archive SHA-256：`e44b24e4728a412441e9c3163026c655a13d7c90acfad6ad08ae50a85a562294`。
- Universal 2 public CLI 与 private runner 均含 `x86_64 + arm64`；本机 arm64 CLI smoke PASS，runner 直接调用退出 `125`。
- 全新隔离 `CODEX_HOME` 通过 GitHub Git-backed Marketplace 的 `--sparse` 路径安装 rc.4；安装 cache 与 bundle A 逐文件一致。
- 只使用远程安装 cache 的 CLI，在中文、空格和 `#` 路径完成 embedded Starter 创建、初始化、Godot 4.7.2 Headless 8/8、GDScript 6/6、standard Debug build 与 Release export。
- GitHub Actions run `34143217696` 在 Marketplace revision 上完成 `verify-macos-arm64`，结论 `success`。
- 真实用户级 lifecycle 完成 rc.3 基线、rc.3→rc.4 升级、损坏候选退出 `1` 且 rc.4 保持 active、回滚 rc.3、重装 rc.4、卸载和配置逐字节恢复。
- 恢复后的 `config.toml` mode `0600`、大小 `5955`、SHA-256 `cbc4faaece632f785d8d3441216fbf2a39f85c493372217eaba9815f39bfe262`；Atelier Plugin/Marketplace 条目及目录均不存在，演练前存在的三个 staging 目录未修改。

## 下载观察

首次隔离 rc.4 sparse 获取最终成功。随后在真实用户级重复获取 rc.3 时，GitHub 经本机代理的 partial clone 在约 11 MiB 报 `curl 18`、`early EOF` 和 `index-pack failed`；失败发生在注册配置前，新增 staging 已移入私有演练快照。为把网络取得与 lifecycle 状态语义分开，真实用户级升级/回滚使用了已分别与远程 rc.3/rc.4 revision 核对的本地 Marketplace 工作树。该网络失败不得改写为 PASS，但不否定已经完整成功的隔离 rc.4 远程安装。

## 拒绝原因

安装 rc.4 后启动一次 `--ephemeral --sandbox read-only` 的全新 Codex 会话。Skill 发现本身 PASS：会话看见 `codex-game-atelier:develop-godot-game`，路径正确指向 `0.3.0-rc.4/skills/develop-godot-game/SKILL.md`，不再出现 rc.2 的陈旧版本路径。

同一启动日志明确警告 rc.4 `.codex-plugin/plugin.json` 的 `interface.defaultPrompt` 有 4 条，而当前 Codex CLI 最多支持 3 条，因此第 4 条被忽略。虽然 Plugin 安装、Skill 发现和 Godot E2E 均成功，但分发 manifest 不应在每次新会话产生已知警告或静默丢弃入口提示。rc.4 因此在 strict 聚合与正式最终审计前主动拒绝。

## 修复与回归要求

1. 把 validate 与固定 GDScript test 合并为一条清晰 prompt，使总数为 3。
2. `package_plugin.py` 在读取源或 bundle manifest 时强制 `defaultPrompt` 为一至三条非空字符串。
3. 增加回归测试，证明当前源为 3 条且第 4 条会被打包门禁拒绝。
4. 经受保护 PR 和 required CI 合并到新的 clean `main`。
5. 从新提交构建 rc.5，重新执行 A/B reproducibility、远程 sparse 安装、新会话无 warning、Godot E2E、strict 与独立最终只读审计。

rc.4 不创建 tag、Release 或正式 Plugin 发布。
