# M3 Codex Marketplace 生命周期准备验证

- 日期：2026-08-30
- Codex CLI：`0.151.0-alpha.7.1`
- 结论：本地 marketplace A/B 静态准备 PASS；用户级安装/升级/卸载/回滚 `NOT RUN`

## 当前 CLI 实证

本机 `codex plugin --help` 提供：

- `plugin marketplace add/list/upgrade/remove`
- `plugin add/list/remove`
- JSON 输出选项
- 非默认 marketplace 通过 `plugin marketplace add <local-path>` 注册

官方网页检索本轮未返回可用正文，因此上述仅作为当前本机 CLI 行为，不写成跨版本官方保证。Plugin Creator 随附规则要求 repo/team marketplace 的 Plugin source 为 `./plugins/<plugin-name>`，entry 明示 installation/authentication/category，更新后由新任务拾取。

## 已生成候选

- A：`.tools/marketplaces/codex-game-atelier-0.2.0-m3/`
- B：`.tools/marketplaces/codex-game-atelier-lifecycle-b/`
- marketplace name：`codex-game-atelier-local`
- Plugin selector：`codex-game-atelier@codex-game-atelier-local`
- A version：`0.2.0`
- B version：`0.2.0+codex.lifecycle-b`

B 的 public CLI `--version` 精确输出 `codex-game-atelier 0.2.0+codex.lifecycle-b`；Plugin bundle trusted native smoke PASS。A/B 均位于仓库忽略的 `.tools/`，尚未注册到用户 Codex。

## 自动化

| 验证 | 结果 |
| --- | --- |
| Python validators | PASS；48 项 |
| local marketplace builder | PASS；4 项，含已有输出、symlink 与未知根内容负例 |
| Plugin version override | PASS；SemVer/cachebuster 正例与非法版本负例 |
| Plugin Creator `validate_plugin.py` | PASS；A 已复制 marketplace bundle |
| Plugin Creator `read_marketplace_name.py` | PASS；输出 `codex-game-atelier-local` |
| A/B 包内版本与 native smoke | PASS |

## 权限边界与下一步

下一步会写用户级 Codex config/cache，并需要一个新任务验证 Skill 发现，必须先获得用户明确授权。授权不包含 remote、push、登录、发布、修改默认 personal marketplace 或删除其他 Plugin。演练最终应移除测试 Plugin 和专用 marketplace，使非测试用户状态回到开始快照。
