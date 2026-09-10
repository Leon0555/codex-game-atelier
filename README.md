# Codex Game Atelier

让 Codex 帮你设计制作游戏，协助你构建、测试并导出——过程可检查，结果有证据。（目前支持 Godot 引擎）

**简体中文** | [English](docs/readmes/README.en.md)

[![CI](https://github.com/Leon0555/codex-game-atelier/actions/workflows/ci.yml/badge.svg)](https://github.com/Leon0555/codex-game-atelier/actions/workflows/ci.yml)
[![Version](https://img.shields.io/badge/version-v1.0.0-2f81f7)](https://github.com/Leon0555/codex-game-atelier/tree/v1.0.0)
[![Godot](https://img.shields.io/badge/Godot-4.7.2--stable-478cbf?logo=godot-engine&logoColor=white)](docs/support-matrix.md)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Codex Game Atelier 是一个面向 Godot 开发的开源、Codex 原生协作框架。Codex 负责规划、判断、实现和有界协作；Plugin 内置的 Go CLI 负责确定性的项目操作、工作流门禁、结构化结果和持久化证据。

v1.0 有意保持精简：一个 Codex Plugin、一个顶层 Skill、一个内置 Starter 项目，以及一条经过真实验证的 Godot 工作流。普通用户不需要克隆本仓库、编译 CLI、安装 Node.js，也不需要先学会大量命令。

## 目录

- [为什么需要它](#为什么需要它)
- [三步开始](#三步开始)
- [包含什么](#包含什么)
- [工作原理](#工作原理)
- [已经验证的流程](#已经验证的流程)
- [支持矩阵](#支持矩阵)
- [安全与产品边界](#安全与产品边界)
- [工作流模式](#工作流模式)
- [卸载](#卸载)
- [仓库结构](#仓库结构)
- [项目状态](#项目状态)
- [项目文档](#项目文档)
- [灵感与来源](#灵感与来源)
- [许可证](#许可证)

## 为什么需要它

真正有用的游戏开发助手不能只是一组提示词。它还应该能够回答：实际运行了哪个引擎、哪些测试真的通过了、导出了什么、证据保存在哪里，以及哪些能力目前并不支持。

Codex Game Atelier 提供这层可验证的工程能力，同时不会变成托管式项目管理平台或后台常驻多代理服务：

- 创意和工程决策始终对用户可见，并由用户掌握最终决定权。
- 只有任务确实适合拆分时才使用 Codex 原生子代理，并明确所有权和只读评审边界。
- CLI 只执行定义明确的 Godot 与文件操作，以结构化 JSON 和稳定退出码返回结果。
- 项目状态和运行证据保存在可检查的 `.gameatelier/` 目录中。
- 构建、导出和发布检查自带必要门禁，不依赖可选 Git hook 才能保证正确性。

## 三步开始

### 前置条件

- Apple Silicon Mac。
- Godot `4.7.2-stable` 标准版，使用 GDScript。
- 支持 Plugin 的 Codex。

### 1. 添加固定版本的 Marketplace 来源

```sh
codex plugin marketplace add Leon0555/codex-game-atelier \
  --ref v1.0.0 \
  --sparse .agents \
  --sparse plugins/codex-game-atelier \
  --json
```

### 2. 安装 Plugin

```sh
codex plugin add codex-game-atelier@codex-game-atelier --json
```

以上两条命令会修改 `~/.codex` 下的用户级 Codex Plugin 状态，但不会安装 Godot、修改系统设置、添加 Git hook 或改动任何游戏项目。

### 3. 新建一个 Codex 任务

让 Codex Game Atelier 创建新项目：

```text
使用 Codex Game Atelier 在 /绝对路径/我的游戏 创建一个新的 Godot Starter 项目，
然后完成初始化和检查。
```

也可以接入已有项目：

```text
使用 Codex Game Atelier 检查 /绝对路径/已有游戏 下的 Godot 项目，
并运行 doctor。
```

Plugin 的 `develop-godot-game` Skill 会从自己的安装目录定位内置 CLI、runner、schemas 和 Starter Template，不会要求源码仓库，也不会在 `PATH` 中寻找同名程序。

## 包含什么

| 组件 | 作用 |
| --- | --- |
| `develop-godot-game` Skill | 路由项目创建、检查、验证、测试、构建/导出、证据读取和发布检查 |
| 确定性 Go CLI | 以结构化 JSON 和稳定退出码执行有界的项目与 Godot 操作 |
| 私有 runner | 执行固定的内部 Godot 检查，不是第二个面向用户的命令 |
| Atelier Spark Starter | 可游玩的小型 Godot/GDScript 垂直切片，覆盖输入、信号、资源、UI、胜利和重置流程 |
| 文件化状态与证据 | 在 `.gameatelier/` 下记录项目策略和不可变运行闭包 |
| 逻辑能力 Profiles | 表达责任和能力等级，不在分发内容中嵌入具体模型 ID |
| 工作流门禁 | 提供单调递增的 `manual`、`standard`、`strict` 模式，以及必选 CI 和可选 Git-hook 提示 |

## 工作原理

```text
用户请求
    |
    v
Codex + develop-godot-game Skill
    |-- 判断、规划、有界协作、评审
    |
    v
Plugin 内置的确定性 CLI
    |-- 安全与工作流门禁
    |-- 结构化结果与退出码
    |-- .gameatelier 状态与证据
    |
    v
Godot 4.7.2 标准版/GDScript
    |-- Headless 验证与固定测试
    |-- Debug/Release 技术导出
    `-- Apple Silicon 目标 smoke
```

CLI 不会选择模型，也不会充当隐藏规划器。外部发布、任意脚本、原始日志访问、依赖安装、签名和公证均不属于 v1 Skill 的能力。

## 已经验证的流程

v1.0 已在 macOS Apple Silicon 上真实验证以下路径：

1. 在全新目录创建内置 Starter，或者检查已有 Godot/GDScript 项目。
2. 检测项目和受支持的 Godot 安装。
3. 运行 `doctor`；需要导出时同时检查匹配版本的 export templates。
4. 初始化 `.gameatelier/project.json`，不覆盖合法的既有状态。
5. 执行静态验证或经用户明确授权的 Godot Headless 验证。
6. 执行固定的 GDScript 测试协议。
7. 生成带 manifest 和 hash 的 Debug/Release macOS 技术导出。
8. 检查 Universal 2 二进制 slices，并完成 Apple Silicon 启动/退出 smoke。
9. 通过只读 `release check` 聚合已经记录的事实。

失败会明确区分为 `FAIL`、`BLOCKED`、`SKIPPED` 或 `NOT RUN`；缺少证据绝不会被当作成功。

## 支持矩阵

| 范围 | v1.0 承诺 |
| --- | --- |
| 开发宿主 | macOS Apple Silicon |
| 引擎 | Godot `4.7.2-stable` 标准版 |
| 项目语言 | GDScript |
| Headless 验证 | 已支持并通过发布验证 |
| 游戏导出 | 未签名、未公证的 macOS 技术 ZIP |
| 导出架构 | 生成 Universal 2；只在 Apple Silicon 验证运行 smoke |
| Windows/Linux CLI 产物 | 仅保留交叉构建 inventory，不属于原生支持 |

完整的版本、宿主、导出、升级与淘汰政策见 [Godot v1.0 Support Matrix](docs/support-matrix.md)。

## 安全与产品边界

Codex Game Atelier 不会：

- 自动安装 Godot、SDK 或大型依赖；
- 启用遥测或进行隐藏外部写入；
- 在用户未明确要求时安装 Git hook；
- 把任意 shell、脚本或引擎 eval 暴露为核心工作流；
- 在分发提示、Skills、模板或运行时代码中硬编码具体模型 ID；
- 在 v1.0 中声称原生支持 Windows、Linux、macOS Intel、Godot .NET、移动端、Web 或主机平台；
- 在 v1.0 中发布独立 CLI archive、npm 包、DMG 或 PKG；
- 要求 Plugin-only 安装路径必须使用 Apple 签名或公证。

Headless 验证和测试能够执行所选项目中的 GDScript。请只对自己拥有或已经审阅的项目使用这些操作；v1.0 不声称能够沙箱隔离项目代码。

## 工作流模式

| 模式 | 使用场景 | 行为 |
| --- | --- | --- |
| `manual` | 高级用户显式控制 | 保留强制安全检查，只省略文档允许省略的工作流扩展 |
| `standard` | 日常开发 | 在 build/export 前自动运行必要的 Headless 和固定测试门禁 |
| `strict` | 发布准备 | 要求完整且经过验证的发布证据；缺失事实会继续明确阻断 |

Git hooks 只是可选的前置反馈。即使没有安装 hook，CLI 命令门禁和 required CI 仍然是权威检查。

## 卸载

```sh
codex plugin remove codex-game-atelier@codex-game-atelier --json
codex plugin marketplace remove codex-game-atelier --json
```

卸载会移除 Codex Plugin 及其 Marketplace 注册，不会删除用户创建的 Godot 项目。

## 仓库结构

```text
plugin/codex-game-atelier/   Plugin 与顶层 Skill 源码
packages/cli/                Go CLI 与私有 runner
starter-template/            内置 Atelier Spark Starter 源码
schemas/                     版本化状态、结果与证据契约
examples/                    参考游戏材料
tools/                       打包与验证工具
docs/                        架构、ADR、支持政策与验证证据
```

仓库根目录的 `AGENTS.md` 只约束 Codex Game Atelier 自身的研发过程，明确不会进入 Plugin、Starter Template 或生成的游戏项目。

## 项目状态

`v1.0.0` 是当前稳定的 Plugin-only 正式版本。它已经通过可复现打包、隔离远程安装、安装/升级/回滚/卸载生命周期、特殊路径 Godot 端到端验证、required CI、strict 发布聚合，以及无遗留发现的独立最终只读审计。

项目现处于发布后维护阶段。新增引擎、原生平台、导出目标、运行时或分发渠道都需要重新进行范围决策并补充验证证据。

## 项目文档

| 需要了解 | 从这里开始 |
| --- | --- |
| 产品目标和边界 | [项目简报](docs/project-brief.md) |
| 系统设计 | [架构基线](docs/architecture.md) |
| Godot 与平台支持范围 | [Support Matrix](docs/support-matrix.md) |
| 发布验收证据 | [v1 验收基线](docs/v1-acceptance.md) |
| 关键决策和取舍 | [架构决策记录](docs/adr/) |
| 来源与 clean-room 记录 | [来源记录](docs/provenance.md) |
| 安全问题报告方式 | [安全政策](SECURITY.md) |

## 灵感与来源

早期设计研究参考了 [merlinhu1/codex-game-studio](https://github.com/merlinhu1/codex-game-studio) 和 [Donchitos/Claude-Code-Game-Studios](https://github.com/Donchitos/Claude-Code-Game-Studios)。Codex Game Atelier 围绕更小的 Plugin-only 产品面、Codex 原生有界协作、确定性 Godot 操作和证据化发布门禁进行了独立重设。

本项目没有包含上述仓库的源码、提示词集合、模板或实质性文档。审计 commit、许可证、研究过的思想及 clean-room 差异记录在 [provenance.md](docs/provenance.md)。

## 许可证

Codex Game Atelier 采用 [MIT License](LICENSE)。预构建 Go 二进制相关的版权与第三方声明见 [NOTICE](NOTICE) 和 [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES)。

涉及安全漏洞的敏感报告请按照 [SECURITY.md](SECURITY.md) 使用 GitHub Private Vulnerability Reporting，不要发布到公开 Issue。
