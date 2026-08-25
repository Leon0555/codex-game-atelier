# Phase 1 生产 Go CLI 首轮验证

后续 `initialize` 写入切片及其原子性审计见 `phase1-initialize-2026-08-25.md`。本文件保留首轮只读切片当时的证据，不把后续结果倒写为首轮已完成。

日期：2026-08-25（Asia/Shanghai）
主机：macOS Apple Silicon
范围：产品重命名、生产 `detect`/`doctor`/`status`、结构化结果、进程边界、严格状态读取和交叉构建形状

## 结果

| 检查 | 结果 | 证据与限制 |
| --- | --- | --- |
| 产品名扫描 | PASS | 正式仓库与父工作区 `AGENTS.md` 的旧完整产品名扫描结果为 0；用户可见名称已改为 Codex Game Atelier |
| Go 格式、静态检查、单测 | PASS | Go 1.27.0；`go fmt ./...`、`go vet ./...`、`go test -count=1 ./...` 退出 0 |
| Go race 检查 | PASS | `go test -count=1 -race ./...` 退出 0 |
| macOS Apple Silicon 构建 | PASS | `go build -trimpath ./cmd/codex-game-atelier` 退出 0；本机产物用于真实命令检查，不是发布 artifact |
| Linux x64 交叉构建 | PASS（有限） | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` 退出 0；只证明文件可生成，未在 Linux 运行 |
| Windows x64 交叉构建 | PASS（有限） | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build` 退出 0；Windows Job Object 代码可编译，未在 Windows 原生验证进程树 |
| `detect` 真实项目 | PASS | 对中文/空格路径的参考项目和项目本地 Godot 返回单一 JSON、exit 0；测试确认不会启动候选 executable |
| `doctor` 真实 Godot | PASS | 检出 `4.7.2.stable.official.ed1daf0bf`、标准版/GDScript、darwin/arm64，exit 0 |
| `doctor` 负向/安全 | PASS（本机有限） | 单测覆盖错误 patch、.NET/注释误报、非 Godot、非零退出、截断输出不可信且不泄露、超时，以及正常/异常退出后同一 Unix 进程组的精确 PID 清理；主动 `setsid` 脱组后代不在当前保证内 |
| `status` 严格读取 | PASS（测试 fixture） | 覆盖有效/未来 schema、超长 schema version、未初始化、重复 key、大小写别名、未知字段、非法 UTF-8、缺失/null required、int64 数学整数边界、canonical 时间戳、特殊文件、symlink 和跨宿主安全内部引用 |
| 结果/fixture JSON 语法 | PASS | Ruby JSON 解析 15 个 schemas、fixtures 和 Plugin manifest，退出 0；本地 Markdown 链接扫描通过 |
| Plugin/Skill 离线基础结构 | PASS | Ruby/Psych 解析 frontmatter 与 `openai.yaml`；Plugin manifest JSON 可解析；本轮文件保留为首个只读切片证据，当前能力变化见后续 initialize 验证记录 |
| 独立只读生产切片终审 | PASS（范围限定） | 最终复审无 Blocker/High/Medium 代码缺陷，接受为 Phase 1 首个只读生产代码切片；不代表 v1 发布或三宿主生产级支持 |
| Reference Game 品牌输出 | PASS（有限） | Godot Headless 输出 `atelier_reference_ready` 和中文路径 payload，exit 0 |
| Reference Game 干净外部写入 | BLOCKED | Godot 仍尝试 macOS `~/Library/Application Support/Godot`，沙箱阻止并输出 ERROR；不能声明零外部写入通过 |
| Draft 2020-12 完整语义验证 | BLOCKED | 仍未安装 `jsonschema` 等 validator；JSON 语法与 Go 不变量测试不能替代完整 schema validator |
| 原子 evidence 写入 | NOT RUN | 本首轮三命令刻意保持零文件写入；后续 `initialize` 只实现单文件 state，artifact/evidence/run/state 的多文件提交与恢复仍未实现 |
| Plugin 安装/官方 validator | NOT RUN / BLOCKED | 未执行用户级安装；`quick_validate.py` 与 `validate_plugin.py` 均因项目环境没有 PyYAML 而退出 1，未自动安装 |

## 已验证的不变量

- 子命令 stdout 只写一个 command-result JSON；`PASS` 为 exit 0 且无 errors，`BLOCKED` 为 exit 4，`FAIL` 使用对应非零类别。
- `detect` 只发现项目、候选 executable 和宿主，不运行 Godot。
- `doctor` 只执行固定参数 `--version`，精确接受官方 `4.7.2.stable.official.<7..40 hex>`；任一输出流截断即失败，不把任意或截断前缀当作 Godot。
- 受控进程输出有界；失败输出不回显到结果，避免把任意日志或凭据带入结构化输出。
- Unix 在正常、异常、超时或取消后都会同步终止为 Godot 创建的独立进程组；同组子进程无论持有还是关闭输出管道，精确 PID residual 测试均通过。主动调用 `setsid` 脱组的后代不属于当前保证；leader 回收后立即 killpg 仍有极小的 PID/PGID 复用理论风险。
- `status` 不跟随引用、不隐式修复、不迁移、不写状态；`.gameatelier` 和 `project.json` 符号链接会被拒绝。未来 schema 只进入 `observed_schema_version`，不会冒充当前 `schema_version`。

## 未完成与下一步

1. 在原生 Windows x64 验证 Job Object 的分配、超时、取消和无残留；在 Linux x64 运行同一测试矩阵。
2. 冻结并实现原子 evidence：`artifact -> evidence record -> run result -> optional state index`，逐文件临时文件 + rename，run 最后成为正式引用点。
3. 为 Draft 2020-12 schema 与 Plugin/Skill 官方 validator 安装最小项目本地验证依赖前，另行确认依赖范围。
4. 通过 CLI 驱动真实 Godot Headless；当前生产 `doctor` 只验证版本，不声称完成场景/资源或 export template doctor。
5. 后续 `initialize` 薄切片已完成独立只读代码、安全和可恢复性审计；详见同日 initialize 验证记录。下一次审计门禁随新的写入/引擎执行切片重新触发。
