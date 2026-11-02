---
title: "构建远程 MCP 包"
description: "描述现有 MCP 端点，并区分配置与连接验证。"
canonicalId: "page:build:mcp-remote"
section: "build"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 构建远程 MCP 包

**源码创作：** 以下命令要求[启用创作功能的源码构建](./#build-from-source)。当前安装器版本不包含 `agentplugins author`。

已有服务器且包只需描述连接方法时，选择远程 MCP。创作过程记录 URL，不联系端点。
向用户交付前，作者应了解服务的传输协议和认证要求。

## 显式提供端点

`https://mcp.example.com/mcp` 是文档占位地址，请换成实际端点。
命令接受配置不代表该地址提供 MCP，也不代表您的账户有访问权限。

```bash
agentplugins author init ./docs-service \
  --template mcp-remote \
  --name docs-service \
  --description 'Connect to the team documentation MCP service' \
  --url https://mcp.example.com/mcp
```

目标目录必须不存在，父目录必须存在。模板要求绝对 HTTPS URL，或回环地址上的 HTTP。
URL 不能包含凭证、片段或未解析令牌。请勿将密码写入 URL。

## 理解生成内容

```text
docs-service/
  plugin.json
  mcp.json
  README.md
  .gitignore
```

根清单定义包身份。`mcp.json` 在 `mcpServers` 下描述服务器，模板使用 `type: streamable-http` 和您提供的 URL。
它不生成服务器或技能；若还需要说明工具的使用时机，请选择[混合包](./hybrid)。

在 README 中说明服务用途、账户前提及认证文档。凭证应保存在包外。
符合规范的文件中有 URL，不等于服务在线。

## 检查配置和目标支持

```bash
agentplugins author validate ./docs-service
agentplugins author inspect ./docs-service --target codex,claude
agentplugins author test ./docs-service
agentplugins author compat ./docs-service --target codex,claude
agentplugins author doctor ./docs-service
```

inspect 显示组件和未解决的要求；compat 检查目标适配器的静态支持，两者都不要求客户端已安装。
doctor 不联系端点。需要显式 MCP 连接检查时，请阅读[运行检查](./checks#evaluate-mcp-runtime-explicitly)并明确允许网络访问。

## 明确剩余工作

静态证据应记录传输协议和预期客户端。安装器仍要检查安装策略并报告激活步骤。
用户可能还需要登录、批准访问或新建会话。工具发现和调用必须单独验证。

静态检查失败时先修正捕获的配置。端点之后不可达时，应检查服务或客户端连接；再次通过静态测试不能验证连通性。
继续[理解报告](./checks)和[交付](./handoff)。
