---
title: "构建 Node/TypeScript 运行时插件"
description: "本地运行时插件主要支持的非 Go 路径。"
canonicalId: "page:guide:node-typescript-runtime"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">构建 Node/TypeScript 运行时插件</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 构建 Node/TypeScript 运行时插件

当您的团队需要 TypeScript 但仍需要受支持的本地运行时插件时，这是主要受支持的非 Go 路径。

## 推荐流程

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## 要记住什么

- 这是稳定的本地运行时路径，而不是零运行时依赖性 Go 路径
- 执行机仍然需要 Node.js `20+`
- `doctor` 和 `bootstrap` 在这里比默认的 Go 路径更重要

## 当这是正确的选择时

- 您的团队已经在 TypeScript 中工作
- 该插件按设计保留在存储库本地
- 你想要主要支持的非 Go 路径而不落入 beta 逃生舱口

## 当 Go 更好时

在以下情况下更喜欢使用 Go ：

- 你想要最强的制作合同
- 您希望下游用户避免安装 Node
- 您希望 CI 和其他机器上的引导摩擦最小

有关下一层的详细信息，请参阅[选择运行时](/zh/concepts/choosing-runtime) 和 [Node 运行时 API](/zh/api/runtime-node/)。
</details>
