# M3 rc.5 Plugin prompt 长度实测与修复

- 日期：2026-09-08（Asia/Shanghai）
- 候选版本：`0.3.0-rc.5`
- 源码提交：`e5672b5c747b7f5683c9a31848c2eecdc0dfd0ed`
- Marketplace ref：`marketplace/v0.3.0-rc.5`
- Marketplace revision：`ea66ac21bbb5e5b8a364acaa4e221da0c71c506f`
- 结论：**rc.5 拒绝；修复后重建 rc.6**

## 已通过证据

- 两次独立构建的二进制、Plugin bundle 与 distribution candidate 逐字节一致。
- `DISTRIBUTION-MANIFEST.json` SHA-256：`c1d1632dca0f1405e17041d7d8ef6154803f9c33238dedc7141f639b6c4b90ad`。
- Plugin archive SHA-256：`d586cc9be66f213dab03560e678f3f2ecfbb821c0696a6a497a71a8d5765d50f`。
- 源码和安装 cache 的 `defaultPrompt` 数量均为 3；新增数量门禁与 53 项 Python validators PASS。
- main CI run `34146859477` 与 Marketplace CI run `34147222046` 均为 `success`。
- 全新隔离 `CODEX_HOME` 从 GitHub sparse 获取并安装 rc.5；安装 cache 与 bundle A 逐文件一致，Plugin 列表显示 `0.3.0-rc.5` active。

## 拒绝原因

在隔离 `CODEX_HOME` 启动 `--ephemeral --sandbox read-only` 新会话时，Codex CLI 在任何模型响应之前多次报告：`interface.defaultPrompt[2]` 必须最多 128 字符。rc.5 合并后的第 3 条提示为 137 个 ASCII 字符，因此仍被客户端忽略。

隔离环境没有复制用户凭据，随后模型请求返回 `401 Unauthorized` 是预期的认证隔离结果；它不影响发生在本地 manifest 解析阶段的长度 warning，也不能用来宣称 Skill 模型响应验证 PASS。rc.5 在 Godot E2E、strict 和正式最终审计前主动拒绝。

## 修复与回归要求

1. 把第 3 条提示缩至 111 字符，同时保留 validate、固定 GDScript test、授权和结构化结果语义。
2. 打包器在数量与非空检查后，拒绝任何超过 128 个字符的 `defaultPrompt`。
3. 回归测试加入 129 字符负例。
4. 经受保护 PR/required CI 后，从新 clean `main` 构建 rc.6。
5. rc.6 重新执行远程安装；先以本地客户端启动日志证明 Atelier manifest warning 为零，再在有认证的新会话验证 Skill 发现。

rc.5 不创建 tag、Release 或正式 Plugin 发布。
