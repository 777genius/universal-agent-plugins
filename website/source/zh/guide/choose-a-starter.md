---
title: "选择一个入门存储库"
description: "一个实用的矩阵，用于根据目标、运行时间和交付路径选择正确的官方启动器。"
canonicalId: "page:guide:choose-a-starter"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 选择一个入门存储库

当您想要以最快的路径进入存储库并随后扩展到更多受支持的输出时，请使用此页面。

<MermaidDiagram
  :chart='`
flowchart TD
  Start[Need a starter] --> 产品{主路径是 Codex 或 Claude}
  产品 --> Codex[Codex 入门系列]
  产品 --> Claude[Claude 入门系列]
  Codex --> 运行时{Go、Node 或 Python}
  Claude --> 运行时2{Go、Node 或Python}
`'
/>

在选择之前，请记住一条重要规则：

- 入门模板只决定你如何开始
- 它不是产品的最终边界
- 它不会阻止一个仓库以后支持更多目标

如果这种区别仍然模糊，请先阅读[一个项目，多个目标](/zh/guide/one-project-multiple-targets)。

## 快速选择，然后再扩展

- 当你想要最强的生产路径时选择Go
- 当您想要主要支持的非 Go 路径时，选择 Node/TypeScript
- 当存储库有意 Python-first 并保留在存储库本地时，选择 Python
- 仅当 Claude 挂钩是实际产品要求时才选择 Claude 启动器

选择第一个正确路径的起始点，而不是想象中的永久产品边界。

## 选择后保持真实的内容

- 你仍然保留一个仓库。
- 您仍然保留相同的核心工作流程。
- 随着产品的发展，您可以稍后添加支持的目标。
- 支持深度取决于您添加的目标。

## 入门矩阵

|如果你想要|最佳入门模板|为什么 |
| --- | --- | --- |
|最强的 Codex 生产路径| `plugin-kit-ai-starter-codex-go` | Go - 默认最强、交接最清晰的生产路径 |
| Python 中的仓库本地 Codex 插件 | `plugin-kit-ai-starter-codex-python` |稳定的 Python 子集，仓库布局清晰 |
| Node/TS 中的仓库本地 Codex 插件 | `plugin-kit-ai-starter-codex-node-typescript` |主要支持的非 Go 路径 |
|最强的 Claude 生产路径| `plugin-kit-ai-starter-claude-go` |稳定的 Claude 子集，加上最清晰的生产路径 |
| Python 中的仓库本地 Claude 插件 | `plugin-kit-ai-starter-claude-python` |带有 Python helper 的稳定 Claude hook 子集 |
| Node/TS 中的仓库本地 Claude 插件 | `plugin-kit-ai-starter-claude-node-typescript` |适合 TypeScript 优先团队的稳定 Claude hook 子集 |

## 共享包变体

除非您已经知道您的团队希望 `plugin-kit-ai-runtime` 作为可重用依赖项而不是供应的帮助程序文件，否则请忽略此部分。

在以下情况下使用共享包变体：

- 您想要跨多个插件存储库共享依赖项
- 您可以轻松地显式固定和升级运行时包
- 您不希望将辅助文件复制到每个存储库中

当前共享包启动器：

- [`plugin-kit-ai-starter-codex-python-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-codex-python-runtime-package): Python Codex 启动器，其中 `plugin-kit-ai-runtime` 固定在 `requirements.txt` 中
- [`plugin-kit-ai-starter-claude-node-typescript-runtime-package`](https://github.com/777genius/plugin-kit-ai-starter-claude-node-typescript-runtime-package): Node/TypeScript Claude 启动器，其中 `plugin-kit-ai-runtime` 固定在 `package.json` 中

如果您要在普通 Python 启动器和运行时包 Python 启动器之间进行选择，请先阅读[构建 Python 运行时插件](/zh/guide/python-runtime)，然后再阅读[选择交付模型](/zh/guide/choose-delivery-model)。

## 何时避免过度优化选择

不要花太长时间寻找完美的 starter。

如果您不确定：

1. 从 Go 启动器开始，以获得最强的默认值
2. 从主要支持的非 Go 路径的 Node/TypeScript 启动器开始
3. 仅当团队权衡已经真实时才选择 Python 或共享包变体

## 良好的团队策略

团队范围内的 starter 选择最好在一段时间内保持一致，这样：

- 每个人都熟悉仓库布局
- CI 使用相同的准备流程
- 移交不依赖于维护者的解释

但是，稳定的 starter 选择仍然不会阻止一个仓库在产品需要时稍后添加其他目标。

将此页面与[入门模板](/zh/guide/starter-templates)、[选择交付模型](/zh/guide/choose-delivery-model)和[存储库标准](/zh/reference/repository-standard)配对。

</details>
