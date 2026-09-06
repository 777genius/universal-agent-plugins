---
title: "安装"
description: "使用支持的通道安装 plugin-kit-ai。"
canonicalId: "page:guide:installation"
section: "guide"
locale: "zh"
generated: false
translationRequired: true
---

[使用插件](/zh/use/) · [构建插件](/zh/build/) — 仅为准备内容，尚未发布。v2 暂不支持项目迁移。
<details><summary>plugin-kit-ai v1 · 1.2.4 历史资料；旧说明不是当前的 Build 流程。</summary>

# 安装

当适合您的环境时，默认使用 Homebrew。这里的目标很简单：安装 CLI 并快速到达您的第一个工作存储库。

## 支持的频道

- Homebrew 表示最干净的默认 CLI 路径。
- npm 当您的环境已经以 npm 为中心时。
- PyPI / pipx 当您的环境已经以 Python 为中心时。
- 验证安装脚本作为后备路径。

## 推荐命令

### Homebrew

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

### npm

```bash
npm i -g plugin-kit-ai
plugin-kit-ai version
```

### PyPI / pipx

```bash
pipx install plugin-kit-ai
plugin-kit-ai version
```

### 已验证的脚本

```bash
curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh
plugin-kit-ai version
```

## 大多数人应该使用哪一个？

- 如果您使用的是 macOS 并且想要最流畅的默认路径，请使用 Homebrew。
- 仅当 npm 或 pipx 已经与您的团队环境匹配时才使用。
- 当您需要在包管理器优先设置之外进行后备时，请使用经过验证的脚本。

## 安装后

大多数人应该直接继续 [快速入门](/zh/guide/quickstart) 并在默认的 Go 路径上创建第一个存储库。

如果您选择 `pipx` 是因为您的团队首先是 Python 并且您已经知道您需要 Python 路径，请继续[构建 Python 运行时插件](/zh/guide/python-runtime)。

## CI 安装路径

对于 CI，更喜欢专用的设置操作，而不是教每个工作流程如何手动下载 CLI。

## 重要边界

npm 和 PyPI 软件包是 CLI 的安装通道。它们不是运行时 APIs，也不是 SDKs。

有关合约边界，请参阅[参考 > 安装通道](/zh/reference/install-channels)。
</details>
