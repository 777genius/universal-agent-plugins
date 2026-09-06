---
title: "您可以构建什么"
description: "使用此页面作为产品地图：存在哪些输出、默认启动是什么样子，以及一个存储库以后如何扩展。"
canonicalId: "page:guide:what-you-can-build"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 您可以构建什么

使用此页面作为产品地图。它显示了存在哪些类型的输出，而不是一个存储库稍后应该增长或分裂的时间。

plugin-kit-ai 可以从一个可执行插件开始，并随着时间的推移扩展到其他支持的输出。

## 推荐的起始形状

从一个运行时路径开始，通常是 Codex 运行时和 Go。这使第一个存储库保持简单，并为您提供最清晰的验证和发布循环。

如果您的团队已经在 Node/TypeScript 或 Python 中工作，那么这些起始路径也受支持。

## 一个存储库，许多支持的输出

从同一个项目中，您可以朝着以下方向发展：

- 支持的主机的运行时输出
- 当包装是真正的交付要求时，包装输出
- 期望扩展工件的主机的扩展输出
- 当存储库主要需要另一个工具的签入配置时，存储库拥有的集成设置

## 此页面不适合做什么

选择 Node 或 Python 并不强迫您在第一天就决定每个打包或集成细节。

此页面是概述。如果您的问题是一个存储库是否应该继续增长，请阅读[一个项目，多个目标](/zh/guide/one-project-multiple-targets)。

</details>
