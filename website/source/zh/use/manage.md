---
title: "Maintain installed plugins"
description: "暂无译文；请阅读当前英文页面"
canonicalId: "page:use:manage"
section: "use"
locale: "zh"
generated: false
translationRequired: true
---

# Maintain installed plugins

[暂无译文；请阅读当前英文页面](/en/use/manage). Agent Plugins 创作已通过 `universal-agent-plugins` 中的 Agent Plugins 发布。

[使用插件](/zh/use/) · [构建插件](/zh/build/)

## 在当前源码中维护项目

**仅适用于源码构建，尚未发布。** 每条维护命令都应选择相同的范围和项目：

[维护命令见英文部分](/en/use/manage#maintain-a-project-in-the-current-source)。

如需共享依赖及其固定版本，可将可移植的 `uap.toml` 和 `uap.lock` 纳入版本控制。
`.uap/` 中的所有权记录、恢复状态和保留数据留在本地；数据应单独备份。
删除安装会保留数据和共享内容缓存。Repair 可恢复缺失的自有组件及损坏的受管包；
被外部修改的客户端文件需要检查。恢复操作必须选择原来的项目。

[项目安装指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [当前项目支持情况及限制](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
