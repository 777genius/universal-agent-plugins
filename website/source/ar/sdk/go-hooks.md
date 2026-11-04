---
title: "Go Hooks SDK"
description: "معالجات أحداث أصلية ذات أنواع محددة بلغة Go."
canonicalId: "page:sdk:go-hooks"
section: "sdk"
locale: "ar"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Hooks SDK

تتيح حزمة Go Hooks تسجيل **معالجات أصلية ذات أنواع محددة للأحداث**. وهي ليست مثبّتًا للإضافات، ولا تحول الخطافات تلقائيًا إلى مكونات Skills/MCP قابلة للنقل.

## تثبيت الوحدة المنشورة

```bash
go get github.com/777genius/plugin-kit-ai/sdk@v1.3.0
```

تعلن الوحدة عن Go 1.22 وتوجيه toolchain إلى Go 1.26.10. قد تتضمن شجرة `main` أحداثًا غير متاحة في الإصدار `v1.3.0`.

## مثال

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

تنشئ `New(Config)` التطبيق، وتوفر `Claude()` و`Codex()` و`Gemini()` و`Qoder()` تسجيل المعالجات. تضيف `Use` وسيطًا، وتعالج `Run()` أو `RunContext(ctx)` استدعاءً واحدًا. يجب إعداد ملف التنفيذ واسم الحدث وبيانات الإدخال **في إعدادات الخطافات الأصلية للعميل**؛ ولا يثبت هذا المثال نجاح التثبيت.

## التوافق والتنفيذ

يوثق [README الخاص بالـSDK](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md) أحداث Claude `Stop` / `PreToolUse` / `UserPromptSubmit`، وCodex `Notify`، وتسعة hooks في Gemini، وأحداث Qoder. أحداث أخرى قد تكون `public-beta`. مراقب التوقف في Cursor ما زال **beta** ولا يثبت اكتمال التكامل الأصلي.

تعرض `Supported()` حالة **أحداث وقت التشغيل** فقط، وليس توافق المثبّت. يحد `MaxPayloadBytes` حجم بيانات الإدخال. اترك stdout لرد البروتوكول، واكتب التشخيص إلى stderr. للبرنامج المشترك بين العملاء استخدم [اكتشاف المضيف](./host-detection) دون قراءة stdin مرتين إلا عند توفير `Config.IO` قابل لإعادة تشغيل المدخلات.

[سياسة الاستقرار](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/STABILITY.md).
