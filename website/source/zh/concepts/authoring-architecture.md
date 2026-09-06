---
title: "项目来源和产出"
description: "编写的文件、生成的输出、严格验证和移交如何在 plugin-kit-ai 中组合在一起。"
canonicalId: "page:concepts:authoring-architecture"
section: "concepts"
locale: "zh"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">项目来源和产出</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 项目来源和产出

此页面比主要产品型号窄。它解释了存储库内的工作边界：您编写的内容、生成的内容以及为什么拆分可以保持项目的可维护性。

## 核心形状

```text
project source -> generate -> target outputs -> validate --strict -> handoff
```

来源保持稳定。输出可以根据目标而改变。验证可确保生成的结果仍然可以安全地传递。

## 编写的文件与生成的文件

编写的文件是您需要直接维护的存储库的一部分。

生成的文件是您选择的目标的构建工件。它们是真实的交付输出，但它们不是项目真相应该漂移的地方。

这种区别使存储库保持可读并确保再生安全。

## 为什么拆分很重要

如果没有明确的划分，团队最终会编辑生成的输出，失去可重复性，并使升级变得比他们需要的更加困难。

通过明确的划分，您可以：

- 直接查看源代码更改
- 自信地再生输出
- 每次验证相同的交付形状
- 稍后添加另一个受支持的输出，而无需从头开始重建存储库

## 这与更大的模型有何关系

如果您想要更高级的解释，请从[plugin-kit-ai如何工作](/zh/concepts/managed-project-model)开始。

</details>
