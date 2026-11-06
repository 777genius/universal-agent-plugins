---
title: "Agent 主机识别"
description: "从 hook 输入数据识别 Claude Code 或 Codex。"
canonicalId: "page:sdk:host-detection"
section: "sdk"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 识别调用 hook 的 Agent

`sdk/hostdetect` 用于识别**调用当前 hook 进程的产品**，而不是查询设备上安装了哪些 Agent。

## 内置识别规则

`DefaultRegistry()` **仅支持 Codex 和 Claude Code**：JSON 顶层含 `turn_id` 时识别为 `PlatformCodex`；存在 `hook_event_name` 但没有 `turn_id` 时识别为 `PlatformClaude`。无匹配、JSON 格式错误或超过 `MaxPayloadBytes` 时返回 `PlatformUnknown`。优先检查 Codex。

默认规则**不采用** `PLUGIN_ROOT` / `CLAUDE_PLUGIN_ROOT` 环境变量，因为 Codex 可能同时设置这两个值。旧版 Codex notify 的 argv JSON 键 `turn-id` 也不是此 stdin JSON 检测器的输入格式。

## 示例

```go
package main

import (
    "fmt"

    "github.com/777genius/plugin-kit-ai/sdk/hostdetect"
)

func main() {
    payload := []byte(`{"hook_event_name":"Stop","session_id":"example"}`)
    host, err := hostdetect.Detect(hostdetect.DefaultRegistry(), "", nil, payload)
    if err != nil {
        panic(err)
    }
    switch host {
    case hostdetect.PlatformClaude:
        fmt.Println("Claude hook")
    case hostdetect.PlatformCodex:
        fmt.Println("Codex hook")
    default:
        fmt.Println("Unknown; do not assume a default agent")
    }
}
```

`Detect(registry, override, env, payload)` 依次检查：(1) 有效的显式 override；(2) 自定义有序注册表中的环境标志；(3) 受大小限制的 JSON 顶层键。**无效 override 会报错**；未知 payload 返回 `PlatformUnknown` 而不报错。扩展 `Registry` / `Signal` 时必须使用经过验证且能够区分客户端的信号。

该包出现在**当前源码**中；对外发布前请检查所使用的 Go 模块标签是否包含它。若目的是查看**已安装 Agent 与私有配置**，应使用[TypeScript `discoverTargets`](./typescript-management) 或 [Go `Engine.Discover`](./go-installer)。这两个 API 均不自动安装插件。

[源码与测试](https://github.com/777genius/universal-agent-plugins/tree/main/sdk/hostdetect)。
