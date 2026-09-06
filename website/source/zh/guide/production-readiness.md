---
title: "生产准备情况"
description: "用于确定 plugin-kit-ai 项目是否已准备好进行 CI、移交和广泛共享的公共检查表。"
canonicalId: "page:guide:production-readiness"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 生产准备情况

在您将项目称为生产就绪、移交就绪或准备广泛展示之前，请使用此清单。

<MermaidDiagram
  :chart='`
flowchart LR
  path[有意识地选择路径] --> source[一个源仓库]
  source --> checks[Generate 与 validate 门]
  checks --> boundary[支持边界已确认]
  boundary --> handoff[文档与交接都很明确]
  handoff --> ready[项目已准备好进入生产]
`'
/>

## 1. 有目的地选择正确的道路

- 当您想要最强的运行时通道时，默认为 Go
- 当非 Go 本地运行时权衡是真实的时，选择 Node/TypeScript 或 Python
- 仅当您需要真正的输出时才选择包、扩展或集成通道

## 2. 保持一个仓库的诚实

- 将项目源代码保留在包标准布局中
- 将生成的目标文件视为输出，而不是您编辑的主要位置
- 不要手动修补生成的文件并期望 `generate` 保留这些编辑

## 3. 运行合约门

至少，回购应该能够干净地度过这个流程：

```bash
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform <target> --strict
```

对于基于 launcher 的 Go 通道，请先构建 `bin/<name>`，这样 launcher 入口点会先出现在磁盘上：

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

对于 Python 和 Node 运行时通道，`doctor` 和 `bootstrap` 是准备状态的一部分。

## 4. 验证确切的支撑边界

- 确认主要车道和范围内的每个附加车道均位于公共支持边界内
- 当您需要准确的 `public-stable`、`public-beta` 或 `public-experimental` 术语时，请使用参考页
- 在向下游用户承诺兼容性之前检查生成的目标支持矩阵

## 5. 将安装故事和 API 故事分开

- Homebrew、npm 和 PyPI 软件包是 CLI 的安装通道
- 它们不是运行时 APIs 或 SDK 表面
- public API 存在于生成的 API 部分和记录的工作流程中

## 6. 记录交接

面向公众的存储库应该使这些事情变得显而易见：

- 哪条车道是主要车道
- 真正支持哪些额外车道
- 它使用哪个运行时以及是否随目标而改变
- 哪个命令集是规范验证门
- 是否依赖于共享运行时包或 Go SDK 路径

## 最终规则

如果队友无法克隆存储库、运行记录的流程、传递 `validate --strict` 并在没有部落知识的情况下理解所选通道，则该项目尚未准备好投入生产。

</details>
