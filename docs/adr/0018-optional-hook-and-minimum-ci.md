# ADR 0018：显式可选 Hook 与最小 macOS CI

- 状态：Accepted（Phase 1 M2 实现）
- 日期：2026-08-29
- 决策范围：公开 hooks 命令、所有权/卸载边界与 M2 最小 CI

## 背景

ADR 0002 要求 hook 只提供早期反馈，不能自动安装、覆盖用户已有行为或成为 build/export/release 的唯一门禁。M2 同时需要一个能尽早发现源码回归的 CI 基线，但用户已明确延后 Windows/Linux 原生 runner；M3 的 Godot、分发、许可和发布完整矩阵也尚未完成。

## 决策

1. 公开 `hooks list|plan|status|install|uninstall --project <path>`。所有命令只输出一个 command-result JSON，不写 `.gameatelier` evidence。
2. 只管理默认 `.git/hooks/pre-commit` 与 `.git/hooks/codex-game-atelier.manifest.json`。`initialize`、Plugin 安装和普通 CLI 命令从不调用 install。
3. hook 绑定安装时当前 CLI 的规范化绝对路径，只执行固定 `release check --project <root> --mode manual`；不复制 gate 逻辑、不接受任意命令或参数。
4. install 只在两个目标均不存在时原子发布；已有 hook、部分/损坏状态、有效 `core.hooksPath` 或 linked-worktree `.git` 文件一律阻断，不自动组合或覆盖。
5. manifest 记录 schema、owner、hook、CLI version、固定 check 与 hook SHA-256。uninstall 只删除仍含 ownership marker 且字节摘要匹配 manifest 的 hook；旧 CLI 路径形成 `stale` 状态，但仍可验证所有权并安全卸载。
6. M2 CI 只有一个 `macos-15` Apple Silicon job：`contents: read`，外部 action 使用完整 commit SHA，验证 Go 1.24.x 最低版本、Go format/vet/test、Python Schema/Plugin/Starter 测试，以及本机 CLI/private runner 构建与固定退出 smoke。
7. CI 不调用 hook，不安装 Godot，不执行外部发布。远程仓库未创建时，workflow 的本地结构/等价命令可以 PASS，GitHub-hosted 执行必须保持 `NOT RUN`；远程建立后必须以真实 run 替换该阶段状态。

## 备选方案

### 自动安装或自动合并 hook

拒绝。会改变用户 Git 行为，并需要推测任意第三方 hook 的组合语义。

### hook 执行 standard/strict 或直接 build

拒绝。pre-commit 不应启动 Godot、写 engine user data 或产生昂贵 artifact；真正门禁已经内建于 build/export，发布由 strict CI 聚合。

### 立即增加 Windows/Linux 或完整 Godot CI

延期。用户已明确延后对应原生 runner，完整 Godot/分发发布矩阵属于 M3；M2 先建立不可绕过的源码验证骨架。

## 风险

- 绑定绝对 CLI 路径后移动/升级安装会使 hook 变为 `stale`；用户需显式 uninstall/install，不能静默重写。
- manual release check 是轻量反馈，不证明源码树或 artifact 发布就绪；文档和结果不得夸大它。
- GitHub-hosted runner 标签与 action 版本会演进；更新 SHA 或 runner 必须经过 workflow contract 测试和审阅。
- 当前不支持 linked worktree 与自定义 hooks path；应清晰阻断并给出手动路径，而非静默安装到无效位置。

## 迁移与回退

- hook manifest 使用独立 schema version；当前没有历史 managed hook 需要迁移。
- CLI 升级后先 `hooks status`；`stale` 可用旧 manifest 安全卸载，再由新 CLI 显式安装。
- 删除 CI workflow 可回退远程自动验证，但不会削弱 CLI 内建 build/export 门禁；发布不得在 required-CI 缺失时继续。

## 验证

- list/status/plan 前后 hooks 目录字节级不变。
- install/reinstall/uninstall 正例；既有 hook、篡改、stale、custom hooksPath 和取消负例。
- 实际执行含空格、中文和单引号路径下生成的 hook，确认参数边界不被 shell 展开。
- Schema fixture、CI YAML 解析、只读权限、单 job、固定 runner/action SHA 与本地等价测试。

## 实施证据补记

2026-09-01，公开远程仓库建立后，GitHub-hosted `macos-15` Apple Silicon run `33515728377` 在修复首次 Go 1.24 测试假失败后全部 PASS。运行证据见 [`m3-remote-plugin-lifecycle-2026-09-01.md`](../validation/m3-remote-plugin-lifecycle-2026-09-01.md)。该结果证明 hosted workflow 可运行；在 branch protection 明确将它设为 required check 之前，不宣称远程合并不可绕过。
