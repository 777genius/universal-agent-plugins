---
title: "版本和兼容性政策"
description: "如何考虑版本、兼容性承诺、包装器、SDK 以及 plugin-kit-ai 中的支持词汇。"
canonicalId: "page:reference:version-and-compatibility"
section: "reference"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 版本和兼容性政策

本页用于一项实际的团队决策：我们正在标准化什么，这一承诺有多强？

## 60 秒内选择

- 当您的团队需要针对版本、包装器、SDK、运行时和兼容性承诺的紧凑策略时，请阅读此页面
- 当您需要最短的实际支持答案时，请阅读[支持边界](/zh/reference/support-boundary)
- 当您想要了解特定版本的故事时，请阅读 [版本](/zh/releases/)

## 公共基线

考虑三个层面的标准化：

- 您在存储库中选择的发布线
- 您在该释放线内选择的路径的支持级别
- 围绕该路径的安装或交付机制

这些层是相关的，但它们不可互换。

## 推荐车道和正式层

在文档和政策中使用一种简单的翻译：

- `Recommended` 通常表示升级的 `public-stable` 生产路径
- `Advanced` 表示具有更窄或更专业合同的支撑表面
- `Experimental` 表示选择加入的流失超出了正常的兼容性预期

今天主要推荐的路径有：

- `Codex runtime Go`
- `Codex package`
- `Gemini packaging`
- `Gemini Go runtime`
- `Claude default stable lane`
- `Python` 和 `Node` 本地运行时路径作为受支持目标上受支持和推荐的非 Go 创作选择

## 兼容性真正涵盖了什么

最强烈的公开承诺是：

- 公开的 CLI 合约
- 推荐的 Go SDK 路径和上面列出的推荐生产路径
- 支持的目标上推荐的本地 Python 和 Node 运行时路径
- `public-stable` 生成的输出的记录行为

兼容性并不意味着每个包装器、便捷路径或专用表面都具有相同的承诺。

## 公共语言与正式术语

与团队交谈时使用此翻译：

- `Recommended` 通常表示该路径位于当前最强的 `public-stable` 合约内
- `Advanced` 表示支持该表面，但比第一个默认值更专业或更窄
- `Experimental` 表示选择加入流失，没有正常的兼容性预期

当团队需要确切的策略时，请使用正式术语 `public-stable`、`public-beta` 和 `public-experimental`。

## 包装器、SDKs 和运行时 APIs

不要将它们标准化，就好像它们是同一件事一样。

- Homebrew、npm、PyPI 和经过验证的脚本是 CLI 的安装通道
- Go SDK 是公共 SDK 表面
- 运行时 API 与其声明的运行时路径相关联

如果您将安装包装器视为具有与 SDK 或运行时路径相同的承诺，那么您将标准化错误的层。

## 团队应该标准化什么

健康的团队通常会标准化：- 一项已声明的发布基线
- 具有清晰支持故事的一条主要路径
- 移交和推出之前的一个验证门
- 对正式兼容性术语的一种共同解释

## 最终规则

仅标准化您的团队实际上愿意在 CI、移交和部署中捍卫其公开承诺的发布线和路径。
</details>
