---
title: "Go Hooks SDK"
description: "Go में टाइप किए गए मूल एजेंट hook handlers बनाएँ।"
canonicalId: "page:sdk:go-hooks"
section: "sdk"
locale: "hi"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Hooks SDK

Go SDK **टाइप किए हुए native एजेंट hook handlers** रजिस्टर करता है। यह प्लगइन इंस्टॉल नहीं करता और hooks को अपने आप पोर्टेबल Skills/MCP घटक नहीं बनाता।

## प्रकाशित मॉड्यूल इंस्टॉल करें

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.3.0
```

मॉड्यूल Go 1.22 और toolchain Go 1.26.10 घोषित करता है। नवीनतम `main` पर दिखने वाली सुविधाएँ ज़रूरी नहीं कि `v1.3.0` में हों।

## उदाहरण

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

`New(Config)` ऐप बनाता है; `Claude()`, `Codex()`, `Gemini()` और `Qoder()` handlers रजिस्टर करने के लिए हैं; `Use` middleware जोड़ता है; `Run()` या `RunContext(ctx)` एक invocation संभालता है। एजेंट की **native hook configuration** में executable, event name और payload अलग से सेट करना होगा। केवल यह उदाहरण सफल native इंस्टॉलेशन का प्रमाण नहीं है।

## Event compatibility और प्रक्रिया

[SDK README](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md) में Claude `Stop` / `PreToolUse` / `UserPromptSubmit`, Codex `Notify`, Gemini के नौ hooks और Qoder hooks दिए गए हैं; अन्य event `public-beta` हो सकते हैं। Cursor का stop observer अभी **beta** है, पूरी native qualification नहीं।

`Supported()` **runtime events** की स्थिति बताता है, इंस्टॉलर की नहीं। `MaxPayloadBytes` इनपुट सीमित करता है। stdout को SDK wire response के लिए रखें; diagnostics stderr में लिखें। Shared executable के लिए [Host detection](./host-detection) देखें, पर सही `Config.IO` replay के बिना stdin दो बार न पढ़ें।

[स्थिरता नीति](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/STABILITY.md)।
