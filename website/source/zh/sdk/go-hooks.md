---
title: "Go Hooks SDK"
description: "用 Go 为原生 Agent hooks 构建类型安全的处理器。"
canonicalId: "page:sdk:go-hooks"
section: "sdk"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Hooks SDK

Go Hooks SDK 用于注册**类型安全的原生 Agent 事件处理器**，并返回正确的 hook 协议响应。它不是插件安装器，也不会自动把原生 hooks 转换为可移植的 Skills/MCP 组件。

## 安装已发布模块

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.3.0
```

模块声明 Go 1.22 和 Go 1.26.10 toolchain。当前 `main` 分支中较新的事件不一定包含在 `v1.3.0` 中。

## 最小示例

```go
package main

import (
    "os"

    uap "github.com/777genius/plugin-kit-ai/sdk"
    "github.com/777genius/plugin-kit-ai/sdk/claude"
    "github.com/777genius/plugin-kit-ai/sdk/codex"
)

func main() {
    app := uap.New(uap.Config{Name: "review-hooks"})

    app.Claude().OnStop(func(event *claude.StopEvent) *claude.Response {
        return claude.Allow()
    })
    app.Codex().OnNotify(func(event *codex.NotifyEvent) *codex.Response {
        return codex.Continue()
    })

    os.Exit(app.Run())
}
```

`New(Config)` 创建应用；`Claude()`、`Codex()`、`Gemini()` 和 `Qoder()` 提供注册入口；`Use` 添加中间件；`Run()` 或 `RunContext(ctx)` 处理一次调用。还必须在**对应 Agent 的原生 hook 配置**中指定可执行文件、事件名及输入。示例仅说明 SDK API，不证明客户端已完成安装。

## 事件级兼容性

[SDK 说明](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md)列出 Claude `Stop` / `PreToolUse` / `UserPromptSubmit`、Codex `Notify`、Gemini 的九种 hooks，以及当前源码所支持的 Qoder 事件。其他事件可能属于 `public-beta`。Cursor 的 stop observer 仍为 **beta**，不能视为已验证的完整原生集成。

`Supported()` 仅报告**运行时事件**的支持情况，并非安装器兼容性矩阵。`MaxPayloadBytes` 限制输入大小。stdout 应专用于 SDK 的协议输出；诊断应写入 stderr。共享一个 Agent 可执行文件时，请阅读[主机识别](./host-detection)；不要读取 stdin 两次，除非你提供可回放输入的 `Config.IO`。

[稳定性政策](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/STABILITY.md)。
