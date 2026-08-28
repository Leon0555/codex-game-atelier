# Phase 1 Starter Template 方案 A 打包验证记录

> 历史记录：本页描述 2026-08-26 的初始薄模板 candidate。当前可玩模板重打包与特殊路径 E2E 见 [`m1-playable-vertical-slice-2026-08-28.md`](m1-playable-vertical-slice-2026-08-28.md)；旧 archive 摘要和 evidence 保留、不覆盖。

- 日期：2026-08-26
- 结论：**PASS（仅限确定性本地 Template archive candidate）**
- 决策：ADR 0014 Accepted，v1.0 采用 Starter Template 与已安装 Plugin 配套
- 非结论：不代表 Codex 客户端已安装/发现 Plugin，不代表生命周期、Gatekeeper 或 v1 发布通过
- 宿主：macOS 26.6.2 / Darwin 25.6.0 / arm64
- 基线提交：`2d91d88`；本记录包含其后的未提交方案 A 修改，artifact 不可发布

## 包边界

archive 根目录固定为 `codex-game-atelier-starter/`，包含 8 个模板源文件、`LICENSE`、`NOTICE` 和 `TEMPLATE-MANIFEST.json`。不包含 `bin/`、`skills/`、Plugin manifest、`AGENTS.md`、`.gameatelier/`、Godot cache、SDK 或引擎。

`TEMPLATE-MANIFEST.json` 记录：

- template `0.2.0`，Godot `4.7.2-stable` standard/GDScript。
- 配套 `codex-game-atelier` Plugin，本 candidate 已验证版本 `0.2.0`，`embedded=false`。这不是最低兼容版本声明。
- 10 个非 manifest 文件的相对路径、SHA-256、字节数和 mode；展开内容 8,474 bytes。
- `telemetry_enabled=false` 为分发策略字段，不替代后续动态网络审计。

## 实际结果

| 检查 | 状态 | 证据 |
| --- | --- | --- |
| 源模板 allowlist | PASS | 打包前重跑固定 8 文件/3 目录契约 |
| 配套而非嵌入 | PASS | manifest `embedded=false`；无 `bin/` 或 `skills/` |
| Plugin 版本闭合 | PASS | template 版本与 `verified_plugin_version` 均为 `0.2.0`；验证旧 archive 不依赖当前开发树 Plugin 版本，兼容范围尚未冻结 |
| 源契约重验 | PASS | 安全解包后不信任内部哈希，独立投影8 个源文件重跑 validator |
| archive 可复现 | PASS | 同一 package 两次生成的 `.tar.gz` 逐字节一致 |
| archive 完整性 | PASS | 5,100 bytes；SHA-256 `4b76336a82539f692820e98672653c94d5abd455ef452bfb6e98698dc0bbd547` |
| manifest 快照 | PASS | 2,205 bytes；SHA-256 `3724fbd8d1346ef34bfaa9e9a28330c33917897be192ce69c918913c00257c71` |
| 安全解包 | PASS | 在 tar 解析前对整个单 gzip 解压流设硬上限，拒绝拼接 gzip、PAX/GNU 元数据炸弹、路径穿越、symlink/hardlink、特殊类型、mode 异常和大小写冲突 |
| 回归测试 | PASS | 8 项 package 测试，包含篡改后重建内部 manifest 仍被源契约拒绝、解压流攻击回归，以及跟踪 manifest 与当前源的一致性 |

外部 `.sha256` 与包内 manifest 只证明这组同源文件自洽；攻击者可以同时替换 archive、manifest 和 checksum。真正的执行信任必须来自独立可信渠道的预期摘要、签名或 package provenance；本记录不将 checksum 冒充发布者认证，D-008 继续为 NOT RUN。

跟踪摘要见 [`evidence/phase1-starter-template-package-2026-08-26/`](evidence/phase1-starter-template-package-2026-08-26/)。与本记录摘要一致的最终本地 candidate 保留在被忽略的 `.tools/starter-bundles/option-a-final/`，旧的实现中 artifact 仅作历史保留，未进入 Git。

## 关键命令

```text
python3 tools/package_starter_template.py build --output <new-package-directory>
python3 tools/package_starter_template.py verify <package-directory>
python3 tools/package_starter_template.py archive --package <package-directory> --output <archive.tar.gz>
python3 tools/package_starter_template.py verify-archive <archive.tar.gz>
python3 -m unittest tools.validators.test_package_starter_template -v
```

## 尚未运行

- Codex 客户端中 Plugin 实际安装、Skill 发现和三步新任务演练。
- Plugin 升级失败不切换 active version、回滚与卸载后游戏项目不变。
- 带 quarantine 的真实下载/Gatekeeper 路径。
- 干净用户环境和外部发布。
