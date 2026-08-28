# ADR 0015：macOS 技术导出、Build 薄封装与项目快照

- 状态：Accepted（M1 macOS Apple Silicon 生产薄切片；Debug/Release target smoke 已实证）
- 日期：2026-08-27
- 决策范围：`export`/`build` 公共参数、Godot 固定执行链、项目源写入隔离、Universal 2 artifact 与 evidence

## 背景

`doctor --export` 已能只读检查与 Godot `4.7.2-stable` 匹配的 macOS export templates，但尚不能证明实际导出。Godot 导出还可能生成 `.godot/`、`.uid` 等项目数据；如果 CLI 只声明 `.gameatelier/runs` 与 `.gameatelier/artifacts`，却让引擎直接修改源项目，run intent 就会与真实副作用冲突。

用户已确定 v1.0 只验证 macOS Apple Silicon 技术导出，生成 Universal 2，但不做 Intel 实机 smoke，也不要求游戏产物签名或公证。

## 决策

### 公共命令

1. `export` 接受 `--project`、`--profile debug|release`、固定 `--preset "macOS Technical"`、可选 `--godot`、`--timeout-ms` 和显式 `--allow-engine-user-data`。
2. `build` 是普通用户入口，接受相同参数但不开放 preset 选择；内部固定选择 `macOS Technical`。
3. 两个命令使用同一个执行函数、相同 Godot runner、模板快照、项目快照、artifact 检查和 `export-artifact` evidence。`build` 不是第二条引擎流水线。
4. 当前唯一 target 是 `macos-universal2`。非 macOS Apple Silicon 宿主返回 `EXPORT_HOST_NOT_VERIFIED`/4，不从交叉构建推断原生支持。

### 固定 Godot 执行与副作用

1. CLI 先验证 standard/GDScript、固定 preset、Godot 自报版本和匹配 export templates；普通 `build`/`export` 自行执行门禁，不依赖用户预先运行 `doctor`。
2. private runner 只接受 `export-debug` 或 `export-release`，固定调用 `--headless --path . --no-header --export-{profile} "macOS Technical" .atelier-output/game-{profile}.zip`；不接受任意 Godot 参数或输出路径。
3. runner、Godot executable 和当前宿主所需模板从已打开源描述符复制进 run 内临时运行时，并在前后验证摘要。
4. 项目源复制进 run 内 `.godot-project-snapshot` 后才交给 Godot。根 `.gameatelier`、`.godot`、`.git` 不复制；保留名 `.atelier-output`、symlink、特殊文件、超过 4,096 项、深度 64、单文件 512 MiB 或总计 1 GiB 均阻断。每个文件复制前后核对身份、大小和修改时间。
5. Godot 只把 ZIP 写到快照内 `.atelier-output`。CLI 再以有界复制发布到声明的 `.gameatelier/artifacts/<run-id>/game-{profile}.zip`。所有临时项目/引擎/模板/runner 内容必须在 `result.json` 提交前清理；清理失败让 run 保持 incomplete。
6. `--allow-engine-user-data` 只授权 Godot 标准 OS `user://` 路径，并在 intent 中符号化记录；它不授权修改源项目、网络访问或任意绝对路径写入。导出仍应只用于用户拥有或已审阅的项目，当前不沙箱化项目代码主动发起的外部副作用。

### Artifact 与证据

1. 产物必须是 1 byte 至 4 GiB 的常规 ZIP，最多 4,096 个成员、单成员展开不超过 512 MiB、总展开不超过 1 GiB，且成员路径不能穿越。
2. ZIP 必须恰有一个 `.app/Contents/MacOS/` 主可执行文件。fat Mach-O 必须恰含 `x86_64` 与 `arm64` 两个 slice；fat table、slice 范围、重叠、64-bit thin magic 和 CPU type 必须一致。只看文件名或 preset 不足以声称 Universal 2。
3. PASS evidence 记录目标、profile、preset、Godot 版本、项目相对 artifact 路径、SHA-256、字节数，以及 `unsigned=true`、`not_notarized=true`、`public_distribution_ready=false`。
4. PASS 前必须从刚生成的 ZIP 安全解包目标 `.app`，在 Apple Silicon 通过固定 private runner 执行 `--headless --quit-after 1 --no-header`。退出 0、输出未截断、无 `ERROR:`、runner/进程组/解包目录全部清理后，manifest 才记录 `target_smoke={host:macos,arch:arm64,mode:headless-one-frame,exit_code:0}`。
5. 非 PASS 结果不得发布 artifact metadata。run intent 同时声明 run root、artifact root 和获准的标准 `user://` 外部写入；result 最后提交。

## 备选方案

### 让 Godot 直接在源项目导出

拒绝。实机运行生成了未声明的 `.uid` 文件，证明只声明 `.gameatelier` 写入与实际行为不一致。

### 由 `build` 再实现一套 Godot 调用

拒绝。它会产生两套参数、错误和 evidence 语义，增加漂移而没有用户价值。

### 只验证 ZIP 中存在 `.app` 文件

拒绝。单架构或伪造文件也能满足路径检查，不能支持 Universal 2 声明。

### 在 v1.0 强制签名、公证或 Intel smoke

拒绝。超出已冻结范围；技术导出必须明确不是公开分发就绪产物。

## 风险与剩余范围

- 项目快照能隔离常规相对项目写入，但不是恶意代码沙箱；项目脚本、editor plugin 或引擎仍可能主动访问网络或绝对路径。
- 逐文件稳定复制不等价于原子文件系统快照；并发新增/删除跨文件集合仍可能形成逻辑混合版本。当前建议从静止、已审阅工作树导出，后续 target smoke/干净环境门禁会继续验证。
- Debug build 与 Release export 的 `.app` 已在 Apple Silicon 完成 headless 一帧启动/退出；这不是窗口化长时游戏测试、性能测试或 Intel smoke。
- Intel slice 只做静态结构验证，不做 Intel 实机 smoke。签名、公证和公开分发 readiness 不属于 v1.0。

## 迁移与回退

当前未发布。若后续新增 target，必须先增加固定 target/preset、模板需求、artifact 检查和原生 evidence；不得把自由输出路径或任意 Godot argv 加入现有命令。若项目快照边界无法覆盖真实项目，应以新 ADR 扩大明确授权或改用更强只读工作树快照，不得回退为隐藏源写入。
