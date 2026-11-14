---
title: "Go Installer SDK"
description: "دمج التثبيت مع مراجعة الخطة والاسترداد."
canonicalId: "page:sdk:go-installer"
section: "sdk"
locale: "ar"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Installer SDK

تسمح واجهة Go هذه بدمج **التثبيت والتحديث والإصلاح وإعادة إنشاء الإسقاط والإزالة والفحص والاسترداد** في تطبيقك. وهي مختلفة عن Go Hooks SDK.

## الوحدة وحدود النشر

```text
github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer
```

تتبع الواجهة وحدة `github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins`. ثبّت مراجعة المصدر والاعتماديات بعد فحصها؛ **لا تفترض** أنها تستعمل إصدار hooks `v1.3.0`.

## إنشاء المحرك واكتشاف العملاء

```go
registry, err := clients.NewRegistry(codex.New(), claude.New())
if err != nil { return err }

engine, err := uapinstaller.New(uapinstaller.Config{
    StateRoot: "/absolute/private/uap-state",
    Registry: registry,
})
if err != nil { return err }

available := engine.Discover()
_ = available // inspect scopes, capabilities and detected executables
```

يجب أن يكون `StateRoot` مسارًا مطلقًا وأن يحدد `Registry` المحولات صراحةً. تتحقق `New` من التهيئة فقط؛ **لا تنشئ مجلدات ولا تشغل عملاء**. تعرض `Discover` قدرات المحولات المسجلة ونطاقاتها ووجود ملفات التنفيذ دون إثبات تفعيلها. اضبط helper وrunner وملف العميل وسياسة الثقة وفق حزمة التثبيت.

## التحضير ثم المراجعة ثم التنفيذ

```go
req := uapinstaller.Request{
    Operation:        uapinstaller.OpInstall,
    PackageRoot:      "/absolute/reviewed/package",
    ClientID:         "codex",
    ClientConfigRoot: "/absolute/codex-profile",
    ClientExecutable: "/absolute/path/to/codex",
    InstallationID:   "00000000-0000-4000-8000-000000000099",
    OperationID:      "host-generated-unique-operation-id",
}
prepared, err := engine.Prepare(ctx, req)
if err != nil { return err }
defer prepared.Close()

plan := prepared.Plan()
// Show plan, targets, components, warnings and digests to the user.
// Only after explicit approval, with immutable prepared inputs:
result, err := engine.Apply(ctx, prepared, uapinstaller.Decision{Confirmed: true})
if err != nil { return err }
_ = result

view, err := engine.Inspect(ctx)
if err != nil { return err }
if view.Recovery.Required {
    _, err = engine.Recover(ctx, view)
    if err != nil { return err }
}
```

هذا **مقتطف** يحتاج إلى `ctx` و`engine` والاستيرادات من [المثال الكامل](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go). اعرض **الخطة الثابتة الفعلية** للمستخدم قبل `Decision{Confirmed:true}`. تعيد `Apply` التحقق من الصلاحيات والبصمات. `Inspect` للقراءة فقط؛ وتتطلب `Recover` الملاحظة ذات الأنواع المحددة نفسها.

لا تستنتج `ClientConfigRoot` من `HOME` أو `CODEX_HOME`. تحتاج الملفات المحلية إلى `Assess` أو `Request.Assessment` مرتبطة بالبصمة؛ ولا تستخدم `TrustedLocalPackages` إلا مع محتوى محلي موثوق مسبقًا بشكل صريح. تدعم `Request.Targets` المجموعات بحدود خاصة بـOpenCode والإزالة. تُراجع حالة التخزين القديمة عبر `PrepareStateMigration` و`ApplyStateMigration` مع نسخة احتياطية.

[العقد التفصيلي](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md).
