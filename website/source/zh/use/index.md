---
title: "使用插件"
description: "查找、预览、安装和管理标准插件包。"
canonicalId: "page:use:index"
section: "use"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 使用插件

Universal Agent Plugins 为您选择的 AI 客户端安装标准包。包可以包含技能、MCP 服务器配置，或两者兼有。
您可以为用户账户安装，也可以将某个项目的依赖与用户环境分开管理。

## 获取 CLI

安装 Node.js 22 或更高版本后，可直接运行 npm 包装器：

```bash
npx universal-agent-plugins search docs
```

经常使用时可全局安装：

```bash
npm install -g universal-agent-plugins
agentplugins version
```

也可通过 Homebrew 安装：

```bash
brew install 777genius/agentplugins/agentplugins
```

[原生安装指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)涵盖 macOS、Linux 和 Windows。
原生管理器不依赖 Node.js，但各插件可能需要运行时。本指南使用 `agentplugins`；
`npx universal-agent-plugins` 可运行相同的安装器命令。

## 按步骤安装

1. [查找并检查包](./install)，了解来源和前提条件。
2. 使用 `--dry-run` 预览目标客户端、安装范围和变更。
3. 应用已检查的安装计划，并完成激活步骤。
4. 使用 list、update、repair 和 remove [维护安装状态](./manage)。
5. 需要时从 `uap.toml` 和 `uap.lock` 恢复共享依赖集合。

[CLI 0.1.93](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93)已支持显式项目范围。
即使当前目录存在 `uap.toml`，已发布版本仍默认使用用户范围。每次项目操作都要指定 `--scope project`。
各客户端支持的项目组件和激活要求不同。

## 分别查看每个目标的结果

安装后仍可能需要重新加载、工作区信任、手动激活或 OAuth。`Prepared` 表示仍有客户端步骤未完成。
请按目标查看结果，不要将整次操作完成等同于运行成功。[兼容性参考](../reference/client-compatibility)说明版本化原生观察的证据范围。

## 如果您要编写包

从[构建插件](../build/)开始。交付物是包含根目录 `plugin.json` 及其组件的文件夹；
安装者不需要运行源码创作工具。

| 命令 | 用途 |
| --- | --- |
| `agentplugins validate ./my-plugin` | 为安装读取并验证本地包。 |
| `agentplugins doctor` | 诊断安装器管理的状态。 |
| `agentplugins author validate ./my-plugin` | 在启用创作功能的源码构建中检查符合性和就绪情况。 |
| `agentplugins author doctor ./my-plugin` | 在该源码构建中检查项目和工具链证据。 |

当前 npm 和原生安装器版本不包含 `agentplugins author`。
