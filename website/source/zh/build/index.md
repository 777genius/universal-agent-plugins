---
title: "构建插件"
description: "用标准清单、技能和 MCP 配置构建可移植插件包。"
canonicalId: "page:build:index"
section: "build"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 构建插件

Agent Plugins 1.0 包是包含根目录 `plugin.json` 及技能、MCP 配置或两者的文件夹。
先选择足以解决用户任务的最小组件：技能提供操作说明，MCP 提供工具连接。

## 可用性

当前安装器 [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94)可以验证并安装标准包，
但**不包含 `agentplugins author`**。本节中的创作命令要求**启用创作功能的当前源码构建**。
历史 0.1.65 创作版本的证据仍只对应那个版本，不能说明当前安装器包含哪些功能。

您可以手动编写文件并使用已发布安装器，也可以构建下方源码工具以生成骨架、执行静态检查和显式 MCP 运行检查。
静态检查通过不证明客户端已激活或服务连接正常。

## 手动编写一个小包

创建目录：

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

在 `plugin.json` 中记录包身份：

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "review-helper",
  "version": "0.1.0",
  "description": "审查文档变更的清晰度和证据"
}
```

在 `skills/review-docs/SKILL.md` 中写入：

```markdown
---
name: review-docs
description: 审查文档的清晰度和证据时使用。
---

# 审查文档

明确读者和任务。检查每条命令的输入与效果。
区分已验证行为、前提条件和未经测试的断言。
报告问题的位置，并建议修改文字。
```

安装已发布的管理器，再验证和预览：

```bash
npm install -g universal-agent-plugins
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

检查计划后按[使用插件](../use/install)应用安装。[目录结构](./layout)说明各文件的职责。

## 从源码构建 {#build-from-source}

以下命令从当前源码构建启用了创作功能的 CLI。需要 Git 和仓库声明的 Go 工具链（当前为 Go 1.26.10）。
共享结果时请记录检出的提交。

```bash
git clone https://github.com/777genius/universal-agent-plugins.git
cd universal-agent-plugins/cli
mkdir -p ../bin
authoring_module=$(go list -f '{{.Module.Path}}' ./cmd/agentplugins)
authoring_revision=$(git rev-parse HEAD)
go build -ldflags "-X ${authoring_module}/internal/authoring/commands.Enabled=release-cli-contract-v1 -X ${authoring_module}/internal/authoring/commands.Revision=${authoring_revision}" \
  -o ../bin/agentplugins ./cmd/agentplugins
export PATH="$(cd ../bin && pwd):$PATH"
agentplugins author --help
```

在另一个工作目录创建教程包。这个可执行文件是开发构建，其存在本身不代表公共创作版本已通过发布验收。
如果仍报告未知 `author` 命令，请检查正在使用的可执行文件路径。

## 选择模板

| 需求 | 指南 | 必需参数 |
| --- | --- | --- |
| 可重复执行的操作说明 | [独立技能](./skill) | `--template skill` |
| 连接现有服务 | [远程 MCP](./mcp-remote) | `--template mcp-remote --url <endpoint>` |
| 本地 Node MCP 进程 | [Stdio MCP](./mcp-stdio) | `--template mcp-stdio --runtime node` |
| 将操作说明与工具结合 | [混合包](./hybrid) | `--template hybrid --mcp-template <choice>`，加上该 MCP 模板参数 |

独立技能包也包含根目录 `plugin.json`。从其他仓库安装独立技能属于另一条[安装器流程](../use/install#install-agent-skills)。

## 按顺序创作与检查

1. 在不存在的目标目录创建包，再编辑生成的文件。
2. 对同一个包根目录运行 [validate、inspect 和静态 test](./checks)。
3. 使用 compat 检查目标客户端，并查看 doctor 证据。
4. MCP 运行行为另行通过源码构建显式执行。
5. 将包和证据[交付给安装器](./handoff)。

当前源码还提供基于锁文件的 bootstrap、持续 dev、JSON normalize 和显式 Claude MCP `import native`。
请参阅[检查](./checks)与 [stdio MCP](./mcp-stdio)。当前创作命令不提供客户端投影、包导出或发布。
客户端专用钩子与可移植技能和 MCP 组件仍是不同的能力。
