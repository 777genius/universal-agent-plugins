---
title: "Agent Plugins 文档"
description: "安装、管理和构建可移植 Agent Plugins 包。"
canonicalId: "page:home"
section: "home"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">公共文档</p>
  <h1>Agent Plugins</h1>
  <p class="docs-lead">为您使用的 AI 智能体安装技能和 MCP 工具，或构建自己的可移植插件包。</p>
</div>

## 选择您的路径

<div class="docs-grid">
  <a class="docs-card" href="./use/">
    <h2>使用插件</h2>
    <p>检查来源、预览变更，然后为用户账户或单个项目安装插件。</p>
  </a>
  <a class="docs-card" href="./build/">
    <h2>构建插件</h2>
    <p>使用 plugin.json、技能和 MCP 配置编写标准包，并了解源码构建中的创作工具。</p>
  </a>
</div>

## 安装第一个插件

安装 Node.js 22 或更高版本后，运行：

```bash
npx universal-agent-plugins add context7
```

CLI 会询问要使用哪些兼容客户端。检查计划，并完成结果中列出的激活或登录步骤。
若想直接使用原生可执行文件，请阅读适用于 macOS、Linux 和 Windows 的
[原生安装指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)。原生管理器本身不需要 Node.js；插件可能有自己的运行时要求。

## 当前可用功能

当前已验证的原生安装器版本为 [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94)，支持显式项目范围和锁定恢复。
add 和维护命令需明确指定 `--scope project`。未指定来源参数的 `install` 在当前目录存在普通 `uap.toml` 时自动选择项目范围，不向上查找；显式范围优先，已有锁文件会被严格验证。
请参阅[安装](./use/install)和[维护](./use/manage)。

各分发渠道的版本可能不同。使用 0.1.94 的自动项目选择前，先运行 `agentplugins version` 或
`npx universal-agent-plugins version`。若所选渠道仍较旧，可使用[原生或 Homebrew 安装](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)，
或继续使用 0.1.93 已支持的显式项目范围命令。


当前安装器不包含 `agentplugins author`。[构建指南](./build/)同时说明手动编写标准包的方法和当前源码中实现的创作命令。
旧版创作工具的发布证据仍对应其原始版本，不能用于说明当前安装器下载包含哪些命令。

## 如何理解结果

包验证、文件安装、客户端发现、MCP 工具执行和服务认证是不同的检查。
安装文件成功后，客户端仍可能需要信任确认、重新加载或新建会话。
原生测试只覆盖记录中的客户端版本和平台，不能证明所有插件在所有智能体中都可用。

- [快速入门](./guide/quickstart)
- [包的目录结构](./build/layout)
- [检查与证据](./build/checks)
- [客户端兼容性与测试边界](./reference/client-compatibility)
- [Agent Plugins 1.0 规范](https://agent-plugins.org/specification)

<!-- locale-historical-source:start
locale-historical-frontmatter:start
---
title: "plugin-kit-ai 文档"
description: "plugin-kit-ai 的公共文档。"
canonicalId: "page:home"
section: "home"
locale: "zh"
generated: false
translationRequired: true
---
locale-historical-frontmatter:end
<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">公共文档</p>
  <h1>plugin-kit-ai</h1>
  <p class="docs-lead">
    在一个仓库里构建，默认先走 Go 路径，之后再按需要加入 packages、Claude hooks、Gemini，
    或由仓库管理的集成配置，无需拆分项目。
  </p>
</div>

## 默认开始

- 当您想要最强的运行时和发布故事时，`Codex runtime Go` 是默认开始。

## 立即了解什么

- 当您添加更多通道时，一个存储库仍然是事实来源
- 选择符合您今天需要的起始路径
- 当产品需要更多输出时，稍后从同一存储库进行扩展
- 使用 `generate` 和 `validate --strict` 作为共享准备工作流程

## 支持 Node 和 Python 路径

- `codex-runtime --runtime node --typescript` 是主要支持的非 Go 路径。
- `codex-runtime --runtime python` 是受支持的 Python-first 路径。
- 两者都是本地解释运行时路径，因此目标机器仍然需要 Node.js `20+` 或 Python `3.10+`。
- 对于已经在这些堆栈中工作的团队来说，它们是明确的早期选择，但它们不是默认的开始。

<div class="docs-grid">
  <a class="docs-card" href="./guide/quickstart">
    <h2>快速启动</h2>
    <p>首先使用最强的默认路径，只有当产品需要更多输出时才扩展。</p>
  </a>
  <a class="docs-card" href="./guide/what-you-can-build">
    <h2>查看产品形状</h2>
    <p>查看一个存储库如何发展为运行时、包、扩展和存储库拥有的集成设置。</p>
  </a>
  <a class="docs-card" href="./guide/choose-a-target">
    <h2>选择目标</h2>
    <p>将目标与您想要发送插件的方式相匹配，而不是将每个输出视为相同的东西。</p>
  </a>
  <a class="docs-card" href="./reference/support-boundary">
    <h2>检查确切的合同</h2>
    <p>当您需要精确的支持边界和兼容性术语时，请使用参考页。</p>
  </a>
</div>

## 如果您稍后需要更多

- 当产品要求 Claude 挂钩时，添加 `Claude default lane`。
- 当产品是包或扩展输出时，添加 `Codex package` 或 `Gemini packaging`。
- 当存储库应拥有集成设置时，添加 `OpenCode` 或 `Cursor`。
- 在切换或 CI 之前使用 `validate --strict` 作为准备门。

## 常用扩展路径

- 从 Codex 运行时存储库开始，然后在包装成为产品的一部分时添加 Codex 包或 Gemini 。
- 当 Claude 钩子是产品时，从 Claude 开始，然后保持仓库开放，以供以后更广泛的交付通道使用。
- 从本地 Node 或 Python 开始，然后在下游交付很重要时添加捆绑包切换。
- 当存储库应管理集成配置而不仅仅是可执行行为时，添加 OpenCode 或 Cursor 。

## 按这个顺序阅读

<div class="docs-grid">
  <a class="docs-card" href="./guide/quickstart">
    <h2>1。快速入门</h2>
    <p>在考虑扩展之前先从一条推荐路径开始。</p>
  </a>
  <a class="docs-card" href="./guide/what-you-can-build">
    <h2>2。您可以构建什么</h2>
    <p>查看跨运行时、包、扩展和集成通道的产品形状。</p>
  </a>
  <a class="docs-card" href="./guide/choose-a-target">
    <h2>3。选择目标</h2>
    <p>选择与您实际想要发送插件的方式相匹配的目标。</p>
  </a>
  <a class="docs-card" href="./reference/support-boundary">
    <h2>4。支持边界</h2>
    <p>当您需要精确的兼容性语言和支持详细信息时，请使用参考集群。</p>
  </a>
</div>

如果您是新手，看完这些起始页面就可以先停下来。其他一切都是更深入的参考或实现细节。

## 当前仓库基线

- 此文档集中当前的公共基准是 [`v1.1.2`](/zh/releases/v1-1-2)。
- 这一组补丁先恢复了 legacy 与 current authored layouts 之间的 first-party 安装兼容性，然后修复了来自 GitHub repo-path 源的 Gemini 全量 multi-target 安装。
- 当您需要当前推荐的基线时从这里开始。

## 这个网站可以帮助您做什么

- 启动一个插件存储库，而不是按生态系统分割事实来源
- 选择推荐的起始路径，无需预先了解每个目标细节
- 稍后将相同的存储库扩展到更多运输路径
- 随着存储库的增长保留一个审查和验证故事
- 仅在需要时找到确切的合同
locale-historical-source:end -->
