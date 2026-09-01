# Godot v1.0 Support Matrix 实施基线

状态：**Phase 1 实施目标已冻结；所有生产级声明仍需 Phase 1+ 实证**
调研基准：2026-08-24
当前官方稳定版：Godot `4.7.2-stable`（2026-08-18）

已确认：

- v1.0 单版本基线为 Godot `4.7.2-stable`。
- v1.0 只支持标准版/GDScript，Godot .NET/C# 延后。
- Tier 1 宿主一次包含 macOS Apple Silicon、Windows x64、Linux x64。
- 新 patch 的完整矩阵重验目标为 7 个自然日。
- 不要求 Intel Mac 实机 smoke；macOS 支持声明限定为“生成 Universal 2，但只验证 Apple Silicon”，不声明 Intel 运行已验证。
- v1.0 不要求 Godot 游戏产物的 Developer ID 签名或 Apple 公证；只验证 macOS Apple Silicon 技术导出，不声明公开分发就绪。

## 1. 已确认版本策略

v1.0 只承诺一个经过完整验证的 Godot 稳定小版本，并固定到该系列最新已验证补丁：

- 冻结实施基线：**Godot 4.7.2-stable**。
- 新的 4.7.x 补丁不会自动进入支持矩阵；完整矩阵通过后，才替换基线。
- 4.8 等新 minor 不自动进入；先做兼容性窗口、迁移/回滚分析和完整复验，再形成范围决策。
- 预发布版本可做兼容性预览，永不计入 v1.0 生产承诺。

理由：Godot 官方发布政策说明同一 minor 只支持最新 patch，通常建议跟进 patch；minor 升级则需逐项目评估。把“任何最新 stable minor”自动纳入会让上游发布在没有我们证据时扩大承诺。

## 2. 三种版本策略比较

| 方案 | 截至调研日的具体范围 | 优点 | 风险与 CI 成本 | 建议 |
| --- | --- | --- | --- | --- |
| 单一稳定版本 | `4.7.2-stable` | 承诺清晰；缺陷与文档只维护一条主线 | 引擎版本维度 1×；patch 仍需快速重验 | **推荐 v1.0** |
| 当前稳定 + 前一稳定 | `4.7.2-stable` + `4.6.3-stable` | 兼顾未迁移的 4.6 项目 | 引擎测试、缓存、导入/导出证据约 2×；实际还乘以宿主和目标 | 有明确 4.6 用户需求与维护预算后再选 |
| 跟随当前稳定 minor 滚动 | “最新 stable minor 的最新 patch” | 宣传简单 | 每个 minor 可能立即改变承诺，验证总会滞后；回归/破坏性变化风险最高 | 不作为生产级对外定义 |

“1×/2×”只表示引擎版本轴。真实 CI 组合约为：引擎版本 × 宿主 OS × 测试层级 × 导出目标 × 签名/运行验证；不是精确费用估算。

## 3. 已确认宿主开发平台

| 宿主 | 推荐等级 | v1.0 验证范围 | 不自动推导的承诺 |
| --- | --- | --- | --- |
| macOS Apple Silicon | Tier 1 | CLI、Godot Editor/Headless、测试、构建、导出、异常恢复、中文/空格路径 | 不承诺 macOS Intel 作为开发宿主 |
| Windows x64 | Tier 1 | 同上，使用原生 Windows runner/机器 | 不承诺 Windows ARM/x86_32 |
| Linux x64 | Tier 1 | 同上；作为主要无 GPU headless CI | 不承诺 Linux ARM、发行版全集或所有桌面环境 |

Tier 1 的含义是每个发布候选都必须在该宿主用冻结版本完成规定的端到端流程；任何必选项缺失，该宿主不能标为生产级。

## 4. CI Headless 推荐

- PR 快速层：Linux x64、冻结 Godot 版本、`--headless`，覆盖 schema、CLI、场景/资源加载和测试。
- 合并/夜间层：macOS Apple Silicon、Windows x64、Linux x64 的宿主级 smoke 与恢复场景。
- Release 层：三宿主完整矩阵，加每个承诺导出目标的产物和目标平台启动证据。
- 无 GPU runner 必须使用 headless；有 GPU 时仍可用 headless 避免编辑器 UI。
- Headless 不等于“无需 Godot Editor 或 export templates”。命令行导出仍需要匹配的编辑器和模板。

CI 缓存只能提升速度，不能成为唯一证据。至少保留版本、runner、命令、退出码、日志、产物 hash 和 target smoke 结果。

## 5. 推荐桌面导出范围

宿主开发平台与游戏导出目标是两个独立轴；宿主上编辑器可运行，不证明任意目标可交付。

| 导出目标 | v1.0 推荐 | 验证要求 | 明确限制 |
| --- | --- | --- | --- |
| Windows x64 | 纳入 Tier 1 desktop export | Release/Debug export、产物检查、Windows x64 实机 smoke | 不含 Windows ARM/x86_32；代码签名单独验证 |
| Linux x64 | 纳入 Tier 1 desktop export | Release/Debug export、权限/产物检查、Linux x64 smoke | 不承诺其他架构、发行格式或发行版全集 |
| macOS Apple Silicon（Godot 产物格式为 Universal 2） | 纳入 Tier 1 desktop export | 未签名、未公证的 `.app`/ZIP 技术导出与 Apple Silicon smoke | 不做 Intel smoke；不声明 x86_64 运行或公开分发已验证。DMG 只能从 macOS 导出 |

Godot 官方模板的 macOS 应用是 Universal 2（x86_64 + arm64），但本项目的开发宿主和运行验证都只承诺 Apple Silicon。由于用户确认不做 Intel target smoke，对外必须使用准确声明：“生成 Universal 2，但只验证 Apple Silicon”；不得把产物含 x86_64 slice 等同于 Intel 已支持。

## 6. 构建和导出承诺

冻结矩阵内，“支持构建/导出”至少意味着：

- 实际运行 Debug 与 Release 命令，不能只检查 preset。
- 使用与 Godot 版本匹配的 export templates。
- 检查输出扩展名、文件布局、权限、hash 和非空产物。
- 在目标平台执行最小启动/退出 smoke。
- 不执行 Godot 游戏产物的 Developer ID 签名、公证或商店上传；技术导出 evidence 必须明确标记 `unsigned`、`not_notarized` 和 `public_distribution_ready: false`（最终字段名待 schema 冻结）。
- CJK/emoji 内容按 Godot ICU Data 要求另做 fixture 验证。
- `export_presets.cfg` 可进入项目；可能含密码/密钥的 `.godot/export_credentials.cfg` 必须保持机密并排除出普通证据与版本控制。

## 7. 明确不支持范围（推荐）

- Godot 4.8 dev/beta/RC、4.6 及更旧版本，除非后续矩阵另行纳入。
- Godot .NET/C#；v1.0 已确认只支持标准版/GDScript。
- Web、Android、iOS、主机、XR 和 dedicated-server 发行目标。
- macOS Intel 作为开发宿主或已验证运行目标、Windows ARM、Linux ARM。
- Godot 游戏产物的平台代码签名、公证、公开分发就绪验证、商店发布、账号代办或自动凭据管理。
- 用户自定义引擎构建、第三方 export templates 和所有渲染器/硬件组合。

## 8. 升级与淘汰策略

### Patch

1. 上游发布新的 4.7.x 后创建候选验证，不立即修改对外基线。
2. 在所有冻结宿主、核心测试、参考游戏和导出目标通过后发布框架兼容更新。
3. 新基线发布时淘汰旧 patch 的生产承诺，并保留至少一个可回滚的框架/矩阵记录。
4. patch 完整矩阵重验目标为 7 个自然日；在重验通过前，对外基线仍保持上一已验证 patch，不把上游发布自动视为支持。

### Minor

1. 新 stable minor 先作为非生产兼容性预览。
2. 记录项目迁移、资源重导入、API/渲染/导出变化和回滚方式。
3. 完整矩阵通过并经范围决策后，替换当前 minor；是否短期双轨另行评估。
4. 不用“当前 stable”浮动标签自动改变已发布框架的支持语义。

## 9. 生产级承诺的实际含义

生产级不是“Godot 官方可以下载”，而是：对每个列出的 `框架版本 + Godot patch + 宿主 + 能力/导出目标` 元组，有可复现 PASS evidence、稳定错误与恢复路径、干净环境复验、安装/升级/卸载验证，以及独立只读发布审计。缺失的元组必须标成 BLOCKED/NOT RUN/不支持。

## 10. 已确认的 macOS 分发边界

- 生成 Godot Universal 2 产物，但只在 Apple Silicon 上运行验证。
- 不做 Intel 实机 smoke，不声明 Intel 运行已验证。
- 不做 Godot 游戏产物的 Developer ID 签名或 Apple 公证。
- 对外只声明“macOS Apple Silicon 技术导出已验证”；不得声明产物已验证可直接面向普通 macOS 用户公开分发。
- 框架 v1 只通过 Codex Plugin 分发，包内携带 CLI/runner 与 Starter Template；不提供独立原生下载包。Apple 公证不列为默认门禁，改以干净 Apple Silicon 上的真实远程 Plugin 无阻断安装为门禁；若失败，再通过新决策评估公证这一备选方案。

## 11. 官方依据

- [Godot 官方下载归档](https://godotengine.org/download/archive/)
- [Godot 官方发布政策](https://docs.godotengine.org/en/stable/about/release_policy.html)
- [Godot 官方系统要求](https://docs.godotengine.org/en/stable/about/system_requirements.html)
- [Godot 4.7 命令行教程](https://docs.godotengine.org/en/4.7/tutorials/editor/command_line_tutorial.html)
- [Godot 项目导出](https://docs.godotengine.org/en/stable/tutorials/export/exporting_projects.html)
- [Godot macOS 导出](https://docs.godotengine.org/en/4.7/tutorials/export/exporting_for_macos.html)
- [Godot Windows 导出](https://docs.godotengine.org/en/stable/tutorials/export/exporting_for_windows.html)
- [Godot Linux 导出](https://docs.godotengine.org/en/stable/tutorials/export/exporting_for_linux.html)
