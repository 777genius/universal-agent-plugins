---
title: "如何发布插件"
description: "将 plugin-kit-ai 项目发布到 Codex、Claude 和 Gemini 的实用指南，避免将本地应用与发布计划混淆。"
canonicalId: "page:guide:how-to-publish-plugins"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 如何发布插件

当您的存储库已在 `plugin-kit-ai` 中编写，并且您希望为 Codex、Claude 或 Gemini 发布提供最清晰的下一步时，请使用本指南。

请从已经通过 `plugin-kit-ai generate .` 和 `plugin-kit-ai validate . --strict` 的仓库开始，这样发布命令读取的就是最新的托管产物，而不是过期文件。

## 本指南涵盖的内容

- 哪些平台今天支持真正的本地落地
- 哪些平台目前只支持计划与就绪性检查
- 应该先运行哪个命令
- 命令完成后应该看到什么结果

## 快速比较

|平台|发布模型|当前是否真正可用|主命令|产物|
|---|---|---:|---|---|
| Codex |本地 marketplace 根目录|是| `publish --channel codex-marketplace` | `.agents/plugins/marketplace.json` 和 `plugins/<name>/...` |
| Claude |本地 marketplace 根目录|是| `publish --channel claude-marketplace` | `.claude-plugin/marketplace.json` 和 `plugins/<name>/...` |
| Gemini |仓库发布准备|否| `publish --channel gemini-gallery --dry-run` |有界的发布计划和就绪性诊断|

## 简短规则

- 需要发布工作流时使用 `publish`
- 想先检查或诊断时使用 `publication`
- Codex 和 Claude 支持立即落地到本地 marketplace
- Gemini 在 v1 中只提供计划与就绪性检查，不做本地应用

仓库结构保持不变：

- `plugin.yaml` 是核心插件清单
- `targets/...` 保存特定于目标的创作输入
- `publish/...` 持有发布意图
- `publication` 是检查与诊断入口
- `publish` 是发布工作流入口

## 发布到 Codex

对于 Codex，发布意味着落地到本地 marketplace 根目录。

首先运行这个：

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace --dry-run
```

当计划看起来正确时再执行实际落地：

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace
```

预期结果：

- `.agents/plugins/marketplace.json`
- `plugins/<name>/...`

这样的本地根目录已经可以作为 Codex 插件来源。

## 发布到 Claude

对于 Claude，发布同样意味着落地到本地 marketplace 根目录。

首先运行这个：

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace --dry-run
```

当计划看起来正确时再执行实际落地：

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace
```

预期结果：

- `.claude-plugin/marketplace.json`
- `plugins/<name>/...`

## 发布到 Gemini

对于 Gemini，发布并不意味着建立本地 marketplace 根目录。

在 v1 中，`plugin-kit-ai` 只做三件有界的事情：

- 验证发布意图
- 检查存储库准备情况
- 生成发布计划

从准备开始：

```bash
plugin-kit-ai publication doctor ./my-plugin --target gemini
```

然后检查发布计划：

```bash
plugin-kit-ai publish ./my-plugin --channel gemini-gallery --dry-run
```

预期先决条件：

- 公共 GitHub 存储库
- 有效的 `origin` 远程指向 GitHub
- GitHub 主题 `gemini-cli-extension`
- `gemini-extension.json` 在正确的根目录中

Gemini 在 v1 中使用计划与就绪性发布，而不是本地应用。

## 跨所有已编写渠道统一规划

当一个存储库作者有多个发布渠道时使用此选项：

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --dest ./local-marketplaces --format json
```

重要规则：

- 它只使用已编写的 `publish/...` 渠道
- 它不会从 `targets` 推断渠道
- v1 只做规划
- 只有当已编写渠道里包含 Codex 或 Claude 本地 marketplace 流时才需要 `--dest`
- 纯 Gemini 规划不需要 `--dest`

如果存储库作者只有 `gemini-gallery`，这也有效：

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --format json
```

## 我应该运行哪个命令？

- 我想要本地 Codex 市场根： `plugin-kit-ai publish --channel codex-marketplace --dest <marketplace-root>`
- 我想要本地 Claude 市场根： `plugin-kit-ai publish --channel claude-marketplace --dest <marketplace-root>`
- 我想要 Gemini 发布就绪性检查：`plugin-kit-ai publication doctor --target gemini`
- 我想要 Gemini 发布计划：`plugin-kit-ai publish --channel gemini-gallery --dry-run`
- 我想要一个组合发布计划：`plugin-kit-ai publish --all --dry-run`，并在包含 Codex 或 Claude 创作频道时添加 `--dest <marketplace-root>`

## 进一步阅读

- [CLI 自述文件发布部分](https://github.com/777genius/plugin-kit-ai/tree/main/cli/plugin-kit-ai)
- [`plugin-kit-ai publish`](/zh/api/cli/plugin-kit-ai-publish)
- [`plugin-kit-ai publication`](/zh/api/cli/plugin-kit-ai-publication)
- [`plugin-kit-ai publication doctor`](/zh/api/cli/plugin-kit-ai-publication-doctor)

</details>
