---
title: "v1.0.4 Go SDK"
description: "Go SDK 模块路径修正的补丁发行说明。"
canonicalId: "page:releases:v1-0-4-go-sdk"
section: "releases"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# v1.0.4 Go SDK

发布日期：`2026-03-29`

## 为什么这个补丁很重要

此补丁使公共 Go SDK 模块路径对于正常 Go 消耗来说是真实的。

## 发生了什么变化

- Go SDK 模块根从 `sdk/plugin-kit-ai/` 移至 `sdk/`
- 公共模块路径 `github.com/777genius/plugin-kit-ai/sdk` 现在与真实的存储库布局匹配
- 更新了入门存储库、示例和模板，以停止教授基于 `replace` 的新手解决方法

## 实用指导

- 使用 `github.com/777genius/plugin-kit-ai/sdk@v1.0.4` 或更新版本进行正常 Go 模块消耗
- 将 `v1.0.3` 视为 Go SDK 模块路径的已知错误

## 为什么用户应该关心

该补丁减少了普通 Go 消费者的摩擦，并使推荐的 SDK 路径看起来像普通的公共模块，而不是特殊情况的解决方法。
</details>
