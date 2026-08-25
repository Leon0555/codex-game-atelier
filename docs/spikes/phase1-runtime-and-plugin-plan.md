# Phase 1：CLI Runtime 与 Plugin 打包 Spike 计划

状态：计划已批准；三个项目本地安装与语言对照已完成，Go 决策已冻结
日期：2026-08-24；更新：2026-08-25

## 目的

用最小、可丢弃的样品回答三个问题：

1. Rust 与 Go 哪个更适合确定性 CLI 的 v1 实现。
2. Codex Plugin 如何携带或定位不同平台的预构建 CLI，而不要求普通用户编译。
3. 在已安装受支持 Godot 的前提下，能否形成 Plugin/Starter Template 主入口、CLI 为底座的最多三步首次使用路径。

Spike 样品本身不冻结产品语言，也不进入生产路径、创建远程或发布包。语言对照完成后，用户于 2026-08-25 依据结果另行批准 Go 为 v1 CLI 生产实现语言；Rust 样品仍只作为研究证据。

## 官方版本快照

| 项目 | 候选 | 官方压缩传输量 | 建议安装范围 |
| --- | --- | ---: | --- |
| Rust | `1.98.0` stable，rustup `minimal` | rustup-init + cargo + rustc + rust-std 共 99,090,420 bytes（约 94.5 MiB） | 仓库 `.tools/cargo` 与 `.tools/rustup`；不修改 PATH |
| Go | `1.27.0` stable，darwin-arm64 archive | 68,303,667 bytes（约 65.1 MiB） | 仓库 `.tools/go/1.27.0`；不用系统 pkg |
| Godot | `4.7.2-stable` standard macOS Universal | 170,622,178 bytes（约 162.7 MiB） | 仓库 `.tools/godot/4.7.2`，自包含模式 |
| Godot export templates | `4.7.2-stable` standard | 1,281,349,702 bytes（约 1.19 GiB） | 同一自包含 `editor_data`，不写用户 Godot 目录 |

Godot 官方系统要求把编辑器、全部模板和缓存的推荐存储量列为 2 GB；最初为解压、缓存和两个 CLI build tree 预留 5 GiB。实际达到约 4.8 GiB 时已按计划暂停，用户随后确认真实工具需要时允许超过该估算。Phase 1 当前以 8 GiB 作为异常增长软告警，不作为正常工具安装的硬门禁；所有占用继续保持项目本地、可盘点、可回收。

## 安装隔离

- 所有新增工具放入已忽略的 `.tools/`，不放入 `/Applications`，不修改 shell profile、系统配置或用户级 PATH。
- Rust 使用任务专属 `CARGO_HOME`/`RUSTUP_HOME` 和 `--no-modify-path`。
- Go 使用官方 archive，直接解压，不运行 macOS Installer。
- Godot 使用官方标准版；`_sc_` 放在 `.app` 同级而不是包内，使 editor data、settings、cache 和 templates 留在工具目录且不破坏 app 签名。
- 不安装 .NET SDK、CMake、Docker、Git hooks，也不登录 GitHub/npm。

## Rust/Go 对照用例

两个样品实现同一窄契约：

- `--version` 和 `doctor` 的结构化 JSON 输出。
- 稳定退出码与 `BLOCKED` 错误。
- 含中文、空格和特殊字符的项目路径。
- 启动受控子进程、收集 stdout/stderr、超时和取消。
- 原子写 evidence 到指定相对路径，不越过项目范围。

记录冷/热启动、release binary 大小、构建时间、依赖树、交叉构建要求、错误表达、升级/回滚和供应链面。比较结论必须来自相同用例，不按语言偏好决定。

## Plugin 打包用例

1. 保持 `.codex-plugin/plugin.json`、Skill 与平台二进制路径相对且可验证。
2. 先验证源目录，再在用户另行批准后建立仓库本地 Marketplace 并安装测试。
3. 验证 Plugin 安装缓存中的资源定位、CLI 执行权限、带空格/CJK 路径和 macOS Gatekeeper 行为。
4. 验证升级、回滚和卸载不修改游戏项目外的未声明位置。
5. 新任务中验证 Skill 可发现且不依赖源码 clone/npm build。

当前只创建源骨架；尚未写 Marketplace 或修改用户级 Codex 配置。

## Godot 薄切片

安装获批后依次验证版本输出、自包含写入位置、headless 启动、最小 GDScript 项目、结果/evidence schema，以及 Debug/Release 技术导出。macOS 只在 Apple Silicon 运行验证；产物未签名、未公证且不声明公开分发就绪。

## 停止条件

- 任一安装将写入 `.tools/` 之外时先停止。
- 下载或安装版本超出上述批准范围时先停止；空间超过 8 GiB 软告警时先盘点来源并报告，不因已知正常组件机械失败。
- 同一验收条件连续失败两次时升级分析，不反复试装。
- 需要用户级 Marketplace、GitHub/npm 登录、签名、公证或外部发布时单独请求授权。

## 官方依据

- [Rust 官方发布列表](https://blog.rust-lang.org/releases/)
- [Rust 官方安装说明](https://www.rust-lang.org/tools/install)
- [Go 官方下载页](https://go.dev/dl/)
- [Godot 4.7.2 官方归档](https://godotengine.org/download/archive/4.7.2-stable/)
- [Godot 4.7 系统要求](https://docs.godotengine.org/en/4.7/about/system_requirements.html)
- [Godot EditorPaths 与自包含模式](https://docs.godotengine.org/en/4.7/classes/class_editorpaths.html)
- [Codex Plugin 官方文档](https://learn.chatgpt.com/docs/build-plugins)
