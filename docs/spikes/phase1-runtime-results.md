# Phase 1 Runtime 与 Godot 薄切片初步结果

状态：进行中；Rust/Go 语言对照已完成，原子 evidence、Plugin 安装验证与 Godot 写入边界仍待处理
日期：2026-08-24；更新：2026-08-25
主机：macOS 26.6.2，Apple Silicon arm64

## 安装与来源验证

| 项目 | 结果 | 证据 |
| --- | --- | --- |
| Go 1.27.0 | PASS | 官方 archive SHA-256 通过；`go version go1.27.0 darwin/arm64`；项目本地安装约 271 MiB |
| Godot 4.7.2 standard | PASS | 官方 SHA-512 通过；`4.7.2.stable.official.ed1daf0bf`；在沙箱外验证为 `Notarized Developer ID` |
| Godot export templates | PASS | 官方 SHA-512 通过；`version.txt` 为 `4.7.2.stable`；35 个模板文件，自包含 Godot 目录合计约 2.3 GiB |
| Rust 1.98.0 | PASS | rustup 与通道清单 SHA-256 通过；`rustc 1.98.0`、`cargo 1.98.0`，arm64；项目本地安装包含 cargo、rustc、rust-std、rustfmt、clippy 与离线 docs，约 1.3 GiB |

Godot 签名最初在命令沙箱内误报 invalid；同一沙箱也把系统 Calculator 误报为不受信任。在沙箱外复验后，Godot app 的磁盘签名和 Notarized Developer ID 均有效。这是验证环境差异，不是通过重签名或绕过 Gatekeeper 修复。

## Go CLI Spike

样品位置：`spikes/cli-runtime/go/`；产物只在忽略的 `.tools/spike-bin/`，不进入产品包。

| 检查 | 结果 |
| --- | --- |
| 首次 release build | 5.63 s |
| 热 build | 0.17 s |
| stripped binary | 2,752,690 bytes，Mach-O arm64 |
| 两批各 25 次 `--version` | median 4.451–9.030 ms；观测 min 3.146 ms、max 30.917 ms（最终二进制；本机负载可见） |
| `go vet` | PASS |
| module compile/test | PASS（当前无 Go 单元测试文件） |
| 中文/空格 symlink 项目 doctor | PASS；Godot `4.7.2.stable`；exit 0 |
| 破损项目路径 | BLOCKED；`GODOT_PROJECT_NOT_FOUND`；exit 4 |
| 10 ms 受控超时 | FAIL；`GODOT_TIMEOUT`；exit 6；约 11 ms 返回 |
| 结构化结果基础断言 | PASS；JSON、outcome/exit/errors/version 相互一致 |
| 本地代码签名状态 | Go linker ad-hoc；`codesign --verify --strict` PASS；不是发布签名 |

第一次 Go build 从仓库根运行，因 module 位于子目录而失败；改用 `go -C spikes/cli-runtime/go` 后成功。该命令差异保留为分发脚本设计证据。

最初 Go timeout 只终止 shell 父进程，受控 `sleep` fixture 仍持有输出管道，10 ms 用例实际耗时约 2017 ms。样品随后改用 Unix 独立进程组、显式有界内存管道和组级终止。第二轮独立审阅又加入“父进程先退出、后台子进程持有管道”fixture；最终 10 ms timeout 的 CLI 墙钟约 18–20 ms，父先退出用例约 16–23 ms。主验证在 `--timeout-ms 100` 下连续 30/30 PASS；独立审阅同参数曾出现一批 29/30、随后 30/30，而 `1000 ms` 下两种实现均稳定 PASS 且无残留。因此 100 ms 数据只用于调度敏感的 Spike 观察，不作为发布门禁。Windows 进程组仍需 Job Object 实证，当前 `process_other.go` 只保留可编译的父进程降级行为。

## Rust CLI Spike

样品位置：`spikes/cli-runtime/rust/`；产物只在忽略的 `.tools/spike-bin/`，不进入产品包。

| 检查 | 结果 |
| --- | --- |
| 首次 release build | 113.65 s 墙钟；包含 crates.io 索引与 17 个兼容包下载 |
| 热 release build | 0.09 s；`--locked --offline` |
| stripped binary | 520,512 bytes，Mach-O arm64 |
| 两批各 25 次 `--version` | median 4.298–6.822 ms；观测 min 2.575 ms、max 14.380 ms（最终二进制）；较早的预修复样品首批曾有一次 431.671 ms 冷启动离群值 |
| `cargo fmt --check` | PASS |
| `cargo clippy -D warnings` | PASS；locked/offline |
| module compile/test | PASS（当前 0 个 Rust 单元测试） |
| 中文/空格 symlink 项目 doctor | PASS；Godot `4.7.2.stable`；exit 0 |
| 破损项目路径 | BLOCKED；`GODOT_PROJECT_NOT_FOUND`；exit 4 |
| 10 ms 受控超时 | FAIL；`GODOT_TIMEOUT`；exit 6；修复后 13 ms 返回 |
| 结构化结果基础断言 | PASS；JSON、outcome/exit/errors/version 相互一致 |
| 本地代码签名状态 | Rust linker ad-hoc；`codesign --verify --strict` PASS；不是发布签名 |

Rust 的首个 timeout 样品同样只杀父进程，实际耗时约 2020 ms。修复版在 Unix 创建独立进程组并终止整组，失败时回退到标准父进程终止；第二轮又让 deadline 约束管道 drain，并在父进程正常退出后清理同组残留。最终 10 ms timeout 约 17 ms，父先退出用例约 20 ms。这引入少量 `cfg(unix)` 与 FFI/unsafe 边界。非 Unix 目前只有父进程降级，Windows 仍需独立实现和实机验证。

两份样品还通过了以下负向回归：`--timeout-ms` 只接受 1–3,600,000；空进程错误生成非空 fallback；错误消息最多 2048 字符且读取继续 drain、内存捕获有界；`/usr/bin/true` 不再被误认作 Godot；只有以 `4.7.2.stable` 开头的版本输出在本 Spike 中通过。这里仍只承诺**同一 Unix 进程组**：主动 `setsid`/脱组的后代无法由 `killpg` 清理，生产实现与 Windows Job Object 都需后续专项设计。

## Runtime 对照与推荐

| 维度 | Go | Rust |
| --- | --- | --- |
| 本机 stripped binary | 2,752,690 bytes | 520,512 bytes |
| 热启动中位数 | 4.451–9.030 ms | 4.298–6.822 ms |
| 冷构建 | 5.63 s | 113.65 s（含索引/依赖下载） |
| 热构建 | 0.17 s | 0.09 s |
| 第三方依赖 | 0；标准库 | 3 个直接、17 个锁定包 |
| Unix timeout/process-group cleanup | 标准库 + OS 文件 | 标准库 + OS FFI/unsafe |
| 无额外 SDK 交叉构建 | macOS x86_64、Linux x86_64、Windows x86_64 产物均生成 | NOT RUN；只安装 arm64 macOS target |

**决策：Go 正式作为 v1 确定性 CLI 的生产实现语言。** 用户于 2026-08-25 审阅并批准，ADR 0004 已冻结该选择。理由不是启动速度（两者相当），而是零第三方运行时依赖、维护者工具链更小、Tier 1 预构建矩阵更直接。Rust 的显著优势是二进制约小 2.2 MiB、强类型表达和优秀的本地工具链；Rust 样品保留为研究证据，不进入生产路径。未来更换语言需要新 ADR 和用户批准。

Go 交叉产物仅验证文件形状：macOS x86_64 Mach-O 2,903,312 bytes、Linux x86_64 静态 ELF 2,920,608 bytes、Windows x86_64 PE 3,025,920 bytes。它们未在 Intel Mac、Linux 或 Windows 实机运行，不属于 Support Matrix PASS。

## 尚未实现的 Spike 用例

计划中的“把 evidence 原子写到指定项目相对路径，并拒绝越界”尚未实现：两份样品当前 `evidence` 都为空，也没有 `--evidence` 路径、临时文件 + rename、失败恢复或路径逃逸测试。因此本轮只把**运行时语言对照**标为完成，完整 Runtime/Plugin Spike 继续进行；该项是 `NOT RUN`，不得把空数组解释为 evidence PASS。

## Godot headless 与路径薄切片

最小项目成功导入并执行 `main.tscn`，从 `res://中文 资源/status_payload.gd` 加载脚本并输出：

```json
{"event":"atelier_reference_ready","path_fixture":"中文与空格路径已加载","status":"PASS"}
```

进程退出 0，但不能把整条运行称为 clean PASS：

- `_sc_` 成功把编辑器 settings/cache/templates 放在 `.tools/godot/4.7.2/editor_data`。
- macOS 上项目 `user://` 仍尝试写 `~/Library/Application Support/Godot/app_userdata/...`。
- 当前命令沙箱阻止该写入并产生 Godot ERROR；`--log-file` 只能重定向日志，不能重定向 `user://`。
- 硬链接 headless binary 与进程级 `CFFIXED_USER_HOME` 都未改变该行为，不能作为产品方案。

因此场景/中文资源执行为 PASS，“无仓库外写入尝试的 clean headless run”保持 BLOCKED。后续要么把 Godot 标准 user-data 写入变成明确授权和证据字段，要么找到 Godot 官方支持的隔离方式；不得设置或伪造用户 `HOME`。

## macOS 技术导出

第一次 export 因 Universal 2/arm64 要求 ETC2 ASTC 导入而退出 1；在项目启用 `rendering/textures/vram_compression/import_etc2_astc=true` 后，同一 Release export 成功。

| 检查 | 结果 |
| --- | --- |
| Release ZIP | PASS；约 57 MiB；SHA-256 `f31b3944e0ca7053faca069103eafad534caba1f6e0e0ff38d419b3aa8904821` |
| 架构 | PASS；`x86_64 arm64` Universal 2 |
| Apple Silicon 进程启动 | 功能 PASS；输出 reference JSON、exit 0；仍有上述 `user://` ERROR |
| Developer ID 签名 | NOT RUN/不要求；preset 明确 `codesign/codesign=0` |
| 公证 | NOT RUN/不要求 |
| 当前 app 签名验证 | FAIL（预期）；模板内嵌签名在导出修改后无效，不可视为签名产物 |
| 公开分发就绪 | false |

该产物只证明本机技术导出、Universal 2 形状和 Apple Silicon 运行路径；不证明 Intel 实机、Gatekeeper 下载场景、Developer ID 或公证。

## 当前阻塞与下一步

1. `.tools` 最终快照约 6.2 GiB；用户已确认真实工具需要时可超过原 5 GiB 估算，当前以 8 GiB 作为异常增长软告警而非功能门禁。约 1.4 GiB 已校验下载包和约 337 MiB Godot 解压核验目录属于可回收项；未经用户逐项确认不删除。
2. Go 生产实现语言已冻结；原子 evidence、Plugin 携带/调用和三个目标宿主实际运行仍未完成。
3. Plugin/Skill 官方 validator 仍缺 repo-local `PyYAML`；Draft 2020-12 fixture 验证仍缺 `jsonschema` 等价实现。
4. Godot macOS `user://` 外部写入语义需要单独契约与明确测试策略。

## 官方依据

- [Godot 4.7.2 官方归档](https://godotengine.org/download/archive/4.7.2-stable/)
- [Godot 4.7 macOS 导出](https://docs.godotengine.org/en/4.7/tutorials/export/exporting_for_macos.html)
- [Godot 数据路径](https://docs.godotengine.org/en/4.7/tutorials/io/data_paths.html)
- [Go 官方下载](https://go.dev/dl/)
- [Rust 官方发布](https://blog.rust-lang.org/releases/)
