---
title: "快速入门"
description: "安装已发布的插件，或准备可移植的 Agent Plugins 1.0 包。"
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

<a id="快速入门"></a>

# 使用插件 / 构建插件

安装已发布的插件，或准备可移植的 Agent Plugins 1.0 包。

## 使用插件 {#use-plugins}

现已可用：通过 Universal Agent Plugins 安装插件。使用 Node.js 22+ 时运行：

```bash
npx universal-agent-plugins add context7
```

如需在 macOS、Linux 或 Windows 上进行原生安装，请阅读安装指南。CLI 会询问要使用哪些兼容的客户端；个别插件可能需要自己的运行环境。

[安装指南](https://github.com/777genius/universal-agent-plugins#quick-start)

Agent Plugins 创作已在 `universal-agent-plugins@0.1.65` 中发布；GitHub 版本：`agentplugins-v0.1.65`。

兼容性取决于包。通过模式验证并不证明运行、OAuth 或激活成功。Codex 不支持声明的 MCP SSE；stdio 和 Streamable HTTP 保留现有适配器支持。

[客户端兼容性（英文）](https://github.com/777genius/universal-agent-plugins#supported-clients)

## 构建插件 {#build-plugins}

以 plugin.json 为核心构建可移植的 Agent Plugins 1.0 包，可选包含 skills/ 和 mcp.json。客户端支持取决于包及其组件。

**Agent Plugins 创作已发布**

静态创作已通过 `universal-agent-plugins@0.1.65` 中的 `agentplugins author` 发布；GitHub 版本：`agentplugins-v0.1.65`。npm、Homebrew、原生归档和公共 E2E 已验证。此版本不运行插件代码或开发服务器，不安装依赖项，不导出插件包，也不发布插件包。

[Agent Plugins 1.0 规范](https://agent-plugins.org/specification)
