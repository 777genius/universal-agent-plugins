---
title: "选择一个目标"
description: "实用的公共指南，用于选择与您想要的插件交付方式相匹配的目标。"
canonicalId: "page:guide:choose-a-target"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 选择一个目标

当您已经知道需要 `plugin-kit-ai` 时，请使用此页面，但您仍然需要将存储库与您想要发送插件的方式相匹配。

选择目标意味着选择产品今天真正需要的主要路径，而不是把仓库永久锁死。

<MermaidDiagram
  :chart='`
flowchart TD
  Need["产品现在真正需要什么？"] --> Exec{"可执行行为"}
  Need --> Artifact{"包或扩展产物"}
  Need --> Config{"仓库自管集成配置"}
  Exec --> Codex["codex-runtime"]
  Exec --> Claude["claude"]
  Artifact --> CodexPackage["codex-package"]
  Artifact --> Gemini["gemini"]
  Config --> OpenCode["opencode"]
  Config --> Cursor["cursor"]
`'
/>

## 短规则

- 当您想要最强的默认运行时路径时，选择 `codex-runtime`
- 当 Claude 挂钩是实际产品要求时，选择 `claude`
- 当产品是官方 Codex 包时，选择 `codex-package`
- 当产品是 Gemini 扩展包时，选择 `gemini`
- 当存储库应拥有集成/配置设置时，选择 `opencode` 或 `cursor`

## 目标目录

|目标|何时选择|所属车道|
| --- | --- | --- |
| `codex-runtime` |您想要默认的可执行插件路径 |推荐的 runtime 路径 |
| `claude` |您明确需要 Claude hooks |推荐的 Claude 路径 |
| `codex-package` |您需要 Codex 打包输出 |推荐的包路径 |
| `gemini` |您要交付 Gemini 扩展包 |推荐的扩展路径 |
| `opencode` |您需要仓库自管的 OpenCode 集成配置 |仓库自管的集成配置 |
| `cursor` |您需要仓库自管的 Cursor 集成配置 |仓库自管的集成配置 |

## 安全默认值

如果您不确定，请从 `codex-runtime` 和默认 Go 路径开始。

在您选择更窄或更专业的路径之前，这为您提供了最干净的生产起点。

当您稍后移动到 `codex-package` 时，官方包通道将遵循官方 `.codex-plugin/plugin.json` 包布局。

如果您有意开始使用受支持的 Node/TypeScript 或 Python，则会改变语言选择，而不需要在第一天就决定每个打包或集成细节。

## 当您需要多个目标时该怎么办

- 选择定义当今产品的主要路径
- 保持仓库统一
- 仅当出现实际交付或集成需求时才添加更多目标

当您想要更广泛的多目标心智模型时，请阅读[一个项目，多个目标](/zh/guide/one-project-multiple-targets)。

</details>
