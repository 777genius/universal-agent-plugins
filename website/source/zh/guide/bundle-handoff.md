---
title: "捆绑交接"
description: "如何导出、安装、获取和发布可移植的 Python 和 Node 捆绑包以支持切换流程。"
canonicalId: "page:guide:bundle-handoff"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 捆绑交接

当 Python 或 Node 插件应作为便携式工件而不是作为实时存储库签出时，请使用本指南。

这是真正的公共功能，但故意比主 Go 路径更窄。

## 涵盖内容

稳定的捆绑切换子集用于：

- 在 `codex-runtime` 和 `claude` 上导出 `python` 捆绑包
- 在 `codex-runtime` 和 `claude` 上导出 `node` 捆绑包
- 本地捆绑安装
- 远程捆绑获取
- GitHub 发布捆绑包

在以下情况下，这是正确的选择：

- 另一个团队应该收到现成的工件而不是您的完整存储库
- 您的发布流程已使用 GitHub 发布
- 您想要 Python 或 Node 运行时的更清晰的交接故事

## 实际流程

生产方是：

```bash
plugin-kit-ai export . --platform <codex-runtime|claude>
plugin-kit-ai bundle publish . --platform <codex-runtime|claude> --repo <owner/repo> --tag <tag>
```

消费者方是：

```bash
plugin-kit-ai bundle install <bundle.tar.gz> --dest <path>
```

或：

```bash
plugin-kit-ai bundle fetch <owner/repo> --tag <tag> --platform <codex-runtime|claude> --runtime <python|node> --dest <path>
```

安装或获取后，生成的存储库仍然需要正常的运行时引导和准备情况检查。

## 什么不会自动发生

`bundle install` 和 `bundle fetch` 不会默默地将捆绑包转变为完全验证的插件。

将已安装的捆绑包视为下游设置的开始：

1.安装运行时必备软件
2.运行`plugin-kit-ai doctor .`
3. 运行任何所需的引导步骤
4.运行`plugin-kit-ai validate . --platform <target> --strict`

## 当捆绑交付比实时存储库更好时

在以下情况下选择捆绑切换：

- 发布工件才是真正的交付合约
- 下游消费者不应克隆源存储库
- 您想要 Python 或 Node 通道的可重复 GitHub 版本发行版

在以下情况下保持实时存储库路径：

- 团队仍然直接编辑项目源码
- 主要需求是在一个存储库内进行协作
- Go 已经为您提供了所需的干净编译二进制切换

## 重要边界

捆绑传递并不是“针对每个目标的通用打包”。

它是 `codex-runtime` 和 `claude` 上导出的 Python 和 Node 子集的受支持的便携式切换流程。

不要假设同一合同适用于：

- Go SDK 存储库
- 工作区配置目标，例如 Cursor 或 OpenCode
- 仅打包目标，例如 Gemini
- CLI 安装包

## 推荐阅读顺序

将此页面与[选择交付模型](/zh/guide/choose-delivery-model)、[生产准备情况](/zh/guide/production-readiness) 和[支持边界](/zh/reference/support-boundary) 配对。

</details>
