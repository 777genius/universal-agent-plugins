---
title: "故障排除"
description: "针对最常见的安装、生成、验证和引导问题的快速恢复步骤。"
canonicalId: "page:reference:troubleshooting"
section: "reference"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 故障排除

当工作流程停止移动时使用此页面。首先从最简单的检查开始。

## CLI 安装但不运行

检查二进制文件是否确实位于您的 shell `PATH` 上。

如果您通过 npm 或 PyPI 安装，请确保包实际下载了已发布的二进制文件。不要将包装器包本身视为运行时。

## Python 或 Node 运行时项目提前失败

首先检查真实的运行时间：

- Python 运行时存储库需要 Python `3.10+`
- Node 运行时存储库需要 Node.js `20+`

在假设存储库本身已损坏之前，请使用 `plugin-kit-ai doctor <path>` 。

典型回收流程：

```bash
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## `validate --strict` 失败

将其视为信号，而不是噪音。

常见原因：

- 生成的工件已过时，因为 `generate` 被跳过
- 选择的平台与项目来源不匹配
- 运行时路径仍然需要引导程序或环境修复

## `generate` 输出看起来与预期不同

这通常意味着项目来源和你的思维模型背道而驰。

重新检查包标准布局，而不是手动编辑生成的目标文件以强制输出您期望的输出。

## 我不确定应该使用哪条路径

如果您想要最强的合约，请从默认的 Go 路径开始。

仅当本地运行时权衡是真实且有意的时，才转向 Node/TypeScript 或 Python 。

请参阅[构建 Python 运行时插件](/zh/guide/python-runtime)、[创作工作流程](/zh/reference/authoring-workflow) 和 [FAQ](/zh/reference/faq)。
</details>
