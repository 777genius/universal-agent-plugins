---
title: "选择交付模式"
description: "如何在 Python 和 Node 插件的供应帮助程序和共享运行时包之间进行选择。"
canonicalId: "page:guide:choose-delivery-model"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">选择交付模式</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 选择交付模式

Python 和 Node 插件有两种受支持的方式来传送帮助程序逻辑。他们解决不同的实际问题。

<MermaidDiagram
  :chart='`
flowchart TD
  Start[Python or Node plugin] --> Shared{需要跨存储库的一个可重用依赖项}
  共享-->|是|包[共享运行时包]
  共享-->|否|平滑{需要最平滑的自包含启动}
  平滑-->|是|售卖[售卖助手]
  平滑-->|否|套餐
`'
/>

## 快速实用规则

如果您今天只想使用最简单的 Python 或 Node 存储库，请首先使用默认脚手架。

如果您已经知道多个存储库应共享一个辅助依赖项，请从 `--runtime-package` 开始。

## 两种模式

- `vendored helper`：默认脚手架将帮助程序文件写入存储库本身
- `shared runtime package`：`--runtime-package` 导入 `plugin-kit-ai-runtime` 作为依赖项，而不是将帮助程序写入 `plugin/`

## 两种模式下的同一个项目

默认本地帮助程序路径：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

共享包路径：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

## 选择供应商助手时

- 你想要最顺畅的首次运行路径
- 你希望仓库保持独立
- 您希望帮助器实现在存储库中可见
- 您的团队尚未标准化一个共享的 PyPI 或 npm 帮助程序版本

这是默认值，因为它是 Python 和 Node 项目最简单的起点。

## 选择共享运行时包时

- 您想要跨多个插件存储库的一个可重用帮助器依赖项
- 您更喜欢通过正常的软件包版本升级来升级帮助程序行为
- 您的团队可以轻松地将版本固定在 `requirements.txt` 或 `package.json` 中
- 您已经知道存储库从第一天起就应该遵循共享依赖路径

## 人们在实践中通常意味着什么

- 当主要目标是“让一个仓库快速运行”时，选择供应商的助手
- 当主要目标是“跨存储库重用相同的帮助程序包”时，选择共享运行时包
- 不要仅仅因为共享包听起来更像制作而选择它；它不会从执行机中删除 Python 或 Node 运行时要求

## 哪些内容不会改变

- 当您想要最强的生产路径时，Go 仍然是推荐的默认值
- Python 在执行机上仍然需要 Python `3.10+`
- Node 仍然需要执行机上的 Node.js `20+`
- `validate --strict` 仍然是主要的准备情况检查
- CLI 安装包仍然不会成为运行时 APIs

## 推荐团队政策

- 当您想要最强的长期支持路径时，选择 Go
- 当您想要最顺利的 Python 或 Node 启动时，选择供应的助手
- 当您已经知道需要跨存储库的可重用依赖策略时，选择共享运行时包将此页面与[构建Python运行时插件](/zh/guide/python-runtime)、[选择入门存储库](/zh/guide/choose-a-starter)、[入门模板](/zh/guide/starter-templates)和[生产准备](/zh/guide/production-readiness)配对。

</details>
