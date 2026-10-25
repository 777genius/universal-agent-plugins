---
title: "Find and install a plugin"
description: "暂无译文；请阅读当前英文页面"
canonicalId: "page:use:install"
section: "use"
locale: "zh"
generated: false
translationRequired: true
---

# Find and install a plugin

[暂无译文；请阅读当前英文页面](/en/use/install). Agent Plugins 创作已通过 `universal-agent-plugins` 中的 Agent Plugins 发布。

[使用插件](/zh/use/) · [构建插件](/zh/build/)

## 当前源码中的项目级安装

**仅适用于源码构建，尚未发布。** 兼容的源码构建支持为插件包和 Agent Skills
显式选择项目：

[源码构建的命令示例见英文部分](/en/use/install#project-installation-in-the-current-source)。

目录必须已存在。默认范围仍为 `user`；使用 `project` 而未指定 `--project-root`
时，会选择当前执行目录，不搜索父目录。浏览器和终端使用相同的范围；只有受支持
且可用的目标才能被选择。不可用的目标可以继续显示，并说明原因。文件准备完成后，
客户端可能仍需重启、信任工作区或显式选择会话配置。

对于 VS Code，项目作用域会准备 `.github/skills`，并在 `.vscode/mcp.json`
中写入由本次安装拥有的 `servers` 条目。结果仍为 Prepared：请打开所选本地文件夹，
检查工作区和服务器的信任设置，再检查技能与 MCP 配置。此适配器不支持远程工作区
或多根工作区。

[项目安装指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [当前项目支持情况及限制](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
