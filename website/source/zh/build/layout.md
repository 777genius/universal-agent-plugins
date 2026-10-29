---
title: "理解包根目录"
description: "在明确的标准包根目录中保存身份和组件。"
canonicalId: "page:build:layout"
section: "build"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 理解包根目录

**源码创作：** 以下命令要求[启用创作功能的源码构建](./#build-from-source)。当前安装器版本不包含 `agentplugins author`。

所选目录就是包根目录。创作工具读取根目录的 `plugin.json` 和标准组件，
不会向上查找项目，也不会退回读取客户端隐藏清单。

## 最小布局与组合布局

仅技能包：

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

远程混合包：

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

stdio 包还包含运行时源码和依赖清单。文件是后续执行的输入，其存在不会触发创作读取器执行代码。

## 各文件的职责

| 文件或目录 | 职责 |
| --- | --- |
| `plugin.json` | 标准包身份和元数据。 |
| `mcp.json` | 提供工具时的 MCP 服务器配置。 |
| `skills/<name>/SKILL.md` | 一项直接子技能的身份、描述和说明。 |
| `README.md` | 用途、前提条件、验证和剩余限制。 |
| `package.json`、`package-lock.json`、`src/server.mjs` | Node stdio 骨架及独立运行工作的输入。 |
| `LICENSE` | init 时明确选择的许可证。 |

骨架将包版本初始化为 `0.1.0`，这是新包的版本，不是创作可执行文件的版本。向用户报告时应区分两者。

## 身份与可选元数据

可重复的 init 说明应明确提供 `--name` 和 `--description`。
命令可从裸目录名推导名称，并提供确定的默认描述，但 `./review-helper` 这类路径应带显式名称。
这些教程同时提供两者，避免意外身份。

含技能的模板在省略 `--skill-name` 时默认使用包名；需要不同名称时应明确指定。技能名不会被静默规范化。

可选 `--author-name` 指定作者，不从 Git 或环境推断。
生成许可证时可明确选择 MIT 或 ISC，并同时提供 `--copyright-holder` 和四位数 `--copyright-year`。
没有默认许可证。

## 始终选择相同根目录

```bash
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper
agentplugins author test ./research-helper
```

三条命令选择同一个包根目录。`./research-helper/skills` 是错误根目录。
读取命令的可选路径只默认当前目录，不寻找父目录。

标准符合性和文件系统安全是不同问题。符合 schema 的身份仍可能不适合主机路径；
只有可读取清单也可能缺少组件证据。请查看完整报告，再继续[检查](./checks)和[交付](./handoff)。
