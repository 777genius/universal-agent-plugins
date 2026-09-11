---
title: "支持边界"
description: "对 plugin-kit-ai 推荐内容的最短实用答案，谨慎支持，并保持实验性。"
canonicalId: "page:reference:support-boundary"
section: "reference"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">支持边界</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 支持边界

当您需要有关支持的最简短诚实答案时，请使用此页面。

它回答了三个团队问题：

- 默认推荐什么是安全的
- 支持什么，但应有意选择
- 哪些内容仍处于实验阶段，不应悄悄成为团队政策

## 安全默认值

这些是当今最安全的默认设置：

- Go 是推荐的默认运行时路径。
- `validate --strict` 是本地 Python 和 Node 运行时存储库的主要就绪门。
- `Codex runtime Go`、`Codex package`、`Gemini packaging`、`Gemini Go runtime` 和 Claude 默认稳定通道是主要推荐的生产通道。
- 当有意进行本地解释运行时权衡时，`Python` 和 `Node` 受支持的非 Go 路径和推荐的非 Go 选择。

## 这如何映射到正式合同

公共文档首先使用三个简单的词：

- `Recommended` 通常映射到当前最强的 `public-stable` 生产通道。
- `Advanced` 表示具有更窄、更专业或更仔细的合同的支撑表面。
- `Experimental` 表示选择加入超出正常兼容性预期的流失。

当团队需要精确的策略语言时，正式术语优先：`public-stable`、`public-beta` 和 `public-experimental`。

## 今天推荐

如果您需要实际的答案，请从这里开始：

- 建议在默认稳定挂钩路径上使用 Claude。
- 建议将 Codex 用于 `Notify` 运行时路径和官方 `codex-package` 路径。
- 建议使用 Gemini 打包，并且升级的 Gemini Go 运行时也可用于生产。
- OpenCode 和 Cursor 是存储库拥有的集成设置路径。它们很有用，但它们不是默认的可执行运行时启动。

## 高级表面

仅当权衡明确且值得时才选择高级表面。

典型例子：

- OpenCode 和 Cursor 当存储库应该拥有集成配置而不是传送运行时路径时
- 超出主要推荐路径的更窄或专门的运行时扩展
- 当真正关心的是 CLI 交付而不是运行时 APIs 或 SDKs 时安装包装器
- 有用的专用配置界面，但不是大多数团队的第一个默认设置

## 实验表面

将实验区域视为选择性加入和高流失率。

它们对于早期采用者来说可能很有用，但它们不应该悄悄成为团队的长期标准。

## 实用规则

如果您正在选择一个团队，请标准化最窄的路径，您实际上愿意在 CI、部署和移交中捍卫其承诺。
</details>
