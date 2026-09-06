---
title: "构建一个团队就绪的插件"
description: "旗舰级公共教程，用于将插件从脚手架转变为 CI 就绪、移交就绪和团队可读的形状。"
canonicalId: "page:guide:team-ready-plugin"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 构建一个团队就绪的插件

本教程将从第一个成功的插件停止的地方开始。目标不仅仅是“它可以在我的机器上运行”，而是另一个队友可以在没有隐藏知识的情况下克隆、验证和发布的存储库。

<MermaidDiagram
  :chart='`
flowchart LR
  scaffold[仓库脚手架已生成] --> explicit[明确路径与目标范围]
  explicit --> honest[保持生成文件可信]
  honest --> ci[加入可重复的 CI 门]
  ci --> handoff[让交接对队友可见]
  handoff --> ready[仓库已准备好交给团队]
`'
/>

## 结果

到最后，您应该拥有：

- 一个包标准编写的存储库
- 从项目源复制生成的文件
- 干净利落地通过严格的验证检查
- 为队友记录的明确的主要目标或范围内的目标
- 按目标明确的运行时选择或运行时策略
- 可以在另一台机器上重复的 CI 友好路径

## 1. 从最窄的稳定路径开始

从真正匹配当前任务的最窄稳定路径开始：

```bash
plugin-kit-ai init my-plugin --template custom-logic
cd my-plugin
plugin-kit-ai inspect . --authoring
go mod tidy
plugin-kit-ai validate . --platform codex-runtime --strict
```

这为您以后的交接提供了最干净的基础。

## 2. 明确选择

团队就绪的存储库至少应该说明：

- 哪个目标是主要目标以及哪些其他目标得到真正支持
- 它使用哪个运行时以及是否随目标而改变
- 主要验证命令是什么，或者多目标存储库需要哪些验证命令
- 是否依赖于 Go SDK 路径或共享运行时包

如果该信息仅存在于一名维护人员的头脑中，则该存储库尚未准备好。

## 3. 保持存储库的诚实

在扩展项目之前，请执行三个规则：

- 项目源位于包标准布局中
- 生成的目标文件是输出
- `generate` 和 `validate --strict` 仍然是正常工作流程的一部分

不要手动修补生成的文件，然后希望团队永远不会重新运行生成。

## 4. 添加可重复的 CI 门

对于默认的 Go launcher 路径，最小门应如下所示：

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

如果选择的是 Node 或 Python 路径，请加入 `bootstrap` 并在 CI 中固定运行时版本：

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

如果存储库支持多个目标，则 CI 门应显式检查每个支持的目标，而不是假设间接覆盖。

## 5. 检查您是否确实需要不同的路径

只有在真正需要权衡时才远离默认路径：

- 当产品要求 Claude 挂钩时，使用 `claude`
- 当团队首先是 TypeScript 并且本地运行时权衡可接受时使用 `node --typescript`
- 当项目有意位于存储库本地且 Python-first 时，请使用 `python`

改变路线应该解决产品或团队问题，而不仅仅是反映语言偏好。如果产品确实是多目标的，请直接说：存储库在支持的范围内有一个主要路径和其他目标。

## 6. 使切换可见

新队友应该能够从存储库和文档中回答以下问题：

- 如何安装先决条件？
- 什么命令证明回购是健康的？
- 我要验证什么目标？
- 哪些文件是创作状态的，哪些是生成的？

如果其中任何一个的答案是“询问原作者”，那么存储库仍然没有准备好。

## 7. 将回购链接回公共合约

团队就绪的插件存储库应该指出：

- [生产准备情况](/zh/guide/production-readiness)
- [CI 集成](/zh/guide/ci-integration)
- [存储库标准](/zh/reference/repository-standard)
- 当前的公开发行说明，现在为 [v1.1.2](/zh/releases/v1-1-2)

## 最终规则

当另一个队友可以克隆它、了解路径和目标范围、重现生成的输出并通过严格的验证门而无需即兴创作时，该存储库就已准备就绪。

将本教程与[构建您的第一个插件](/zh/guide/first-plugin)、[创作架构](/zh/concepts/authoring-architecture)和[支持边界](/zh/reference/support-boundary)配对。

</details>
