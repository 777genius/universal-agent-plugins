---
title: "构建混合包"
description: "在一个包身份下组合技能和 MCP 配置。"
canonicalId: "page:build:hybrid"
section: "build"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 构建混合包

**源码创作：** 以下命令要求[启用创作功能的源码构建](./#build-from-source)。当前安装器版本不包含 `agentplugins author`。

当说明需要解释如何以及何时使用工具时，混合包很合适。两种组件共用一个根清单。
客户端可能对它们有不同支持，交付时应保留逐组件的兼容性结果。

## 明确选择 MCP 模板

使用 `--mcp-template` 选择 `mcp-remote` 或 `mcp-stdio`，再提供相应模板的必需输入。
不会从 URL、本地文件或已安装运行时推断传输协议。

远程混合包：

```bash
agentplugins author init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Guide research using the team MCP service' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

URL 是占位地址，请换成实际端点，并检查[远程 MCP 要求](./mcp-remote)。

## 或选择本地进程

为另一个不存在的目标目录创建 stdio 版本：

```bash
agentplugins author init ./local-research \
  --template hybrid \
  --name local-research \
  --description 'Guide research using local Node tools' \
  --skill-name research-guide \
  --mcp-template mcp-stdio \
  --runtime node
```

stdio 版本不应添加 `--url`。参阅[stdio 前提与运行边界](./mcp-stdio)。

## 将说明与工具连接起来

远程混合包的核心布局：

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

还会创建 README 和 `.gitignore`。stdio 版本增加 Node 源码和包文件。
在 `SKILL.md` 中说明何时使用工具、需要什么输入，以及如何报告缺失权限。
不要将技能文本当作授权。

## 检查两种组件

```bash
agentplugins author skills validate ./research-helper
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper --target codex,claude
agentplugins author test ./research-helper
agentplugins author compat ./research-helper --target codex,claude
```

本地版本改用 `./local-research`。客户端支持技能，不代表它也支持 MCP 传输协议或已激活服务。
合并打包不会消除这些限制。继续[添加技能](./skills)或[交付](./handoff)。
