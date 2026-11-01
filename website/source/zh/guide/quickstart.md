---
title: "快速入门"
description: "安装现有插件，或准备一个可移植 Agent Plugins 1.0 包。"
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

<a id="quickstart"></a>
<a id="快速入门"></a>

# 使用插件 / 构建插件

为您使用的客户端安装包，或创建包含操作说明和工具配置的可移植 Agent Plugins 1.0 包。

## 使用插件 {#use-plugins}

<a id="optional-first-proof"></a>

安装 Node.js 22 或更高版本后，运行：

```bash
npx universal-agent-plugins add context7
```

CLI 会询问所选兼容客户端，并报告剩余激活步骤。也可先检查和预览：

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins add context7 --target codex,cursor --dry-run
```

[原生安装指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)提供 macOS、Linux 和 Windows 可执行文件。
原生管理器不需要 Node.js，但插件可能需要自己的运行时。

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95)保留显式项目范围和 0.1.94 引入的自动安装选择，并新增[本地管理 API](../use/manage#local-management-api)和[Pi 原生扩展](../reference/client-compatibility#pi-native-extensions)。预览将包添加到项目：

对于符合条件的已审核目录条目，请复制其固定到完整 Git 提交 SHA 的来源和兼容客户端。Directory v1 的别名（如 `context7`）仅适用于用户范围。请替换 `OWNER/REPOSITORY`、`FULL_COMMIT_SHA` 和 `PACKAGE_PATH`；如果包位于仓库根目录，请省略 `//PACKAGE_PATH`。

```bash
npx universal-agent-plugins add 'github:OWNER/REPOSITORY@FULL_COMMIT_SHA//PACKAGE_PATH' --target codex --scope project \
  --project-root /path/to/project --dry-run
```

选择已存在的项目目录。add 和维护命令默认使用用户范围，应明确指定项目范围。
未指定来源参数的 `install` 则在当前目录存在普通 `uap.toml` 时自动选择项目，不向上查找；显式范围优先，已有锁文件会被严格检查。

各分发渠道的版本可能不同。使用 0.1.94 的自动项目选择前，先运行 `agentplugins version` 或
`npx universal-agent-plugins version`。若所选渠道仍较旧，可使用[原生安装器](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)，
或继续使用 0.1.93 已支持的显式项目范围命令。

检查计划后再去掉 `--dry-run`。写入包文件不等于客户端已发现、授予信任、完成认证或成功调用 MCP。

[完整安装流程](../use/install) · [客户端兼容性与测试边界](../reference/client-compatibility)

### 恢复已保存的依赖集合

先检查所选用户环境的记录：

```bash
npx universal-agent-plugins deps status
```

对于已保存的 `uap.toml` / `uap.lock`，在交换文件夹导出：

```bash
npx universal-agent-plugins deps export
```

将该文件夹复制到另一台机器，安装目标客户端，再在该目录运行：

```bash
npx universal-agent-plugins deps import
npx universal-agent-plugins install --locked --scope user
```

只转移经过验证的清单和锁文件；配置目录、设置、凭证、收据和插件数据需要单独处理。
安装仍需确切来源或已验证缓存。项目应提交这两个文件，并以显式 `install --locked --scope project` 恢复。
[维护与恢复](../use/manage)说明恢复和转移细节。

## 构建插件 {#build-plugins}

<a id="recommended-default"></a>
<a id="if-you-only-read-one-thing"></a>

标准包从根目录 `plugin.json` 开始，可包含 `skills/<name>/SKILL.md`、`mcp.json` 或两者。
可以手动编写文件，再使用已发布安装器验证：

```bash
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin --target codex --dry-run
```

这些例子假设 `./my-plugin` 已经是完整包。[构建指南](../build/)提供手动技能包示例和源码骨架工具的步骤。

**创作工具可用性：** 当前 npm 和原生安装器不包含 `agentplugins author`。
当前源码实现了骨架、静态检查、显式 MCP 运行检查、开发周期、依赖 bootstrap 和 JSON 维护。
这些命令需先[从源码构建](../build/#build-from-source)。旧版创作发布证据仍只对应其原始版本，不能说明当前下载可用功能。

静态验证不证明运行、OAuth 或激活。MCP 传输支持也因客户端而异；例如 Codex 不支持声明为 SSE 的 MCP。
请在 README 中保留前提条件和待完成检查。

[Agent Plugins 1.0 规范](https://agent-plugins.org/specification)

<!-- locale-historical-source:start
locale-historical-frontmatter:start
---
title: "快速入门"
description: "通往工作 plugin-kit-ai 项目的最快推荐路径。"
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---
locale-historical-frontmatter:end
# 快速入门

当您想要一个插件存储库稍后可以发展为更多方式来运送插件时，这是最短的推荐路径。

首先从一条强有力的道路开始。稍后当产品实际需要时添加包、扩展或存储库拥有的集成设置。

## 如果你只读一件事

从默认的 Go 路径开始，除非您已经知道 Claude 挂钩、Node/TypeScript 或 Python 定义了产品要求。

您的第一个选择是起点，而不是存储库的永久边界。

## 推荐默认值

如果您没有充分的理由选择其他路径，请从这里开始：

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
plugin-kit-ai init my-plugin
cd my-plugin
go mod tidy
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

这为您提供了当今最强大的默认路径：基于 Go 的 Codex 运行时存储库，可以轻松验证、移交和稍后扩展。

## 为什么这是默认值

- 从第一天开始就一个仓库
- 今天最干净的运行时和发布故事
- 为以后的封装、扩展和集成通道提供最简单的基础

## 你得到什么

- 从第一天起就有一个插件仓库
- 新仓库在 `plugin/` 下创作文件
- 从同一存储库生成 Codex 运行时输出
- 通过 `validate --strict` 进行干净的准备检查

## 支持 Node 和 Python 路径

如果您的团队已位于 Node/TypeScript 或 Python 中，则这些路径从一开始就受支持且可见：

- `codex-runtime --runtime node --typescript`
- `codex-runtime --runtime python`
- 两者都是本地解释运行时路径，因此目标机器仍然需要 Node.js `20+` 或 Python `3.10+`
- 当您想要最强的一般制作故事时，Go 仍然保持默认值

## 如果您有意从 Node 或 Python 开始

仅当语言选择已是产品要求的一部分时才使用此替代流程：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

或者以 Python 开头：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## 接下来做什么

- 编辑 `plugin/` 下的插件
- 更改后再次运行 `plugin-kit-ai generate ./my-plugin`
- 再次运行 `plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict`
- 只有在产品需要时才添加另一种运输方式

## 稍后展开

|如果你想要|稍后添加 |
| --- | --- |
| Claude 与真实产品挂钩 | `claude` |
|官方Codex包| `codex-package` |
| Gemini 扩展包 | `gemini` |
|回购拥有的集成设置 | `opencode` 或 `cursor` |

仅当 Claude 挂钩已经是实际产品需求时，才首先选择 `claude`。

## 稍后扩展的内容

- 当您添加更多通道时，存储库保持统一
- 包和扩展通道来自同一来源
- 当存储库应该拥有集成设置时，OpenCode 和 Cursor 适合
- 确切的支持边界保留在参考文档中，而不是在您的首次启动流程中

## 快速入门后

- 如果您想要最窄的推荐教程，请继续[构建您的第一个插件](/zh/guide/first-plugin)。
- 如果您想要完整的产品地图，请继续[您可以构建什么](/zh/guide/what-you-can-build)。
- 当您准备好将存储库与您想要的运输方式相匹配时，继续[选择目标](/zh/guide/choose-a-target)。
- 当您准备好扩展第一条路径之外时，继续[一个项目，多个目标](/zh/guide/one-project-multiple-targets)。
locale-historical-source:end -->
