---
title: "选择运行时"
description: "如何在 Go、Python、Node 和 shell 创作路径之间进行选择。"
canonicalId: "page:concepts:choosing-runtime"
section: "concepts"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 选择运行时

运行时选择不仅仅是语言偏好。它改变了插件的运行方式、执行机必须安装的内容以及 CI 和切换的简单程度。

<MermaidDiagram
  :chart='`
flowchart TD
  Start[Need a runtime lane] --> Prod{需要最强的运行时通道}
  产品-->|是| Go[去]
  产品-->|否|本地{插件存储库按设计是本地的}
  本地 -->|是|团队{是团队 Python 优先还是 Node 优先}
  团队 --> Python[python]
  团队 --> Node[节点或节点 --typescript]
  本地 -->|否|逃生{只需要一个逃生舱口}
  转义 --> 外壳[外壳]
`'
/>

## 选择 Go 时

- 你想要最强的运行车道
- 你想要类型化的处理程序和最干净的发布故事
- 您希望 CI 和其他机器上的引导摩擦最小

## 选择 Python 或 Node 时

- 该插件的设计是存储库本地的
- 你的团队已经生活在那个运行时
- 您接受自己拥有运行时引导程序
- 您对 Python `3.10+` 或 Node.js `20+` 出现在执行机上感到满意

## 仅当以下情况时才选择 Shell

- 你需要一个狭窄的逃生舱口
- 您明确接受实验性或高级权衡

## 安全默认矩阵

|情况|推荐选择|
| --- | --- |
|最强运行车道| `go` |
|主要非Go运行时通道| `node --typescript` |
|本地Python-一线队| `python` |
|逃生舱口 | `shell` |

</details>
