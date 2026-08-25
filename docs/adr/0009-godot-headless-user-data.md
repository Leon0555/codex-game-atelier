# ADR 0009：Godot Headless 与 `user://` 明示授权

- 状态：Accepted（Phase 1 macOS Apple Silicon 薄切片）
- 日期：2026-08-25
- 决策范围：`validate --headless`、Godot 固定调用、外部写入授权、结构化结果与证据

## 背景

Godot 4.7.2 在 macOS 上会自动确保 `user://` 可写；默认位置是 `~/Library/Application Support/Godot/app_userdata/<project-name>`。官方 CLI 没有把该目录重定向到 Atelier 项目根的参数。自包含编辑器模式只控制编辑器数据，不能被当成导出项目或游戏 `user://` 的通用隔离保证。

直接启动会把标准引擎行为变成隐藏外部写入；完全禁止又无法获得真实场景、资源和 GDScript 加载证据。Phase 1 需要一个不会把任意参数、绝对路径或未授权副作用混入持久状态的窄契约。

## 决策

1. 保留 `validate --project <dir>` 的静态 baseline 行为，不默认启动 Godot。
2. 新增显式 `--headless`；它只执行固定序列：CLI 先打开并固定 Godot 与配套 runner 的源文件身份，为 version 分别创建私有 runner/engine 快照并检查受支持版本；删除并同步两个快照后，才从同一对源描述符创建独立的 scene runner/engine 快照。私有 runner 只从继承的 fd 读取项目目录、引擎快照和带随机 nonce 的有界控制消息；它先对项目目录执行 `fchdir`，再以固定参数把自身替换为 Godot。scene 参数只可能是 `--headless --path . --quit-after 1 --no-header`。Godot 不直接解析 `/dev/fd/3`，也不接收公开项目绝对路径。CLI 在引擎前后核对公开项目路径与 pinned 目录身份；并发项目路径替换不能重定向执行且会使 observation 作废，并发 Godot 或 runner 公共路径替换也不能改变已固定的源文件身份。不接受任意 Godot 参数。
   该操作会执行项目主场景中的 GDScript；`--headless` 本身就是对这一引擎限定执行的显式请求，不提供通用 shell/任意命令入口。调用者只能对自己拥有或已审阅的项目使用它。
3. `--godot`、`--timeout-ms` 和 `--allow-engine-user-data` 只在 `--headless` 下合法。
   Headless 默认总超时为 30 秒（仍可在 1–3,600,000 ms 内显式覆盖），因为 macOS 需要在同一总预算内为 version/scene 的 runner 与 engine 瞬时 Mach-O 快照进行本地 ad-hoc 重签名；`doctor` 的只读版本探测默认值仍为 5 秒。
   文件散列和 runner 复制按 1 MiB chunk 检查同一 deadline，子进程也使用该 deadline。不可中断的单次内核 clone/fsync/close 与失败清理仍可能使实际返回时间略晚于 deadline；超时分类保持 `FAIL`/6，不把这种尾部清理误报为 PASS。
4. 没有 `--allow-engine-user-data` 时，命令在启动 Godot 前以 `BLOCKED`/4 和 `ENGINE_USER_DATA_NOT_AUTHORIZED` 结束，并提交可审计 evidence。
5. 明示授权后，intent 使用符号值 `godot:user-data:standard-os-location` 记录外部写入范围；不持久化用户绝对路径。项目内写入仍只声明具体 run root。
6. 选中的可执行文件必须先自报可接受的 Godot 标准版 `4.7.2-stable` 标识。该文本只证明进程输出符合版本契约，不单独证明二进制来源；项目本地官方包的来源、散列和签名由安装/环境验证另行证明。版本不支持为 `BLOCKED`/4；进程失败或 Godot `ERROR:` 为 `FAIL`/5；总超时或取消为 `FAIL`/6；version/scene 任一阶段的 stdout 或 stderr 超过 256 KiB 时，截断优先于普通非零退出而拒绝信任并返回 `GODOT_OUTPUT_TRUNCATED`，但取消和超时优先级更高。
7. 原始 stdout/stderr 暂不持久化，避免把项目日志中的凭据或绝对路径写入 evidence。首个 payload 只记录严格、脱敏的检查结果；日志脱敏与保留契约另行实现。
8. macOS Apple Silicon 真实参考项目验证必须同时满足：精确版本通过、主场景至少完成一帧、进程退出 0、bounded stderr 没有 `ERROR:`、run/evidence 闭包原子提交。

## 取舍

- 明示授权保留了真实 Godot 语义，也让 CLI 在执行前可阻断隐藏外部写入；代价是用户首次运行 Headless 需要额外确认。
- 不修改子进程 `HOME`。这样不会用临时目录伪装真实玩家数据路径，也避免改变被测项目对操作系统目录的观察。
- CLI 与私有 runner 是同一预构建分发物中的配套二进制。runner 不提供公开 argv 命令，不从 argv 接受可执行文件或 Godot 参数；只有环境 nonce 与 fd 控制消息一致时才执行固定阶段。成功时以同一 PID `exec` 为快照，因此沿用外层进程组、超时、取消和输出上限。
- 通过 PATH、npm 或包管理器 symlink 启动公开 CLI 时，先解析 CLI 的真实文件路径，再在真实目录查找 sibling runner；不会错误地在 symlink 所在目录寻找内部组件。
- macOS/APFS 对大型 Godot engine 使用同卷 `fclonefileat` 创建写时复制快照；小于等于 64 MiB 的配套 runner 从固定 fd 做有界普通复制、fsync 和摘要验证，因此 CLI/Plugin 安装位置不必与项目同卷。复制 Mach-O 会失去原文件身份带来的可执行许可，所以两类瞬时快照都用固定 `/usr/bin/codesign --force --sign - --identifier org.codex-game-atelier.godot-snapshot` 做本机 ad-hoc 重签名。这不是游戏导出签名、公证或发布身份，也不使用账号、证书或 Keychain 私钥。Linux 对 engine 使用 1 GiB 上限、对 runner 使用 64 MiB 上限的本地复制且不执行该 macOS 签名步骤。快照与 codesign 临时文件位于尚未提交的 run root，阶段结束后先删除并同步目录，随后才允许提交 result。当前 macOS 实现只要求大型 Godot engine 与项目状态位于支持同卷 clone 的文件系统；无法安全创建/签名快照时返回 `GODOT_EXECUTABLE_SNAPSHOT_UNAVAILABLE`，不降级为重复解析公共路径。
- version 与 scene 对 runner 和 engine 都使用阶段独立快照，避免版本进程修改自身或配套 runner 后影响 scene。每个快照执行前后均以 SHA-256、大小、文件身份和修改时间复核；异常变化返回 `GODOT_EXECUTABLE_CHANGED`。只要任一快照/codesign 临时文件清理失败或仍存在，`result.json` 就不得发布，run 保持 incomplete 供后续恢复。该边界防止正常路径替换、更新竞态和阶段间自修改，但不宣称抵御能够定位并恶意篡改私有 run staging 的同一用户并发进程。
- 不持久化原始日志降低了诊断深度；当前用稳定错误码、阶段、退出状态和 checks 建立最小证据，后续 `logs` 命令再补受控日志。
- `--quit-after 1` 是场景启动薄切片，不等于完整玩法、输入、UI 或 GDScript 测试套件。
- Atelier 当前只能声明并控制自身 run 写入与 Godot 标准 `user://`；项目脚本主动访问网络、绝对路径或外部服务的行为属于项目代码，不由本薄切片沙箱化。严格模式在具备相应 containment 前不得把“没有观察到额外副作用”升级为保证。

## 回退

可以移除 `--headless` 分支并保留静态 baseline 与既有 committed runs。不得删除已生成的 Godot 用户数据目录；如需清理，必须先列出精确目录并单独获得用户确认。

## 官方依据

- [Godot 4.7 数据路径](https://docs.godotengine.org/en/4.7/tutorials/io/data_paths.html)
- [Godot 4.7.2 `OS::get_user_data_dir`](https://github.com/godotengine/godot/blob/4.7.2-stable/core/os/os.cpp)
- [Godot 4.7.2 macOS 数据路径实现](https://github.com/godotengine/godot/blob/4.7.2-stable/platform/macos/os_macos.mm)
