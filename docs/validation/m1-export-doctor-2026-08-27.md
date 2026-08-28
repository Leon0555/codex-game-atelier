# M1 export doctor 验证记录

日期：2026-08-27
宿主：macOS Apple Silicon
范围：只读 `doctor --export`、Godot 4.7.2-stable 宿主模板检查、Schema 与回归测试

## 结果

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 默认 doctor 保持轻量 | PASS | 未传 `--export` 时第六项 `export_templates` 明确为 `SKIPPED`，不把模板安装变成普通检测前置条件 |
| macOS 模板正例 | PASS | 实际命令 run `atelier-20260827t134355.563113000z-3be33da1fde0`；Godot 自报 `4.7.2.stable.official.ed1daf0bf`，模板版本 `4.7.2.stable`，`macos.zip` 与 `icudt_godot.dat` 为有界常规文件 |
| 缺失/错版本负例 | PASS | Go 单元测试分别验证缺失模板与 `version.txt` 不匹配，均返回 `BLOCKED`/4、`GODOT_EXPORT_TEMPLATES_MISSING` |
| 参数与结构化结果 | PASS | 新 flag 使用严格重复/尾随参数规则；doctor data 固定六项检查及 export-template 状态对象 |
| Go 回归 | PASS | `go test ./...`、`go vet ./...` 退出 0 |
| Schema | PASS | Draft 2020-12：18 schemas、20 fixtures、7 个已持久化 Starter Template records、24 个负例断言 |
| 打包/模板 validators | PASS | Python `unittest` 26 项通过，含 Plugin bundle structural/native smoke |

## 实机命令边界

仓库内原始下载的 headless Godot 文件在 macOS 上直接执行会因本地签名状态被系统终止。与既有 Headless 验证一致，本次使用已有的临时 ad-hoc 签名 Godot 副本执行固定 `--version`；为了让只读 template locator 看到已批准安装在项目 `.tools/godot/4.7.2/editor_data/export_templates/` 下的模板，测试期间在该临时副本旁创建一个指向上述目录的临时 `editor_data` symlink，命令完成后立即移除。没有修改用户级 Godot 数据目录。

这证明当前 locator 与真实 Godot/模板集合能闭合。后续 `build`/`export` 实机结果另见 [`m1-macos-export-build-2026-08-27.md`](m1-macos-export-build-2026-08-27.md)；本记录本身仍不证明产物 smoke、Windows/Linux 原生执行或普通用户 Godot 安装布局已经通过。

## 未覆盖

- 本次 doctor 记录没有执行 `export`/`build`；相关实现和 artifact evidence 是后续独立验证。
- 本次 doctor 记录未执行 target smoke；后续最终 Debug/Release 命令已在独立记录中完成。
- Windows x64、Linux x64 原生模板检查按已批准路线延期。
