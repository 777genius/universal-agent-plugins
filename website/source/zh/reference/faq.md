---
title: "FAQ"
description: "对团队在启动和扩展 plugin-kit-ai 存储库时最常提出的问题的简短回答。"
canonicalId: "page:reference:faq"
section: "reference"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# FAQ

## 我应该从 Go、Python 还是 Node 开始？

从 Go 开始，除非您有真正的理由不这样做。

选择 Node/TypeScript 作为主要支持的非 Go 路径。当插件位于存储库本地并且您的团队已经是 Python 时，请选择 Python。

## 最简单的 Python 设置是什么？

首先使用默认的 Python 脚手架：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

然后编辑插件，重新生成并再次验证。

请参阅[构建 Python 运行时插件](/zh/guide/python-runtime)。

## 我什么时候应该使用 `--runtime-package`？

仅当您有意希望跨多个存储库共享一个辅助依赖项时，才使用 `--runtime-package` 。

大多数团队应该首先从默认的本地助手开始。

## npm 和 PyPI 上的 `plugin-kit-ai` 是 runtime API 吗？

不是。它们安装的是 CLI，不是 runtime API，也不是 SDK。

## 我什么时候应该使用 bundle 命令？

当另一台机器需要可获取或可安装的 Python 或 Node 工件时，使用 bundle 命令。

不要把 bundle 交付和主 CLI 安装路径混为一谈。

## 我可以保留本机目标文件作为我的事实来源吗？

不会。预期的长期模型是把真实来源保留在包标准布局中，并把目标文件视为生成输出。

## `generate` 是可选的吗？

不是。如果你要走托管项目工作流，`generate` 就是流程中的必需步骤。

## `validate --strict` 是可选的吗？

将其视为主要的准备情况检查，尤其是对于本地 Python 和 Node 运行时存储库。

## 一个仓库可以拥有多个目标吗？

是的。

实际规则是：

- 把 authored state 保留在一个托管仓库中
- 从你今天真正需要的主目标开始
- 仅在出现实际产品、交付或集成需求时添加更多目标

请参阅[一个项目，多个目标](/zh/guide/one-project-multiple-targets) 和 [目标模型](/zh/concepts/target-model)。

## 所有目标都同样稳定吗？

不。

不同的路径承载着不同的支持承诺。使用 [支持边界](/zh/reference/support-boundary) 作为简短答案，使用 [目标支持](/zh/reference/target-support) 作为精确矩阵。

</details>
