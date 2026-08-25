# Phase 1 Plugin Bundle 本地验证记录

- 日期：2026-08-26
- 结论：**PASS（仅限本地 candidate bundle、Apple Silicon 包内入口、Headless validate 与固定 GDScript test）**
- 非结论：不是 v1.0 发布通过，不代表 Codex 实际安装、Gatekeeper、Linux/Windows 原生运行或 Marketplace 发布通过
- 基线提交：`4319f37f7418eb6c508451aa9b7d34868139e7a0`；验证包含其后的未提交 Phase 1 Plugin 修改，因此 artifact 不可发布
- 宿主：macOS 26.6.2 / Darwin 25.6.0 / arm64
- Go：`go1.27.0 darwin/arm64`（项目本地工具）

## 实际结果

| 检查 | 状态 | 证据摘要 |
| --- | --- | --- |
| Plugin manifest | PASS | `plugin-creator` 官方 validator 通过；版本 `0.2.0` |
| `develop-godot-game` Skill | PASS | `skill-creator` quick validator 通过；只暴露已实现命令和固定 CLI 路径 |
| 源文件边界 | PASS | 显式 allowlist 仅含 manifest、Skill 与 UI metadata；bundle 无 `AGENTS.md`、Go 源码或构建缓存 |
| macOS artifact | PASS（形状） | CLI/runner 均为 Universal 2，实际 slices 为 `x86_64 + arm64` |
| Apple Silicon CLI | PASS | 包内 `codex-game-atelier --version` 精确输出 `codex-game-atelier 0.2.0` |
| Apple Silicon runner | PASS | 无 fd 控制协议时固定 stderr，退出 125，未被误当公共 CLI |
| 包内 Headless validate | PASS | 固定一帧参考游戏验证 PASS/0；run `atelier-20260825t174009.156041000z-f3be43ad33e9`，producer `0.2.0` |
| 包内 GDScript test | PASS | 固定 runner 5/5 PASS/0；run `atelier-20260825t174030.695616000z-213342fb2e04`，engine `4.7.2.stable.official.ed1daf0bf` |
| 包内结构化日志 | PASS | 包内 CLI 对既有 GDScript test run 查询为 PASS/0，输出仍为零自由文本结构事件 |
| Linux artifact | PASS（仅形状） | CLI/runner 为 ELF amd64；原生运行 NOT RUN |
| Windows artifact | PASS（仅形状） | CLI/runner 为 PE32+ amd64；原生运行 NOT RUN |
| bundle manifest | PASS | 11 个内容文件、展开 27,730,300 bytes；逐文件 SHA-256/size/mode 与实际一致 |
| archive | PASS | 可复现 `.tar.gz` 规则已实现；当前 finalcheck archive 11,629,325 bytes |
| 外部 checksum | PASS | 当前本地 finalcheck archive SHA-256：`68ee56e1dba6a68b723eef602f048a0f978bb64235f1f96e8e54baf665fc9758` |
| archive 重现性 | PASS | 同一 bundle 两次生成的压缩包经 `cmp` 逐字节相同 |
| 中文/空格/特殊路径 | PASS（本地 staging） | 在 `.tools/Plugin 安装验证 #1/` 中复验 archive、复制后的 bundle、`--version` 与结构化 logs；不等价于 Codex 客户端安装 |
| tamper / 伪二进制 | PASS | 12 个 packager 回归测试覆盖文本/截断头伪装、bundle/archive 篡改、路径穿越、symlink/hardlink、未知内容/角色 mode、既有输出、static/trusted smoke 分离、版本不匹配、后台 child 与输出洪泛 |

## 关键命令

```text
/usr/bin/python3 -m unittest tools/validators/test_package_plugin.py
/usr/bin/python3 tools/package_plugin.py build ...
/usr/bin/python3 tools/package_plugin.py verify <bundle>  # static only
/usr/bin/python3 tools/package_plugin.py smoke-trusted-bundle <trusted-local-bundle>
/usr/bin/python3 tools/package_plugin.py archive --bundle <bundle> --output <archive>
/usr/bin/python3 tools/package_plugin.py verify-archive <archive>
<bundle>/bin/darwin-universal2/codex-game-atelier --version
<bundle>/bin/darwin-universal2/codex-game-atelier logs --project examples/reference-game --run-id <verified-run>
```

构建使用 `-trimpath` 以及 `-ldflags '-s -w -X .../internal/app.Version=0.2.0'`。产物保留在忽略的 `.tools/` 下，没有加入 Git。

首次在受限沙箱内运行 bundle Headless validate 得到预期的 FAIL/5：Godot 无法写标准 `user://logs` 并读取系统 CA，run `atelier-20260825t173834.842209000z-01ceb14128fb` 保留了失败 evidence。随后按既有 `--allow-engine-user-data` 授权在沙箱外重跑，同一固定工作流 PASS。该失败不能删除，也不能计入功能回退；它证明环境权限不足时命令不会误报通过。

## 防误报说明

- Manifest 的 macOS `native_validation` 保持 `NOT_RECORDED`；本次 Apple Silicon PASS 只记录在本文，避免常量冒充 evidence。
- “生成 Universal 2”不代表 Intel 实机 smoke；Intel 为 NOT RUN。
- Linux/Windows 交叉构建只证明当前字节形状，不证明目标宿主行为。
- checksum/manifest 不证明代码安全，也不替代可信发布来源和 package provenance。`build`/`verify`/`archive`/`verify-archive` 均不执行输入代码；只有显式 `smoke-trusted-bundle` 可执行同一可信本地流水线产物。
- `telemetry_enabled=false` 是分发策略字段，不是无网络/无遥测的动态行为证据；v1 的对应验收仍未因此自动通过。

## 尚未运行

- Codex 客户端真实安装缓存中的加载、新任务 Skill 发现、相对路径定位、卸载和残留检查。
- 带真实 quarantine 的下载/解包与 Gatekeeper 验证；框架 CLI 是否需要签名/公证仍未决。
- 升级失败不切换 active version、上一版本回滚。
- Linux x64 与 Windows x64 原生 runner/机器验证（按用户决定延后）。
- npm、GitHub Release、Marketplace 发布或任何远程写入。

## 独立只读终审

第四轮复核在静态验证/可信执行拆分、二进制结构边界、固定内容 allowlist、Linux/Windows Skill 门禁和后台进程测试全部修正后给出：Blocker 0、High 0、Medium 0、Low 0，可提交。审查代理未修改文件；真实安装、Gatekeeper、生命周期、Linux/Windows native 与外部发布继续按上节保持 NOT RUN。
