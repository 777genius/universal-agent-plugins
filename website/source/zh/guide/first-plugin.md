---
title: "构建你的第一个插件"
description: "从初始化到严格验证的最小端到端教程。"
canonicalId: "page:guide:first-plugin"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 构建你的第一个插件

本教程为您提供了最强默认路径上最简单的第一个工作存储库。

它故意缩小范围：

- 第一个目标：`codex-runtime`
- 第一语言：`go`
- 第一个准备门：`validate --strict`

这种狭窄的形状仅适用于第一次运行。如果您主要关心的是更广泛的一个存储库、多个输出的故事，请在本教程之后阅读[一个项目，多个目标](/zh/guide/one-project-multiple-targets)。

## 1. 安装 CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. 脚手架A项目

```bash
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
```

默认的 `init` 路径已经是推荐的生产起点。

## 3. 生成目标文件

```bash
plugin-kit-ai generate .
```

将生成的目标文件视为输出。通过 `plugin-kit-ai` 继续编辑存储库，而不是手动维护生成的文件。

## 4. 运行准备门

```bash
plugin-kit-ai validate . --platform codex-runtime --strict
```

使用它作为本地插件项目的主要 CI 级门。

## 你现在拥有的

- 一个插件存储库
- 新仓库在 `plugin/` 下创作的文件
- 生成 Codex 运行时输出
- 通过 `validate --strict` 的明确准备门

## 5. 何时切换路径

仅当您确实需要时才切换到另一路径：

- 为 Claude 插件选择 `claude`
- 选择 `--runtime node --typescript` 作为主要支持的非 Go 路径
- 当项目位于存储库本地且您的团队优先为 Python 时，选择 `--runtime python`
- 仅当您确实需要不同的方式来发送插件时，才选择 `codex-package`、`gemini`、`opencode` 或 `cursor`

这并不意味着存储库必须永远保持单一目标：从今天最重要的目标开始，只有当产品真正扩展时才添加其他目标。

## 后续步骤

- 在离开默认路径之前，请阅读[选择运行时](/zh/concepts/choosing-runtime)。
- 如果单回购、多输出的想法是您关心该产品的核心原因，请阅读[一个项目，多个目标](/zh/guide/one-project-multiple-targets)。
- 当您想要一个已知良好的示例存储库时，请使用[入门模板](/zh/guide/starter-templates)。
- 当您需要精确的命令行为时，请浏览 [CLI 参考](/zh/api/cli/)。

</details>
