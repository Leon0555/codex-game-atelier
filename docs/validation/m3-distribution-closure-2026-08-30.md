# M3 本地分发闭包验证

- 日期：2026-08-30
- 宿主：macOS Apple Silicon
- 基线提交：`98ffa65`；本记录还包含其后的未提交 M3 实现与 Windows 交叉编译修复
- candidate 版本：`0.2.0`（仅本地开发候选，不是 v1.0 版本冻结）
- 结论：本地分发闭包 PASS；真实 Codex 安装/升级/卸载/回滚与外部发布均 `NOT RUN`

## 完成范围

- 新增维护端 `package_distribution.py build|verify`，不进入 Plugin/Starter 用户运行时。
- Plugin、公共 CLI、private runner、Starter 与配套 Plugin 版本精确闭合。
- candidate 只含两个 archive/外部 checksum、MIT `LICENSE`、`NOTICE` 和分发 manifest。
- 两个 component archive 重新执行安全解包/静态验证；候选文件 hash、size、mode 和 allowlist 重新核对。
- framework 签名/公证/Gatekeeper 状态保持 `NOT_EVALUATED`，没有借用 Godot 游戏技术导出的免签决定。

## 实际产物

- candidate：`.tools/distributions/codex-game-atelier-0.2.0-m3-final/`
- 展开候选大小：12,622,521 bytes（manifest 自身不计入 inventory）
- `DISTRIBUTION-MANIFEST.json` SHA-256：`b1b616d1cc4f7f8c2a0e7a2c20f8b91f9769d5f86eee23bc73d35625ef45ece5`
- Plugin archive：12,613,930 bytes；SHA-256 `fdc2c4f908097d812560e66d2984decde1254274b932fdc476bddd45c79557d2`
- Starter archive：6,640 bytes；SHA-256 `30573faf9eb5df215a02b42ba910080340887a0dc569ee44814bfe984982e318`
- 第二份相同输入 candidate 经 `diff -rq` 无输出，逐文件字节一致。

上述 `.tools/` 路径均被忽略，不进入源码提交，也不是外部发布地址。

## 构建与宿主证据

- macOS public CLI/private runner 分别构建 arm64 与 amd64，再由 `lipo` 合并；两者均验证为 `x86_64 arm64`。
- 当前 trusted Plugin bundle 的 public CLI `--version` 与 private runner 固定拒绝契约 smoke PASS。
- Linux amd64 public/private 交叉构建 PASS；public 文件为静态 ELF x86-64。
- Windows amd64 首次交叉构建 FAIL：公共导出路径正则仅定义于 Unix build 文件。将纯校验移动到跨平台文件后，public/private 交叉构建 PASS；public 文件为 PE32+ x86-64。
- Windows/Linux 原生运行没有执行，继续为 `NOT RUN`，不构成宿主生产支持证据。

## 自动化

| 验证 | 结果 |
| --- | --- |
| Go 全量单测 | PASS；app package 50.904 秒 |
| Go vet / gofmt | PASS；`go vet ./...` 退出 0，`gofmt -l .` 无输出 |
| Python validators | PASS；43 项，其中 distribution builder 4 项 |
| Draft 2020-12 | PASS；25 schemas、30 fixtures、31 个负例断言及既有持久 evidence |
| component 静态 verify | PASS |
| Apple Silicon trusted bundle smoke | PASS |
| candidate 独立 verify | PASS |
| 两次 candidate 重现性 | PASS |
| 分发具体模型 ID 扫描 | PASS；无匹配 |

## 仍未完成

- Codex 客户端真实安装、Skill 发现和包内 CLI 相对路径调用。
- 升级失败不切 active、卸载保留用户项目/凭据、回滚到上一已验证版本。
- quarantine/Gatekeeper 动态行为、默认网络/遥测/隐藏外部写入动态审计。
- GitHub-hosted required CI、干净环境最终闭环和独立只读终审。
- v1.0 最终版本号与正式发布；没有任何 remote、push、npm 或 Marketplace 写入。
