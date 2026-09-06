---
title: "plugin-kit-ai 的工作原理"
description: "当您生成输出、严格验证并交付干净的结果时，一个存储库如何保持事实来源。"
canonicalId: "page:concepts:managed-project-model"
section: "concepts"
locale: "zh"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# plugin-kit-ai 的工作原理

plugin-kit-ai 将一个存储库保留为插件的真实来源。您编辑您拥有的文件，生成您需要的输出，严格验证结果，并交付随时间推移保持可预测的存储库。

## 简短版本

核心循环很简单：

```text
source -> generate -> validate --strict -> handoff
```

该循环很重要，因为该项目不仅仅是一个入门模板。生成的输出可以随着目标的发展而改变，而您编写的源代码保持清晰且可维护。

## 一个仓库作为事实来源

存储库是插件真正存在的地方。

- 创作的文件由您掌控
- 生成的输出是从该源重建的
- 验证检查您计划发送的输出
- 仅在生成的结果干净后才会发生切换

这可以让一个项目谨慎地发展，而不是将相同的插件逻辑分散到多个存储库中。

## 你实际编辑的内容

您不断编辑项目源代码和您拥有的插件代码。您不会将生成的输出视为项目真正所在的位置。

该边界使升级、目标更改和维护工作保持可控。

## 为什么这不仅仅是入门模板

入门模板为您提供初始形状。 plugin-kit-ai 在第一天之后继续管理循环：

- 它从同一源重新生成特定于目标的输出
- 它验证您要运送的物品
- 它使编写的文件和生成的文件清晰分开
- 它允许一个存储库稍后扩展到更多输出，而无需重写整个项目模型

## 接下来去哪里

- 阅读[项目源和输出](/zh/concepts/authoring-architecture) 了解创作与生成的边界。
- 阅读[目标模型](/zh/concepts/target-model) 了解不同的输出类型。
- 当您想进一步扩展一个存储库时，请阅读[一个项目，多个目标](/zh/guide/one-project-multiple-targets)。

</details>
