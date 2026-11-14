---
title: "एजेंट होस्ट पहचान"
description: "Hook डेटा से Claude और Codex की पहचान करें।"
canonicalId: "page:sdk:host-detection"
section: "sdk"
locale: "hi"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Hook चलाने वाले एजेंट की पहचान

`sdk/hostdetect` **इस hook process को बुलाने वाले एजेंट** की पहचान करता है, मशीन पर इंस्टॉल सभी एजेंटों की सूची नहीं बनाता।

## डिफ़ॉल्ट संकेत

`DefaultRegistry()` सिर्फ **Codex और Claude Code** पहचानता है। JSON के सबसे ऊपर `turn_id` होने पर `PlatformCodex`, और `turn_id` के बिना `hook_event_name` होने पर `PlatformClaude` मिलता है। गलत, बहुत बड़े (`MaxPayloadBytes` से अधिक) या अनजान payload पर `PlatformUnknown` मिलता है। Codex की प्राथमिकता पहले है।

`PLUGIN_ROOT` और `CLAUDE_PLUGIN_ROOT` पर्यावरण संकेतक भरोसेमंद नहीं हैं: Codex दोनों सेट कर सकता है। इसलिए डिफ़ॉल्ट registry उनका उपयोग नहीं करती। पुराने Codex notify के argv JSON में `turn-id` भी यह stdin JSON detector नहीं पहचानता।

## उदाहरण

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

`Detect(registry, override, env, payload)` पहले सही explicit override, फिर कस्टम registry के क्रमबद्ध env संकेत और अंत में JSON top-level keys देखता है। **अज्ञात override पर error** आता है; कोई पहचान न मिले तो `PlatformUnknown` और nil error। `Registry` / `Signal` बढ़ाते समय केवल सत्यापित, अलग पहचान देने वाले संकेत जोड़ें। प्रकाशित Go tag में पैकेज की उपस्थिति भी जाँचें।

**इंस्टॉल clients और निजी profiles** खोजने के लिए [TypeScript `discoverTargets`](./typescript-management) से `manage discover` चलाएँ, या [Go `Engine.Discover`](./go-installer) उपयोग करें। ये खुद इंस्टॉल नहीं करते।

[स्रोत और टेस्ट](https://github.com/777genius/universal-agent-plugins/tree/main/sdk/hostdetect)।
