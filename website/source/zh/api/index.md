---
title: "API"
description: "为 plugin-kit-ai 生成了 API 参考。"
canonicalId: "page:api:index"
section: "api"
locale: "zh"
generated: false
translationRequired: true
aside: false
outline: false
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">API</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

<div class="docs-hero docs-hero--compact">
  <p class="docs-kicker">生成的参考</p>
  <h1>API 接口总览</h1>
  <p class="docs-lead">
    本部分汇总 plugin-kit-ai 的公开 API：CLI、Go SDK、runtime helper、平台事件和能力。
  </p>
</div>

<div class="docs-grid">
  <a class="docs-card" href="./cli/">
    <h2>CLI</h2>
    <p>从实时 Cobra 树导出的命令.</p>
  </a>
  <a class="docs-card" href="./go-sdk/">
    <h2>Go SDK</h2>
    <p>面向生产就绪 runtime 插件的公开 Go 包。</p>
  </a>
  <a class="docs-card" href="./runtime-node/">
    <h2>Node 运行时</h2>
    <p>面向 JS 和 TS 使用者的类型化 runtime helper。</p>
  </a>
  <a class="docs-card" href="./runtime-python/">
    <h2>Python 运行时</h2>
    <p>仅包含公开的 Python runtime helper，不包含安装包装器。</p>
  </a>
  <a class="docs-card" href="./platform-events/">
    <h2>平台事件</h2>
    <p>按目标平台分组的事件接口。</p>
  </a>
  <a class="docs-card" href="./capabilities/">
    <h2>能力</h2>
    <p>跨平台和事件维度整理的能力视图。</p>
  </a>
</div>

## 打开正确的接口面

- 当您需要命令、标志或创作工作流程时，打开 `CLI`。
- 当您在 Go 中构建生产就绪的 runtime 插件时，打开 `Go SDK`。
- 当您需要仓库本地 runtime 的共享 helper API 时，打开 `Node Runtime` 或 `Python Runtime`。
- 当您选择特定目标事件时，打开 `Platform Events`。
- 当您想查看跨平台存在哪些操作和扩展点时，请打开 `Capabilities`。

## API 部分涵盖的内容

- 实时 Cobra 命令树
- 公共 Go 包
- Node 和 Python 的共享 runtime helper
- 特定于平台的事件
- 能力层级的跨平台元数据

</details>
