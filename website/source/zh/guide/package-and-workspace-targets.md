---
title: "包和集成设置"
description: "当打包或签入集成设置是正确的答案时，而不是可执行运行时插件。"
canonicalId: "page:guide:package-and-workspace-targets"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">包和集成设置</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 包和集成设置

并非每个项目都应该作为可执行运行时插件提供。

有时，真正的需求是另一个系统将加载的包、扩展工件或存储在存储库中的签入集成设置。

## 简短规则

当交付形式比直接运行插件更重要时，选择包或集成设置。

## 选择此页面时

在以下情况下这是正确的路径：

- 包装是真正的交货要求
- 主机期望扩展或打包工件
- 存储库主要需要为另一个工具签入集成设置
- 可执行运行时会增加不必要的操作工作

## 这与运行时路径有何不同

当您需要可执行插件时，运行时路径通常是最清晰的默认值。

包和集成设置回答了一个不同的问题：这个插件应该如何交付或连接到另一个系统？

## 安全心理模型

当您想直接运行插件时选择运行时。当交付形状是主要要求时，选择封装或集成设置。

## Codex 包边界

对于官方 Codex 包通道，保持包布局明确且狭窄：

- `.codex-plugin/` 仅包含 `plugin.json`
- 可选的 `.app.json` 和 `.mcp.json` 留在插件根目录

此包路径适用于官方 Codex 插件包表面，而不是用于将存储库本地运行时连接混合到包布局中。
</details>
