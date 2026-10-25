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

```sh
agentplugins add owner/repo --alias review --target cursor \
  --scope project --project-root /path/to/project
agentplugins skills add owner/repository --ui \
  --scope project --project-root /path/to/project
```

目录必须已存在。默认范围仍为 `user`；使用 `project` 而未指定 `--project-root`
时，会选择当前执行目录，不搜索父目录。浏览器和终端使用相同的范围，只展示该范围
支持的目标。文件准备完成后，客户端可能仍需重启、信任工作区或显式选择会话配置。

[项目安装指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [当前项目支持情况及限制](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
