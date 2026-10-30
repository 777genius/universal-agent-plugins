---
title: "查找并安装插件"
description: "检查插件来源、预览目标变更，并完成客户端激活。"
canonicalId: "page:use:install"
section: "use"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 查找并安装插件

先了解包的来源和用途。以下命令使用已发布的安装器；尚未安装时请参阅 [CLI 安装](./)。
使用 `--dry-run` 在应用变更前检查计划。

## 发现并检查包

```bash
agentplugins search docs
agentplugins info context7
```

`context7` 等短名称来自带签名的 Universal Agent Plugins Registry。
搜索和包信息用于筛选候选项，不会执行插件，也不能证明其服务可用。安装前请检查工具、技能、运行时和账户访问要求。

## 预览并应用明确的选择

```bash
agentplugins add context7 --target codex,cursor --dry-run
```

检查来源、目标、路径、安全检查结果和待完成的激活工作。这条命令只输出计划，不应用安装，也不测试 MCP 服务器。
检查完成后去掉 `--dry-run`：

```bash
agentplugins add context7 --target codex,cursor
```

交互使用时可省略 `--target`。只有一个检测到的客户端时可自动选择；多个客户端时会显示选择器。
脚本和自动化中应明确指定目标，以便重现选择。安装器预先检查目标，并保护不属于它的文件。
若报告回滚失败或待恢复操作，请保留报告并先按恢复说明处理。

## 完成激活

按结果中的客户端说明重新加载、新建会话、授予工作区信任、登录或激活。
准备好文件或完成原生注册，不等于 OAuth 已完成或工具调用已成功。
文件缺失或漂移时使用[状态诊断](./manage)；包已存在但不可用时还应检查客户端认证。

## 安装本地包

选择包含根目录 `plugin.json` 的文件夹：

```bash
agentplugins validate ./my-plugin
agentplugins add ./my-plugin --target codex --dry-run
```

安装器验证和规划会应用安全策略，必要时可能获取扫描器或元数据。即使输入是本地目录，也不保证离线执行。
`--dry-run` 避免应用客户端安装变更，不代表完全不联网或不使用缓存。

检查计划后去掉 `--dry-run`。本地目录仍是本地安装，不能作为另一台机器可抓取的可移植依赖。
请向作者索取[包交付证据](../build/handoff)。

## 选择固定的 GitHub 源

原有 GitHub 语法要求完整的 40 字符提交 SHA。请将示例换成实际仓库、已审核提交和包路径：

```text
agentplugins add owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin --target codex --dry-run
```

`owner/repository@...` 这种写法拒绝分支、标签和缩短的 SHA。省略路径时会选择有效的根包或唯一有效的嵌套候选包。
有多个候选包时必须给出路径；可能的包超过 16 个时，要先提供路径才会抓取候选项。
repair 重放记录的来源；切换到不同固定来源是单独的源选择操作。

CLI 0.1.93 也支持 HTTPS/SSH Git、受支持的提供方浏览器链接，以及完整的 HTTPS 或本地归档。
完整语法、引用和摘要规则见[直接安装完整包](https://github.com/777genius/universal-agent-plugins/blob/main/docs/DIRECT_PACKAGE_SOURCES.md)。

## 安装 Agent Skills {#install-agent-skills}

独立技能使用 `skills` 命令组，来源可以是公共 GitHub 仓库、文件夹或 `SKILL.md` URL，
也可以是提供完整 `.well-known` 技能索引的 HTTPS 发布方：

```bash
agentplugins skills add owner/repository --ui
```

请换成实际来源。`--ui` 在交互终端中打开内置浏览器选择器；`--no-ui` 使用终端选择。
明确选择时可运行：

```bash
agentplugins skills add owner/repository --skill review --skill pdf \
  --target codex,claude --no-ui --yes
```

- 重复 `--skill` 指定精确名称；名称有歧义时用 `--path` 指定仓库内技能目录。
- URL 的分支与路径边界有歧义时用 `--ref` 明确 Git 引用。
- `--dry-run` 只预览安装。
- JSON 或非交互执行需要明确技能和目标；除预览外，还需要 `--yes`。
- `--ui` 不能与 `--skill`、`--target` 或 `--dry-run` 组合。
- HTTPS 发布方安装需要精确的 `--skill` 名称，且索引必须列出该技能的所有文件。

自动安全检查可能阻止安装。用 `--security-details` 查看详细结果，再决定是否明确接受 `--accept-security-risk`。
已安装技能会出现在 `agentplugins list` 中，并使用普通维护命令。

## 为单个项目安装

[CLI 0.1.93 及以后版本](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93)支持显式项目范围。
选择已存在的目录，并在每条命令中指定范围：

```bash
agentplugins add context7 --target cursor --scope project \
  --project-root /path/to/project --dry-run
agentplugins skills add owner/repository --ui --scope project \
  --project-root /path/to/project
```

第一条只预览，准备应用时去掉 `--dry-run`。第二条打开绑定到该项目的选择器，可应用用户确认的选择。
指定项目范围但省略 `--project-root` 时，仅使用调用目录，不向上查找。add 和技能安装默认使用用户范围。在 CLI 0.1.94 中，未指定来源参数的 `install` 可根据当前目录的清单自动选择项目范围，详见[维护与恢复](./manage)。

目标必须支持所请求的项目组件。ChatGPT 明确不支持原生项目安装。
VS Code 为单个本地文件夹准备 `.github/skills` 和 `.vscode/mcp.json` 中属于 UAP 的 `servers` 成员；
结果保持 `Prepared`，需要在客户端检查工作区和服务器信任。元数据观察不证明 MCP 启动、工具调用或模型执行。

查看[项目安装指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md)和
[各客户端项目边界](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)，然后继续[维护与恢复](./manage)。
