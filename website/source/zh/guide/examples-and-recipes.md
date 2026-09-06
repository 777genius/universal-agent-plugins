---
title: "示例和食谱"
description: "plugin-kit-ai 中的公共示例存储库、入门存储库、本地运行时引用和技能示例的引导图。"
canonicalId: "page:guide:examples-and-recipes"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">示例和食谱</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 示例和食谱

当您想查看 `plugin-kit-ai` 在真实存储库中的样子而不是仅阅读抽象指南时，请使用此页面。

## 1. 生产插件示例

这些是成品公共形状的最清晰示例：

- [`codex-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-basic-prod)：Go 加 `codex-runtime` 的生产参考仓库
- [`claude-basic-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/claude-basic-prod)：Go 加 `claude` 的生产参考仓库
- [`codex-package-prod`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/codex-package-prod)：`codex-package` 目标
- [`gemini-extension-package`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/gemini-extension-package)：`gemini` 打包目标
- [`cursor-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/cursor-basic)：`cursor` 工作区配置目标
- [`opencode-basic`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/plugins/opencode-basic)：`opencode` 工作区配置目标

当您需要时，请阅读这些内容：

- 具体的仓库布局
- 真实生成的输出
- “健康”的真实公开例子

重要提示：这些示例显示了不同的公共产品形状。它们并不意味着必须将真实系统分为每个目标的单独存储库。

## 2. 入门存储库

当您想要从已知良好的基线而不是从空目录开始时，请使用入门存储库。

它们最适合：

- 首次设置
- 团队入职
- 在 Go、Python、Node、Claude 和 Codex 起始点之间进行选择

最直接的 code-first 链接是：

- [`plugin-kit-ai-starter-codex-go`](https://github.com/777genius/plugin-kit-ai-starter-codex-go)
- [`plugin-kit-ai-starter-codex-python`](https://github.com/777genius/plugin-kit-ai-starter-codex-python)
- [`plugin-kit-ai-starter-codex-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-codex-node-typescript)
- [`plugin-kit-ai-starter-claude-go`](https://github.com/777genius/plugin-kit-ai-starter-claude-go)
- [`plugin-kit-ai-starter-claude-python`](https://github.com/777genius/plugin-kit-ai-starter-claude-python)
- [`plugin-kit-ai-starter-claude-node-typescript`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript)

如果您仍在选择，请将其与 [选择一个入门存储库](/zh/guide/choose-a-starter) 配对。

## 3. 本地运行时引用

`examples/local` 区域显示 Python 和 Node 保持本地优先的存储库的运行时引用。

这些在以下情况下很有用：

- 您想更深入地了解解释的运行时故事
- 您想要比较 JavaScript、TypeScript 和 Python 本地运行时设置
- 除了入门存储库之外，您还需要一个具体的参考

可以先看：

- [`codex-node-typescript-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-node-typescript-local)
- [`codex-python-local`](https://github.com/777genius/plugin-kit-ai/tree/main/examples/local/codex-python-local)

## 4. 技能示例

`examples/skills` 区域显示支持技能示例和帮助程序集成。

这些并不是大多数插件作者的主要切入点，但它们在以下情况下很有价值：

- 您想要将文档、审阅或格式化助手连接到更广泛的工作流程中
- 您想了解相邻技能如何适应插件存储库

## 按目标推荐阅读

- 想要最强的运行时示例：从 Codex 或 Claude 生产示例开始，然后阅读 [构建团队就绪插件](/zh/guide/team-ready-plugin)。
- 想按语言和目标看 code-first 示例：先打开上面的 Go、Python 或 Node 入门仓库，然后阅读 [Build Custom Plugin Logic](/en/guide/build-custom-plugin-logic)。
- 想要打包或工作区配置示例：从 Codex 包、Gemini、Cursor 或 OpenCode 示例开始，然后阅读 [包和工作区目标](/zh/guide/package-and-workspace-targets)。
- 想要一个干净的起点，而不是一个完成的示例：转到[入门模板](/zh/guide/starter-templates)。
- 想要在查看存储库之前选择目标：请阅读[选择目标](/zh/guide/choose-a-target)。
- 首先想要完整的单存储库扩展故事：阅读[你可以构建什么](/zh/guide/what-you-can-build)。

## 最终规则

示例应该阐明公共契约，而不是取代它。

使用示例存储库来查看形状和健康的输出。对于单存储库多目标心智模型，请阅读[一个项目，多个目标](/zh/guide/one-project-multiple-targets)。

</details>
