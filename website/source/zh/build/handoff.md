---
title: "将包交付给安装器"
description: "让包来源和证据跨越明确的安装边界。"
canonicalId: "page:build:handoff"
section: "build"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 将包交付给安装器

有效交付包含标准包目录，以及清楚的已检查事项。接收端安装器针对所选客户端和范围制定自己的计划。
源码创作工具可以提供证据，接收机器无需具备这些工具。

## 准备包与证据

选择含根目录 `plugin.json` 的文件夹，而不是存在歧义的父仓库。记录：

- 包身份、版本、精确源提交或审核过的快照。
- 相对包根目录，以及技能和 MCP 服务器的用途。
- 验证与检查结果，并附工具版本或源提交。
- 预期客户端 ID 和逐组件的兼容性结果。
- 运行时前提、认证要求和仍未评估的工作。
- 已执行的显式运行检查，包括服务器、工具和平台。

包版本与 CLI 版本表示不同对象，应分别记录。共享诊断时避免凭证和不必要的绝对路径。

## 可选的源码创作检查

以下命令需要[启用创作功能的源码构建](./#build-from-source)，当前安装器不包含这些命令：

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
agentplugins author compat ./review-helper --target codex
agentplugins author doctor ./review-helper
```

这组命令是静态检查，不运行包代码，也不检查真实用户的客户端配置目录。
如果执行过显式 [MCP 运行检查](./checks)，应单独记录。直接运行成功仍不能证明原生客户端发现或 OAuth。

## 请求已发布安装器制定计划

```bash
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

第一条提供安装器读取包的证据；第二条选择 Codex 并预览安装，不应用受管理的客户端安装变更。
两者都会应用安装器安全策略，必要时可能获取扫描器或元数据；本地包和 dry-run 不保证离线执行。
其他客户端应选择兼容性检查中确认的目标。项目计划还需 `--scope project --project-root /path/to/project`。

检查包身份、来源、路径、安全结果和剩余激活要求。与作者预期不一致时先解决差异，再应用。
计划不是已安装状态收据。

## 分享可重现的来源

远程交付可使用接收端 CLI 接受的源：带签名的 Directory 包、固定 GitHub 源，或受支持的直接 Git/归档源。
[安装来源](../use/install)说明语法与可用性。需要精确恢复时，使用不可变提交或持久的版本化归档。
本地目录需要另行明确转移。

源码创作 CLI 不提供包发布、bundle 导出或 Directory 提交命令。托管来源与验证来源是不同步骤。
`uap.toml` 依赖声明属于 UAP 管理器约定；根目录 `plugin.json` 仍是可移植包身份的权威来源。

## 说明剩余工作

| 已提供证据 | 仍需完成 |
| --- | --- |
| 标准符合性 | 说明的实用性和工具行为的正确性。 |
| 静态创作检查 | 发布批准和公共渠道验收。 |
| 静态客户端兼容性 | 实际原生发现和激活。 |
| 已执行的直接 MCP 检查 | 其他工具、客户端、认证和未测试环境。 |
| 安装器预览 | 应用已审核变更，并完成客户端设置。 |

接收者准备安装时可继续[使用插件](../use/install)。请随包保留客户端专用的信任和激活说明。
