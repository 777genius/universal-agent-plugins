---
title: "稳定性模型"
description: "plugin-kit-ai 如何对公共稳定区、公共测试区和公共实验区进行分类。"
canonicalId: "page:concepts:stability-model"
section: "concepts"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 稳定性模型

`plugin-kit-ai` 使用正式的合同条款，因此团队可以准确决定他们想要标准化的内容。

<MermaidDiagram
  :chart='`
flowchart TD
  Stable[public stable] --> 测试版[公开测试版]
  Beta --> 实验[公共实验]
  StableNote[正常生产预期] -.-> 稳定
  BetaNote[支持但不冻结] -.-> Beta
  实验注释[选择流失] -.-> 实验
`'
/>

## 公共语言与正式语言

公共文档使用更简单的第一遍词汇：

- `Recommended` 通常指向最强的电流 `public-stable` 路径
- `Advanced` 指向更窄或更专业的受支持表面
- `Experimental` 映射到 `public-experimental`

当您设置兼容性策略时，应以正式条款为准。

## 如何阅读推荐

`Recommended` 是产品语言，不能替代正式合同。

- 它通常意味着升级的 `public-stable` 生产路径
- 这并不意味着每个目标都平等
- 它不会仅通过措辞升级 `public-beta` 或 `public-experimental` 表面

## 公共稳定版

将 `public-stable` 视为您可以根据正常生产预期进行构建的级别。

这是大多数团队应该更喜欢的默认标准和长期部署的层级。

## 公开测试版

将 `public-beta` 视为受支持，但不冻结。

仅当权衡明确且对产品而言值得时才使用 Beta 版。

## 公共实验

将 `public-experimental` 视为超出正常兼容性预期的选择加入流失。

它对于学习或早期采用可能很有用，但它不应该悄然成为团队的默认设置。

## 实用规则

1. 优先选择您正在构建的产品的推荐路径。
2. 仅当您需要策略或兼容性精确性时才使用准确的正式术语。
3. 使用 `validate --strict` 作为您计划发送的存储库的就绪门。

</details>
