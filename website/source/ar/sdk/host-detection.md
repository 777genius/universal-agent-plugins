---
title: "اكتشاف العميل المستدعي"
description: "تمييز Claude Code وCodex من بيانات الخطاف."
canonicalId: "page:sdk:host-detection"
section: "sdk"
locale: "ar"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# اكتشاف الوكيل الذي استدعى الخطاف

تحدد `sdk/hostdetect` **المنتج الذي شغّل عملية الخطاف الحالية**، ولا تبحث عن جميع التطبيقات المثبتة على الجهاز.

## الإشارات الافتراضية

لا يدعم `DefaultRegistry()` سوى **Codex وClaude Code**. وجود `turn_id` في أعلى JSON يعني `PlatformCodex`، ووجود `hook_event_name` دون `turn_id` يعني `PlatformClaude`. البيانات غير الصحيحة أو الأكبر من `MaxPayloadBytes` أو التي لا تطابق أي إشارة تعيد `PlatformUnknown`. أولوية Codex أولًا.

لا يعتمد السجل الافتراضي على متغيري `PLUGIN_ROOT` أو `CLAUDE_PLUGIN_ROOT` لأن Codex قد يضبطهما معًا. كما أن بيانات Codex notify القديمة في argv بمفتاح `turn-id` لا يتعرف عليها كاشف stdin JSON هذا.

## مثال

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

تفحص `Detect(registry, override, env, payload)` بالتتابع: تجاوزًا صريحًا صالحًا، ثم إشارات البيئة في السجل المخصص المرتب، ثم مفاتيح JSON العليا. **التجاوز غير المعروف خطأ**، وغياب التطابق يعيد `PlatformUnknown` دون خطأ. أضف `Registry` / `Signal` فقط عند وجود إشارة مميزة ومتحقق منها. تحقق من وجود الحزمة في وسم Go المنشور الذي تستخدمه.

للعثور على **العملاء المثبتين وملفاتهم الخاصة** استخدم [`discoverTargets`](./typescript-management) الذي يشغّل `manage discover` أو [`Engine.Discover`](./go-installer) للمحولات المسجلة. لا ينفذ أي منهما التثبيت.

[المصدر والاختبارات](https://github.com/777genius/universal-agent-plugins/tree/main/sdk/hostdetect).
