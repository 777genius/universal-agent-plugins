---
title: "وثائق Agent Plugins"
description: "الوثائق العامة لـ Agent Plugins."
canonicalId: "page:home"
section: "home"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">الوثائق العامة</p>
  <h1>Agent Plugins</h1>
  <p class="docs-lead">ثبّت Skills وأدوات MCP للوكلاء الذين تستخدمهم، أو أنشئ حزمتك الخاصة القابلة للنقل.</p>
</div>

## اختر مسارك {#choose-your-journey}

<div class="docs-grid">
  <a class="docs-card" href="./use/">
    <h2>استخدام الإضافات</h2>
    <p>افحص الحزمة، وعاين تغييراتها، ثم ثبّتها لحسابك أو لمشروع واحد.</p>
  </a>
  <a class="docs-card" href="./build/">
    <h2>بناء الإضافات</h2>
    <p>اكتب حزمة معيارية تضم plugin.json وSkills وإعدادات MCP. تعرّف على أدوات التأليف المبنية من المصدر.</p>
  </a>
</div>

## ثبّت إضافتك الأولى {#install-your-first-plugin}

باستخدام Node.js 22 أو أحدث:

```bash
npx universal-agent-plugins add context7
```

تسألك واجهة CLI عن الوكلاء المتوافقين الذين تريد استخدامهم. راجع الخطة، ثم أكمل خطوات التفعيل أو تسجيل الدخول التي تعرضها. إذا كنت تفضّل ملفًا تنفيذيًا أصليًا، فاتبع [دليل التثبيت الأصلي](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) لنظام macOS أو Linux أو Windows؛ فالمدير الأصلي نفسه لا يحتاج إلى Node.js.

## ما المتاح حاليًا {#what-is-available}

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) هو الإصدار الحالي من المثبّت الأصلي، وقد نُشر في 29 أكتوبر 2026. يضيف واجهة API للإدارة المحلية وامتدادات Pi الأصلية ضمن نطاق المستخدم. راجع [الإدارة المحلية](./use/manage#local-management-api) و[توافق Pi](./reference/client-compatibility#pi-native-extensions).

يحتفظ بتحديد نطاق المشروع صراحةً والاستعادة وفق ملف القفل. تعمل معظم الأوامر افتراضيًا على بيئة المستخدم. منذ 0.1.94، يختار `install` دون وسيطة مصدر المشروع الحالي فقط إذا احتوى المجلد الحالي نفسه على ملف عادي باسم `uap.toml`؛ وتكون الأولوية للنطاق الصريح. حدّد `--scope project` في أوامر الإضافة والصيانة للمشروع. راجع [التثبيت](./use/install) و[الصيانة](./use/manage).

قد تختلف الإصدارات بين قنوات التوزيع. تحقّق من `agentplugins version`؛ واستخدم [المثبّت الأصلي](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) للحصول على الإصدار 0.1.95 المتحقق منه. يتطلب الاختيار التلقائي للمشروع 0.1.94 أو أحدث، ويتوفر نطاق المشروع الصريح منذ 0.1.93.

لا يتضمن المثبّت الحالي `agentplugins author`. يشرح [دليل البناء](./build/) إنشاء الحزم يدويًا وأوامر التأليف المنفذة في المصدر الحالي. تبقى أدلة إصدارات التأليف الأقدم مرتبطة بإصداراتها الأصلية، ولا تصف تنزيل المثبّت الحالي.

## افهم النتيجة {#understand-the-result}

التحقق من الحزمة، والتثبيت، واكتشاف العميل للمكونات، وتنفيذ أدوات MCP، والمصادقة مع الخدمة فحوص منفصلة. قد ينجح تثبيت الملفات وتظل هناك حاجة إلى منح الثقة داخل العميل، أو إعادة تحميله، أو بدء جلسة جديدة. تغطي الاختبارات الأصلية إصدارات ومنصات محددة للعملاء؛ ولا تمنح اعتمادًا لكل إضافة أو لكل وكيل.

- [البدء السريع](./guide/quickstart)
- [بنية الحزمة](./build/layout)
- [الفحوص والأدلة](./build/checks)
- [توافق العملاء وحدود الاختبارات](./reference/client-compatibility)
- [مواصفة Agent Plugins 1.0](https://agent-plugins.org/specification)
