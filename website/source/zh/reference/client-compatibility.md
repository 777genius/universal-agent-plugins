---
title: "UAP 客户端兼容性证据"
description: "详细的兼容性证据以英文维护。"
canonicalId: "page:reference:client-compatibility"
section: "reference"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "english-fallback"
---

# UAP 客户端兼容性证据

详细证据、已测试版本及其限制均在规范英文页面中维护。

[查看英文证据](/en/reference/client-compatibility)

## 当前源码中的项目范围

**仅适用于源码构建，尚未发布。** 上述历史版本和客户端观察结果保持原有范围。
项目级安装为每个客户端提供单独的支持约定：文件所有权、客户端发现以及 MCP／模型
运行分别验证。ChatGPT 会明确拒绝项目范围；其他目标各有组件、准备步骤和受支持
客户端的限制。

VS Code 项目准备使用 `.github/skills` 和 `.vscode/mcp.json`。
[固定版本 VS Code 1.143.1 的观察记录](https://github.com/777genius/universal-agent-plugins/actions/runs/40188310034)验证了独立 A/B 文件夹、
A 的重新启动及移除后的元数据，且未授予工作区信任。这仅证明元数据行为；
适配器返回 Prepared 并验证已安装文件，不据此声称 MCP 已启动、工具已调用
或模型已执行。

[项目安装指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [当前项目支持情况及限制](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
