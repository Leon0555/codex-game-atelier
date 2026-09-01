# M3 单 Plugin 本地候选与 embedded Starter 实证

- 日期：2026-09-01
- 源码提交：`969bef08d54c73d131ee0476ac096a168fcf80e9`
- 候选版本：`0.3.0-rc.1`，`local-candidate`
- Plugin bundle：`.tools/plugin-bundles/plugin-only-0.3.0-rc.1-969bef0/`
- Distribution candidate：`.tools/distributions/codex-game-atelier-0.3.0-rc.1-plugin-only-969bef0/`
- 宿主：macOS Apple Silicon
- 结论：单 Plugin 本地闭包、可信本机入口、embedded Starter 创建/初始化、strict 本地分发门禁和 A/B 重现性 PASS；远程 Plugin、required hosted CI 与最终独立审计仍 NOT RUN

本轮只写项目仓库、项目忽略的 `.tools/` 构建目录和 `/private/tmp` 临时项目。没有修改用户级 Codex 状态，没有安装 Plugin，没有联网、登录、创建 remote、push 或发布。

## 1. 提交前门禁

| 验证 | 结果 |
| --- | --- |
| Draft 2020-12 Schema | PASS；26 schemas、31 fixtures、35 negative assertions |
| Python validators | PASS；52/52 |
| Go tests | PASS；`internal/app` 42.818 秒 |
| Go vet / gofmt / `git diff --check` | PASS |
| Linux amd64 / Windows amd64 交叉编译 | PASS；只证明 artifact 可生成，不证明原生支持 |

## 2. 单 Plugin 候选

使用仓库内 Go `1.27.0`、`CGO_ENABLED=0`、`-trimpath` 和版本 `0.3.0-rc.1` 构建 public CLI/private runner。macOS 的 amd64 与 arm64 薄文件由 `/usr/bin/lipo` 合并；两个入口均实测为 `x86_64 arm64`。Plugin 打包器验证六个文件、八条 Go build record 全部对应 clean revision `969bef0...`，随后完成 trusted native smoke：

- public CLI `--version`：PASS，返回 `codex-game-atelier 0.3.0-rc.1`。
- private runner 无 fd invocation：PASS，按契约退出 `125`。
- Plugin manifest schema：`1.2.0`；32 个固定文件；embedded Starter 与 Plugin 精确配对。
- Distribution manifest schema：`1.2.0`；唯一产品 archive 为 Plugin；`distribution_channel=codex-plugin-only`；`apple_notarization_required=false`；`remote_plugin_gatekeeper_validation=NOT_RUN`。

主要候选摘要：

| 文件 | 字节 | SHA-256 |
| --- | ---: | --- |
| `DISTRIBUTION-MANIFEST.json` | 2,542 | `75b1c15360e51b4fda091a508d44146c54ec758c7428db384bd73566ce94d1ee` |
| `codex-game-atelier-0.3.0-rc.1.tar.gz` | 13,395,630 | `a36b0658944077bbaeacc7d6d158fb1f510a255c60309a3435d2b61c519ee36c` |
| `THIRD_PARTY_NOTICES` | 1,640 | `7055aea9a5203534e38b838951401c19bede9a34c7515b6966a111019178f2e6` |

## 3. 真实 embedded Starter 创建

从已打包 Plugin 的实际路径运行 public CLI，不注入源码 fixture：

1. `starter create --project <中文、空格、# 的新目录>`：PASS；创建 11 个用户文件、13,617 bytes；结果只记录 `project=provided`，`created=true`、`initialized=false`。
2. 检查创建树：包含 Starter 源文件及 LICENSE/NOTICE；不含 `TEMPLATE-MANIFEST.json`、Plugin、CLI/runner、Skill、`AGENTS.md` 或 `.gameatelier`。
3. `initialize --project <新目录>`：PASS；原子建立 revision 0、standard、Godot `4.7.2-stable`、GDScript 状态。
4. `status --project <新目录>`：PASS；严格只读回读同一状态。

单元负例另外覆盖：已有目标、父目录缺失、内容/hash 篡改、未知文件、symlink、版本错误、`embedded=false`、mode 错误、非法用法和预取消；均未覆盖用户已有目标或发布半成品目录。

## 4. Strict 本地门禁

候选内 public CLI 对新初始化项目执行显式 strict candidate 检查，退出 `4`、整体 `BLOCKED`，符合未完成门禁语义：

| 门禁 | 结果 |
| --- | --- |
| project-state / support-scope / run-store-integrity | 3 PASS |
| clean-source / Plugin / embedded Starter / license-provenance | 4 PASS |
| latest Headless / fixed tests / Release export | 3 BLOCKED（新项目尚无这些 evidence） |
| remote-plugin-install / required-ci | 2 NOT RUN |

总计 7 PASS、3 BLOCKED、2 NOT RUN，`release_ready=false`。本地 candidate 输入只记录为 `provided`；命令零项目写入、零 evidence、零 candidate 修改。

## 5. 重现性与未完成项

从同一 clean revision 和同一六个二进制独立生成第二份 Plugin bundle 与 distribution candidate；两级 `diff -rq` 均无输出，证明打包结果逐字节一致。

本记录不是远程安装、全新用户/机器复验或最终独立审计。下列门禁仍未完成：

- 真实远程 Plugin 来源在干净 Apple Silicon 上无系统设置放行、无 `xattr`、无隐藏策略修改的安装与 Godot 工作流。
- GitHub-hosted required CI。
- 最终候选的真实升级、失败升级与回滚。
- Windows/Linux 原生证据或正式缩减 Support Matrix 的用户决策。
- 独立只读架构、安全、许可证与发布终审，以及用户正式发布批准。
