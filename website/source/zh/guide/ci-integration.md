---
title: "CI 集成"
description: "将公共创作流程转变为 plugin-kit-ai 项目的稳定 CI 门。"
canonicalId: "page:guide:ci-integration"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# CI 集成

最稳妥的 CI 流程并不复杂，关键在于严格遵守公共契约。

<MermaidDiagram
  :chart='`
flowchart LR
  Doctor[doctor] --> Bootstrap[按需 bootstrap]
  Bootstrap --> Generate[generate]
  Generate --> Validate[validate --strict]
  验证 --> 烟雾[烟雾或捆绑检查]
`'
/>

## 最小 CI 门

对于大多数编写的项目，这是基线：

```bash
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform <target> --strict
```

如果您的通道具有稳定的冒烟测试或捆绑检查，请将它们添加到验证门之后，而不是替换它。

## 为什么这有效

- `doctor` 尽早捕获缺少的运行时先决条件
- `generate` 证明生成的输出可以从创作状态重现
- `validate --strict` 证明该存储库对于所选目标内部是一致的
- 对于多目标存储库，相同的逻辑应该适用于支持范围内的每个目标

## 运行时特定注释

### Go

Go 是最简洁的 CI 路径，因为执行环境不需要为了运行时通道额外安装 Python 或 Node。

对于基于 launcher 的 Go 仓库，请先构建要检查的 launcher：

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Node/TypeScript

请显式加入 bootstrap 步骤：

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Python

使用与 Node 相同的模式，并在 CI 中显式显示 Python 版本。

## 常见的 CI 错误

- 在没有 `generate` 的情况下运行 `validate --strict`
- 将生成的工件视为手动维护的文件
- 忘记 Node 或 Python 通道的运行时先决条件
- 期待超出稳定支持边界的目标也能自动兼容

## 推荐规则

如果 CI 不能重现源码生成结果并通过 `validate --strict`，那么这个仓库就还没有准备好进行稳定交付。对于多目标仓库，这意味着支持范围内的每个目标都必须明确跑绿。

将此页面与[生产准备](/zh/guide/production-readiness)、[支持边界](/zh/reference/support-boundary) 和[故障排除](/zh/reference/troubleshooting) 配对。

</details>
