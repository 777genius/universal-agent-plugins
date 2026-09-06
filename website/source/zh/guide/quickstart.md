---
title: "快速入门"
description: "通往工作 plugin-kit-ai 项目的最快推荐路径。"
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 快速入门

当您想要一个插件存储库稍后可以发展为更多方式来运送插件时，这是最短的推荐路径。

首先从一条强有力的道路开始。稍后当产品实际需要时添加包、扩展或存储库拥有的集成设置。

## 如果你只读一件事

从默认的 Go 路径开始，除非您已经知道 Claude 挂钩、Node/TypeScript 或 Python 定义了产品要求。

您的第一个选择是起点，而不是存储库的永久边界。

## 推荐默认值

如果您没有充分的理由选择其他路径，请从这里开始：

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

这为您提供了当今最强大的默认路径：基于 Go 的 Codex 运行时存储库，可以轻松验证、移交和稍后扩展。

## 为什么这是默认值

- 从第一天开始就一个仓库
- 今天最干净的运行时和发布故事
- 为以后的封装、扩展和集成通道提供最简单的基础

## 你得到什么

- 从第一天起就有一个插件仓库
- 新仓库在 `plugin/` 下创作文件
- 从同一存储库生成 Codex 运行时输出
- 通过 `validate --strict` 进行干净的准备检查

## 支持 Node 和 Python 路径

如果您的团队已位于 Node/TypeScript 或 Python 中，则这些路径从一开始就受支持且可见：

- `codex-runtime --runtime node --typescript`
- `codex-runtime --runtime python`
- 两者都是本地解释运行时路径，因此目标机器仍然需要 Node.js `20+` 或 Python `3.10+`
- 当您想要最强的一般制作故事时，Go 仍然保持默认值

## 如果您有意从 Node 或 Python 开始

仅当语言选择已是产品要求的一部分时才使用此替代流程：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

或者以 Python 开头：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## 接下来做什么

- 编辑 `plugin/` 下的插件
- 更改后再次运行 `plugin-kit-ai generate ./my-plugin`
- 再次运行 `plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict`
- 只有在产品需要时才添加另一种运输方式

## 稍后展开

|如果你想要|稍后添加 |
| --- | --- |
| Claude 与真实产品挂钩 | `claude` |
|官方Codex包| `codex-package` |
| Gemini 扩展包 | `gemini` |
|回购拥有的集成设置 | `opencode` 或 `cursor` |

仅当 Claude 挂钩已经是实际产品需求时，才首先选择 `claude`。

## 稍后扩展的内容

- 当您添加更多通道时，存储库保持统一
- 包和扩展通道来自同一来源
- 当存储库应该拥有集成设置时，OpenCode 和 Cursor 适合
- 确切的支持边界保留在参考文档中，而不是在您的首次启动流程中

## 快速入门后

- 如果您想要最窄的推荐教程，请继续[构建您的第一个插件](/zh/guide/first-plugin)。
- 如果您想要完整的产品地图，请继续[您可以构建什么](/zh/guide/what-you-can-build)。
- 当您准备好将存储库与您想要的运输方式相匹配时，继续[选择目标](/zh/guide/choose-a-target)。
- 当您准备好扩展第一条路径之外时，继续[一个项目，多个目标](/zh/guide/one-project-multiple-targets)。

</details>
