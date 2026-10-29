---
title: "检查包与证据"
description: "分别理解静态检查、显式 MCP 运行和客户端激活证据。"
canonicalId: "page:build:checks"
section: "build"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 检查包与证据

**源码创作：** 以下命令要求[启用创作功能的源码构建](./#build-from-source)。当前安装器版本不包含 `agentplugins author`。

从包的父目录运行相对路径示例，并始终选择同一个明确的包根目录。

## 运行离线创作检查

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

validate 检查捕获的标准配置和创作就绪情况；inspect 显示组件与未解决的要求。
不加运行参数的 test 静态检查配置、目录卫生、技能和 MCP。这些命令不启动服务器，也不执行技能说明。

## 分层理解证据

| 层次 | 能证明什么 | 需要另行检查什么 |
| --- | --- | --- |
| 符合性 | 捕获的标准数据满足内置 schema 和约定。 | 在具体客户端中的实际用途。 |
| 主机安全 | 输入满足有界的文件系统和主机检查。 | 用户对安装或执行的授权。 |
| 就绪情况 | 已有证据满足所检查的策略。 | 公共发布批准和平台验收。 |
| 兼容性 | 所选目标和组件的静态支持。 | 真实客户端的有效发现和激活。 |
| 工具链 | 有界的项目、原生文件和可执行文件元数据。 | 依赖解析和服务器启动。 |
| 运行行为 | 显式执行的 MCP 检查观察到指定阶段。 | 其他工具、客户端、OAuth 或未测试环境。 |

分别查看符合性和总体就绪情况。清单有效仍可能缺少组件或主机证据。
保留“未评估”状态，不应将它转为通过。

## 明确指定客户端

```bash
agentplugins author compat ./review-helper --target codex,claude
agentplugins author inspect ./review-helper --target codex,claude
agentplugins author doctor ./review-helper
agentplugins author capabilities
```

compat 要求不重复、逗号分隔的目标 ID，不接受 `all`。这些静态检查不要求客户端已安装。
doctor 在不启动进程、不联网的情况下检查捕获的原生文件和可执行文件元数据。
capabilities 报告此引擎的 schema、客户端元数据和命令，不认证包，也不代替公共发布验收。

## 显式检查 MCP 运行行为 {#evaluate-mcp-runtime-explicitly}

stdio 执行和开发循环当前仅支持 Linux，需要系统 bubblewrap（`bwrap`）及成功的进程约束（需要委派的 cgroup v2）和命名空间预检。
Node 解析路径须位于 `/usr`、`/bin`、`/lib` 或 `/lib64` 下的受支持系统运行时目录；不符合时拒绝执行。
这些要求属于本地 stdio 路径，不适用于独立的 Streamable HTTP 连接检查。

先为 [stdio 教程](./mcp-stdio)生成的包单独安装识别到的锁定依赖，再从 `mcp.json` 选择精确服务器名：

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

此源码命令执行所选 MCP 进程，检查初始化和工具发现。
调用工具时必须同时提供 `--tool` 和 `--fixture`。参数文件相对于包根目录，内容为该工具的 JSON 对象参数。
为模板 `hello` 工具创建内容为 `{}` 的 `fixtures/hello.json`，再运行：

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools \
  --tool hello --fixture fixtures/hello.json
```

Streamable HTTP 需要显式允许联网：

```bash
agentplugins author test ./docs-service --runtime mcp --server docs-service \
  --allow-network
```

使用您包中的实际服务器标识。默认期限为 10 秒；`--deadline` 可设置不超过一分钟的正时长。
分别检查初始化、工具、可选工具调用和清理证据。这是在创作工具中直接执行，不能证明通过原生客户端运行、OAuth 完成，或其他工具和公共部署可用。

## 使用源码开发循环

```bash
agentplugins author dev ./local-tools --server local-tools
```

dev 在包变更时运行有界 MCP 周期；按 Ctrl+C 停止。一次执行用 `--once`，JSON 输出也要求它。
远程服务器需要 `--allow-network`，工具调用仍要求明确的工具与参数文件。此命令不向原生客户端安装。

## 保存可审核报告

```bash
agentplugins author validate ./review-helper --format json
agentplugins author inspect ./review-helper --format json
```

人类可读输出使用 `--format human`，需要时加 `--no-color`。
`--include-root` 会披露所选根路径，共享诊断时应避免不必要的私有路径。
`--release-policy` 增加有界的发布卫生检查，不是发布批准。技能子命令的参数更少，应查看其自身帮助。

## 有意维护 JSON

当前源码可先计划一个标准文件的确定性规范化，再在明确指定 `--write` 时写入：

```bash
agentplugins author normalize ./review-helper --document plugin.json
agentplugins author normalize ./review-helper --document plugin.json --write
```

要处理 MCP 文件时改用 `mcp.json`。原生导入只读取明确指定的严格 JSON Claude MCP 文件，不搜索配置目录。
先计划一个绝对路径、尚不存在的目标：

```bash
agentplugins author import native ./claude.json --from claude \
  --output /absolute/path/to/new-plugin --name new-plugin \
  --description 'Imported Claude MCP package'
```

检查安全或跳过的服务器及不支持的字段，再加 `--write` 将已验证包写入该目标。
当前导入仅接受安全的 stdio `command` 和 `args` 条目；含 `env`、`type` 或 `url` 的条目会跳过，
包括 HTTP 服务器和显式 `type: stdio`。不安全或含凭证的服务器也会跳过，不能假设所有原服务器都已导入。
此处不提供其他客户端的原生导入。

## 选择正确命令边界

安装器的 `agentplugins validate ./review-helper` 为安装读取包；`agentplugins doctor` 诊断受管理状态。
它们与源码创作操作不同。安装器的范围和安全参数不会自动成为创作参数。
init 会创建真实文件；normalize 和 import 需要 `--write` 才应用计划。继续[交付包](./handoff)。
