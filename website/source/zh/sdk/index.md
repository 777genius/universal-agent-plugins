---
title: "SDK 与集成"
description: "选择 hooks、主机识别、安装及插件管理的合适 SDK。"
canonicalId: "page:sdk:index"
section: "sdk"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK 与集成

当你需要**把 Universal Agent Plugins 嵌入自己的应用**，或处理 Agent 原生 hook 事件时，请使用这些 SDK。普通的插件安装请参阅[使用插件](../use/)。

## 选择正确的 SDK

| 目标 | 指南 | 入口 |
| --- | --- | --- |
| 处理 Agent 原生事件 | [Go Hooks](./go-hooks) | `github.com/777genius/plugin-kit-ai/sdk` |
| 判断哪个 Agent 调用了 hook | [主机识别](./host-detection) | `sdk/hostdetect` |
| 在 Go 应用中安装、更新、修复和恢复 | [Go Installer](./go-installer) | `install/integrationctl/agentplugins/installer` |
| 在 Node/Electron 中管理插件 | [TypeScript 管理 SDK](./typescript-management) | `universal-agent-plugins-management-sdk` |

## 版本与支持范围

- **Go Hooks SDK**：已发布模块 `github.com/777genius/plugin-kit-ai/sdk@v1.3.0`，稳定性按**具体事件**区分。
- **主机识别**：`sdk/hostdetect` 存在于当前源码；使用时应确认**特定发布标签**包含此包。
- **Go Installer**：独立模块和 API，需锁定审查过的提交及依赖；不能沿用 Hooks SDK 的版本。
- **TypeScript 管理 SDK**：源码包版本 **0.1.1**，要求 Node.js **22.18+**，使用 Management API **v1**；安装前请验证 npm 上的版本。
- **Go 管理引擎**：CLI **0.1.95+** 提供 `manage serve` 和 `manage discover`。TS SDK 的 `checkUpdates` **不受 CLI 0.1.95 及更早版本支持**。

`hostdetect.Detect` 识别**正在调用 hook 的 Agent**；`discoverTargets` 检查**已安装的客户端及其私有配置位置**；`Engine.Discover` 列出 Go 中显式注册的适配器。这些都不会自动安装插件。包校验、文件写入、原生激活及 MCP 身份验证必须分别验证。

继续阅读：[Go Hooks](./go-hooks)、[主机识别](./host-detection)、[Go Installer](./go-installer)、[TypeScript 管理](./typescript-management)。
