---
title: "构建 Python 运行时插件"
description: "存储库本地 Python 插件的简单端到端路径。"
canonicalId: "page:guide:python-runtime"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<p class="locale-historical-identity">构建 Python 运行时插件</p>

[plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。](/en/legacy/v1/)

<details><summary>plugin-kit-ai v1 分支的历史快照。1.2.4 是命令基准，并非此快照的确切版本。</summary>

# 构建 Python 运行时插件

当您的团队已经编写 Python 并且您希望插件从此存储库运行时，请使用此路径。

如果您想要一个已编译的二进制文件和最简单的分发故事，请选择 Go 。当存储库本身作为插件开发和运行的主要位置时，Python 是受支持的路径。

## 在 10 秒内选择您的 Python 路径

当您想要最简单的第一个存储库时，请使用默认的 Python 路径：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
```

当您想要跨多个存储库从 `requirements.txt` 导入 `plugin_kit_ai_runtime` 时，请使用共享包路径：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

如果您不确定，请先从默认路径开始。

## 这条路径给你带来什么

- 一个插件存储库
- Python `3.10+` 在运行插件的机器上
- 本地 `.venv`
- `codex-runtime` 或 `claude` 支持的 Python 流
- 提交或移交前的一项主要检查：`validate --strict`

## 如果你只想要最短路径

复制此内容并进入第一个绿色运行：

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
cd my-plugin
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai test . --platform codex-runtime --event notify
```

仅在共享依赖项要求成立后才切换到 `--runtime-package` 。

## 1. 安装 CLI

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

## 2. 搭建 Python 项目

对于正常的 Python-第一个 Codex 路径：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python
cd my-plugin
```

如果 Claude 钩子是真正的第一个要求，则使用脚手架 Claude 代替：

```bash
plugin-kit-ai init my-plugin --platform claude --runtime python
cd my-plugin
```

## 3. 准备本地 Python 环境

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
```

`doctor` 告诉您存储库是否已准备好。

`bootstrap` 在需要时创建 `.venv` 并安装 `requirements.txt`。

## 4. 生成并验证

```bash
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

`generate` 更新源文件中生成的启动器和配置文件。

对于 Claude-first 存储库，切换验证目标：

```bash
plugin-kit-ai validate . --platform claude --strict
```

## 5. 添加您的 Python 逻辑

默认脚手架将帮助程序保留在 `plugin/plugin_runtime.py` 中，因此第一个版本保持独立。

典型的 Codex 起始形状：

```python
from plugin_runtime import CodexApp, continue_

app = CodexApp()


@app.on_notify
def on_notify(event):
    _ = event
    return continue_()


if __name__ == "__main__":
    raise SystemExit(app.run())
```

编辑 `plugin/main.py` 作为您的插件逻辑。保留标准输出用于工具响应，并仅将诊断写入标准错误。

## 6. 运行冒烟测试

对于 Codex 运行时路径：

```bash
plugin-kit-ai test . --platform codex-runtime --event notify
```

您还可以直接运行生成的启动器：

```bash
./bin/my-plugin notify '{"client":"codex-tui"}'
```

对于 Claude，最简单的烟雾检查是：

```bash
plugin-kit-ai test . --platform claude --all
```

## 7. 何时使用共享 Python 包

当您想要最简单的第一个存储库时，请保留默认的本地助手。

当您希望跨多个存储库使用相同的帮助程序包时，请使用共享依赖路径：

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime python --runtime-package
```

该路径从已发布的 [`plugin-kit-ai-runtime`](https://github.com/777genius/plugin-kit-ai/tree/main/python/plugin-kit-ai-runtime) 包导入 [`plugin_kit_ai_runtime`](/zh/api/runtime-python/plugin-kit-ai-runtime)，而不是生成 `plugin/plugin_runtime.py`。

如果您使用此源树中的 CLI 的本地开发版本，请在 `init` 期间显式传递 `--runtime-package-version`。
已发布的稳定 CLI 会自动推断匹配的帮助程序版本。

## 简短规则

- 当您想要最干净的包装和分发故事时，选择 Go
- 当团队已经位于 Python 并且插件是存储库本地时，选择 Python
- 使用 `doctor -> bootstrap -> generate -> validate --strict` 作为正常的 Python 流程
- 仅当您确实需要共享依赖项时才切换到 `--runtime-package`

## 后续步骤

- 阅读[选择运行时](/zh/concepts/choosing-runtime) 了解运行时权衡。
- 阅读[选择交付模型](/zh/guide/choose-delivery-model) 了解本地帮助程序与共享包决策。
- 当您需要帮助程序引用时，打开 [Python 运行时 API](/zh/api/runtime-python/)。

</details>
