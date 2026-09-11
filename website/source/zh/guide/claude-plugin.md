---
title: "构建 Claude 插件"
description: "plugin-kit-ai 中稳定的 Claude 插件路径的重点指南。"
canonicalId: "page:guide:claude-plugin"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">构建 Claude 插件</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 构建 Claude 插件

当您显式定位 Claude 挂钩而不是默认的 Codex 运行时路径时，请选择此路径。

## 推荐起点

```bash
plugin-kit-ai init my-claude-plugin --platform claude
cd my-claude-plugin
plugin-kit-ai generate .
plugin-kit-ai validate . --platform claude --strict
```

## 这条路径的含义

- 项目目标 Claude 钩子执行
- 稳定子集比完整的 Claude 运行时功能集更窄
- `validate --strict` 仍然是主要的准备情况检查

## 谨慎使用扩展钩子

```bash
plugin-kit-ai init my-claude-plugin --platform claude --claude-extended-hooks
```

仅当您有意想要更广泛的支持集并且您接受比稳定子集更宽松的稳定性时，才选择扩展挂钩。

## 何时适合这条路径

- 必须与 Claude 运行时挂钩集成的插件
- 需要一个存储库和一个工作流程而不是手动编辑本机 Claude 工件的团队
- 需要比临时本地脚本更强大的结构的用户

## 后续步骤

- 阅读 [目标模型](/zh/concepts/target-model) 以了解 Claude 与打包或工作区配置目标有何不同。
- 检查[平台事件](/zh/api/platform-events/claude)以获取事件级参考。

</details>
