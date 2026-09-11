---
title: "创作工作流程"
description: "主要工作流程从初始化到生成、验证、测试和切换。"
canonicalId: "page:reference:authoring-workflow"
section: "reference"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">创作工作流程</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 创作工作流程

推荐的工作流程故意简单：

```text
init -> generate -> validate --strict -> test -> handoff
```

<MermaidDiagram
  :chart='`
flowchart LR
  Init[init] --> 生成[生成]
  生成 --> 验证[validate --strict]
  验证 --> 测试[测试或冒烟检查]
  测试 --> 切换[handoff]
  Bootstrap[按需运行 doctor 或 bootstrap] -. 支持 .-> 生成
  Bootstrap -. 支持 .-> 验证
`'
/>

## 每一步的含义

|步骤|目的|
| --- | --- |
| `init` |创建符合 package 标准的项目布局 |
| `generate` |从项目源码生成目标产物 |
| `validate --strict` |运行主要就绪性检查 |
| `test` |在适用时运行稳定的冒烟测试 |
| `export` / bundle flow |为受支持的 Python 和 Node 场景生成交付产物 |

## 保持仓库健康的规则

- 项目源位于包标准项目布局中
- 生成的目标文件是输出，而不是长期的事实来源
- 严格验证是必需检查，不是可选附加项

此工作流程对于单目标和多目标存储库同样重要。

唯一的区别是，在多目标项目中，对于存储库实际承诺支持的每个目标都会重复 `generate` 和 `validate` 循环。

## 当工作流程发生变化时

对于特殊情况，工作流程可以扩大：

- `doctor` 和 `bootstrap` 对于 Python 和 Node 运行时路径很重要
- 将手动管理的目标文件合并到托管项目模型中时，`import` 和 `normalize` 很重要
- 打包相关命令对于可移植的 Python 和 Node 交付流程很重要

当您需要最短路径时，请从[快速入门](/zh/guide/quickstart)开始。

</details>
