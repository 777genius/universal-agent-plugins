---
title: "存储库标准"
description: "一个健康的 plugin-kit-ai 存储库应该是什么样子，以及如何将项目源与生成的输出分开。"
canonicalId: "page:reference:repository-standard"
section: "reference"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">存储库标准</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 存储库标准

此页面定义了健康的 `plugin-kit-ai` 存储库的公共形状。

## 主要规则

存储库应使其预期设置显而易见，并且生成的输出可重现。

实际上，这意味着：

- 项目来源很容易找到
- 生成的目标文件显然是输出
- 主要目标或范围内的目标可见
- 运行时选择或运行时策略可见
- 验证命令已记录

## 什么应该很容易找到

一个健康的仓库应该让这些东西无需深挖就能找到：

- 主要目标或范围内的目标
- 按目标选择的运行时或运行时策略
- 规范的 `validate --strict` 命令，或验证命令（如果有多个目标）
- 运行时先决条件，例如 Go、Python 或 Node
- 存储库是否使用 Go SDK 路径或共享运行时包

## 什么不应该是真相的来源

这些不应作为事实的主要来源：

- 手工编辑生成的目标文件
- 包装安装包被视为运行时 APIs
- 关于“您实际需要运行的命令”的部落知识

## 健康的存储库信号

- `generate` 可以重现目标输出
- `validate --strict` 干净地传递给预期目标，或存储库公开声称支持的每个目标
- 存储库在面向公众的文档或自述文件中解释了其选择的路径
- CI 使用与本地开发相同的公共准备流程

## 弱存储库信号

- 目标文件生成后手动修补
- 运行时或目标选择在机器之间是隐式的或不一致的
- 下游用户需要维护者指导才能重现基本流程
- 仓库承诺支持已经超出公开声明的支持边界

## 与此文档站点的关系

该公共文档站点将存储库标准视为以下位置：

- 创作指南开始运作
- 支持边界变得可执行
- 交接变得可信

将此页面与[创作工作流程](/zh/reference/authoring-workflow)、[生产准备情况](/zh/guide/production-readiness) 和[词汇表](/zh/reference/glossary) 配对。

</details>
