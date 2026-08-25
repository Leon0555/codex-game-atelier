# Phase 1 基线验证记录

后续生产 Go CLI 首轮实现与验证见 `phase1-production-cli-2026-08-25.md`；本文件保留 2026-08-24 当时的 Spike/骨架证据，不把后续结果倒写成当日事实。

日期：2026-08-24
主机：macOS Apple Silicon
范围：ADR/Support Matrix 冻结、schema、Plugin/Skill 源骨架与工具链计划

| 检查 | 结果 | 证据/说明 |
| --- | --- | --- |
| Phase 0 审阅门禁 | PASS | 用户明确回复“Phase 0 审阅通过，进入 Phase 1” |
| JSON 语法 | PASS | 系统 Python `json.tool` 解析 `schemas/v1/*.json`、fixtures 与 `plugin.json`，退出 0 |
| Skill frontmatter/占位符 | PASS（离线基础检查） | 系统 Ruby/Psych 成功解析 YAML；必需 `name`/`description` 存在；无 `[TODO:` |
| Plugin 基础清单 | PASS（离线基础检查） | JSON 可解析；必需元数据存在；目录名与 manifest name 都是 `codex-game-atelier` |
| 分发内容模型名/Unity 扫描 | PASS | `plugin/`、`schemas/`、schema fixtures 中未发现 `gpt-5*`、`Unity` 或 `unity` |
| `skill-creator` 官方 validator | BLOCKED | 系统与 Codex bundled Python 均缺少 `PyYAML`；未自动安装 |
| `plugin-creator` 官方 validator | BLOCKED | 系统与 Codex bundled Python 均缺少 `PyYAML`；未自动安装 |
| Draft 2020-12 schema/fixture 语义验证 | BLOCKED | 环境缺少 `jsonschema`/等价 validator；JSON 语法通过不等于 schema 语义通过 |
| Go 语言对照样品 | PASS | Go 1.27.0；vet/build、结构化 doctor、同 Unix 进程组 timeout、父先退出/持管道、输入与消息边界通过；详细数据见结果文档 |
| Rust 语言对照样品 | PASS | Rust 1.98.0；fmt、locked/offline build/test、Clippy，以及与 Go 相同的结构化/负向矩阵通过；详细数据见结果文档 |
| v1 CLI 生产实现语言 | PASS | 用户于 2026-08-25 明确批准 Go；ADR 0004 已更新，Rust 只保留为非生产对照证据 |
| 原子 evidence 写入 | NOT RUN | 两份样品 `evidence` 均为空；尚无项目相对路径、临时文件 + rename、失败恢复或越界拒绝实现 |
| Go x64 交叉构建形状 | PASS（有限） | 不增加 SDK 生成 macOS x86_64 Mach-O、Linux x86_64 静态 ELF、Windows x86_64 PE；均未在目标宿主运行 |
| Godot 版本与模板检测 | PASS | 4.7.2 official；templates 4.7.2.stable；官方散列通过 |
| Godot 场景与 CJK/空格资源 | PASS | headless 输出 reference JSON、exit 0 |
| Godot clean headless 外部写入边界 | BLOCKED | macOS 项目 `user://` 仍尝试用户 Application Support；沙箱阻止并产生 ERROR |
| macOS Release 技术导出 | PASS（有限） | Universal 2 ZIP 与 Apple Silicon 功能启动通过；签名/公证不运行，public_distribution_ready=false |
| Codex Marketplace 安装测试 | NOT RUN | 当前只生成仓库内源骨架，未获用户级 Marketplace 修改授权 |

## 已执行命令类别

- `python3 -m json.tool`：JSON 语法。
- `ruby -r yaml -r json`：Skill frontmatter 与 Plugin 基础不变量。
- `rg`：未完成占位符、具体模型名和 Unity 分发内容扫描。
- 官方下载页、响应头、清单和散列：固定版本与来源验证；在用户批准后下载并安装到仓库 `.tools/`。
- `go fmt/vet/test/build` 与 `cargo fmt/test/clippy/build --locked --offline`：两个可丢弃 runtime 样品。
- Godot headless、场景执行、Release export、产物架构/散列检查：参考项目薄切片。

## 限制

当前结果证明 Phase 1 源骨架、两个可丢弃 runtime 语言对照样品和 Godot 场景/技术导出薄切片的本机行为；不证明原子 evidence、Plugin 安装、schema fixtures Draft 2020-12 语义、生产 CLI，或 Windows/Linux 宿主已经验证。后续不得把 BLOCKED/NOT RUN 或交叉编译文件形状改写成生产 PASS。
