# ADR 0006：Phase 1 最小 CLI 命令契约

- 状态：Accepted（Phase 1 生产基线，尚非 v1 稳定兼容承诺）
- 日期：2026-08-25
- 决策范围：生产 CLI 名称、`detect`、`doctor`、`status`、默认副作用与首批错误语义

## 背景

ADR 0004 已冻结 Go 为 v1 CLI 生产实现语言，ADR 0005 定义通用结果、状态与 evidence 边界，但尚未固定首批命令的输入和 `data` 形状。直接从 Spike 复制会保留错误命令名、宽松版本匹配、尾随参数、输出泄露和非 Windows 进程树控制等已知缺陷。

用户已授权进入 Phase 1 生产实现。首批只实现能够验证契约的只读垂直路径，不提前铺开完整适配器。

## 决策

### 程序和协议

1. 生产可执行文件名为 `codex-game-atelier`，Go module 位于 `packages/cli`。
2. 每个子命令向 stdout 写且只写一个符合 `command-result.schema.json` 的 JSON 对象；机器不得解析 `summary` 推断结果。
3. `--version` 是独立的纯文本探测，不与子命令 JSON 协议混用。
4. 首批命令默认零文件写入，`evidence` 返回空数组。不得把空数组描述为 evidence 闭环已经实现。
5. 产品状态根目录采用 `.gameatelier/`。更早的状态根草案从未发布，不作为兼容输入。
6. `detect`、`doctor`、`status` 的非 usage 结果 `data` 分别由独立 schema 约束，并由 command name 与退出码条件引用；参数错误统一返回空对象。内部 state reference 使用 `.gameatelier/` 开头的受限小写 ASCII 路径，避免宿主间解释差异。

### `detect`

输入：`--project <dir>`，默认当前目录；可选 `--godot <file>`。

- 只读解析项目根、`project.godot`、Godot 候选来源和宿主。
- 不启动 Godot，不读取或创建 `.gameatelier`。
- 自动发现顺序固定为：`CODEX_GAME_ATELIER_GODOT`、PATH 中的 `godot`、PATH 中的 `godot4`、平台已知位置。
- 缺少项目、Godot 或受支持宿主时返回 `BLOCKED`/4。
- `data` 包含 `project`、`godot` 和 `host`。

### `doctor`

输入：与 `detect` 相同，另有 `--timeout-ms`，默认 5000，范围 1–3,600,000。

- 先执行与 `detect` 相同的纯读检查，再只受控执行已解析的 Godot executable 及固定参数 `--version`。
- 只接受选中进程自报的标准版标识 `4.7.2.stable.official.<7..40 hex>`；该文本门禁不单独证明二进制来源，官方包身份由安装来源、散列和平台签名验证另行证明。Godot .NET/C# 和伪前缀不进入 v1 基线。
- 首批 checks 为 `host`、`project_file`、`project_language`、`godot_executable`、`godot_version`；不声称已检查 export templates 或完整项目加载。
- Godot 非零退出返回 `FAIL`/5；超时或取消返回 `FAIL`/6；不受支持版本返回 `BLOCKED`/4。
- 任意 Godot stdout/stderr 不直接进入结果；只有未截断输出中的完整白名单版本字符串可以进入结果。任一输出流截断都返回 `FAIL`/5，不能用截断前缀判定版本，避免误判或泄露凭据。

### `status`

输入：`--project <dir>`，默认当前目录。

- 只读且严格解析 `.gameatelier/project.json`，拒绝未知字段、重复 JSON key、超过 1 MiB 的文件、符号链接状态路径和不安全引用。
- 未初始化返回 `BLOCKED`/4；读取、schema 或状态错误返回 `FAIL`/7。
- 首批只返回状态摘要和引用计数，不跟随 task/run 引用，也不隐式修复或迁移。

### 错误和退出码

- 2：用法/参数错误。
- 4：缺失或不受支持的前置条件。
- 5：Godot 受控进程失败。
- 6：超时或取消。
- 7：状态读取、schema 或安全校验失败。
- 8：框架内部或结果编码失败。

底层进程退出码只进入 `data` 或错误 details，不替代 Atelier CLI 退出码。错误码由生产代码集中构造和测试，不从自然语言推断。

## 备选方案

### 直接复制 Go Spike

拒绝。Spike 没有生产单元测试、状态读取、严格参数、输出脱敏、Windows Job Object 或 evidence 语义。

### 首批同时公开 evidence 写入标志

拒绝。artifact、evidence record、run result 和 state index 的失败恢复与提交顺序尚未完成实证。首批保持零写入和空引用。

### `detect` 通过执行 `--version` 验证候选

拒绝。发现与执行语义应分离；只有 `doctor` 启动受控外部进程。

## 风险

- Windows Job Object 当前只能交叉编译，未在 Windows 原生宿主验证。
- Windows `Start -> AssignProcessToJobObject` 之间仍有理论逃逸窗口；原生验证前不声明完整 Windows process-tree containment。
- Unix 只保证清理 CLI 为 Godot 建立的同一进程组；主动 `setsid` 脱组的后代不属于首批保证。
- Unix 在 leader 已被同步回收后立即清理残余进程组；仍存在极小的 PID/PGID 瞬时复用理论风险，后续若扩大到不可信任意工具执行需采用更强 supervisor，而不是扩大当前声明。
- Linux x64 产物当前也只有交叉构建形状，未运行。
- `doctor` 首批不是完整 Godot doctor；export templates、headless 项目加载和用户数据目录边界仍在后续薄切片中实现。
- 状态读取实现先于状态写入；`initialize` 和原子 evidence 提交仍未实现。
- 绝对项目路径会出现在即时 JSON 结果中；在未来持久化前必须定义相对化和脱敏规则。

## 迁移与回退

Phase 1 期间若 Godot 薄切片推翻 `data` 字段或错误语义，必须更新本 ADR、schema、tests 和调用方。对外 v1 兼容冻结后，破坏性变化使用新协议/schema 版本，不静默重写。当前回退方式是移除未发布的 `packages/cli` 产物；Rust Spike 不作为生产回退路径。
