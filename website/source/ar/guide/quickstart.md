---
title: "البدء السريع"
description: "ثبّت الإضافات المتاحة أو جهّز حزمة Agent Plugins 1.0 قابلة للنقل."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

<a id="quickstart"></a>

# استخدام الإضافات / بناء الإضافات {#use-plugins--build-plugins}

ثبّت حزمة للوكلاء الذين تستخدمهم، أو أنشئ حزمة Agent Plugins 1.0 قابلة للنقل تضم تعليمات وإعدادات أدوات.

## استخدام الإضافات {#use-plugins}

<a id="optional-first-proof"></a>

باستخدام Node.js 22 أو أحدث:

```bash
npx universal-agent-plugins add context7
```

تسألك واجهة CLI عن الوكلاء المتوافقين المطلوبين وتعرض أي خطوات تفعيل متبقية. للفحص والمعاينة أولًا:

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins add context7 --target codex,cursor --dry-run
```

يوفر [دليل التثبيت الأصلي](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) أيضًا ملفات تنفيذية لأنظمة macOS وLinux وWindows. لا يحتاج المدير الأصلي إلى Node.js، لكن الإضافات قد تحتاج إلى بيئات تشغيل خاصة بها. تحقّق من `npx universal-agent-plugins version` عند استخدام npm، فقد توفر قناة مدير الحزم CLI أقدم. يتطلب الاختيار التلقائي أدناه 0.1.94؛ استخدم المثبّت الأصلي أو Homebrew عند الحاجة. تعمل أوامر المشروع الصريحة في هذا الدليل أيضًا مع 0.1.93.

يتضمن [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94) نطاق المشروع الصريح والاختيار التلقائي للمشروع في `install`. لمعاينة إضافة حزمة إلى مشروع:

```bash
npx universal-agent-plugins add context7 --target cursor --scope project \
  --project-root /path/to/project --dry-run
```

اختر مجلد مشروع موجودًا. تستخدم أوامر الإضافة والصيانة نطاق المستخدم افتراضيًا، لذا حدّد نطاق المشروع صراحةً. في 0.1.94، لا يختار `install` دون وسيطة مصدر المشروع إلا إذا احتوى المجلد الحالي نفسه على ملف عادي باسم `uap.toml`؛ وتكون الأولوية للنطاق الصريح ولا تُبحث المجلدات الأب. راجع الخطة قبل حذف `--dry-run`. يظل الاكتشاف الأصلي والثقة والمصادقة ونجاح استدعاءات MCP فحوصًا منفصلة عن كتابة ملفات الحزمة.

[مسار التثبيت الكامل](../use/install) ·
[توافق العملاء وحدود الاختبارات](../reference/client-compatibility)

### استعد مجموعة اعتماديات محفوظة {#restore-a-saved-dependency-set}

افحص ما تسجله بيئة المستخدم المختارة:

```bash
npx universal-agent-plugins deps status
```

لتصدير زوج محفوظ من `uap.toml` و`uap.lock`، نفّذ من مجلد تبادل:

```bash
npx universal-agent-plugins deps export
```

انسخ المجلد إلى آلة أخرى، وثبّت العملاء المستهدفين عليها، وشغّل هناك:

```bash
npx universal-agent-plugins deps import
npx universal-agent-plugins install --locked --scope user
```

لا يُنقل سوى الاتفاق والقفل بعد التحقق منهما. تبقى ملفات التعريف والإعدادات وبيانات الاعتماد والإيصالات وبيانات الإضافات منفصلة. لا يزال التثبيت يحتاج إلى المصادر الدقيقة أو الحزم المخزنة مؤقتًا التي جرى التحقق منها. للمشروع، احفظ الملفين في المستودع واستعد باستخدام `install --locked --scope project` صراحةً. راجع [الصيانة والاستعادة](../use/manage) لتفاصيل الاسترداد والنقل.

## بناء الإضافات {#build-plugins}

<a id="recommended-default"></a>
<a id="if-you-only-read-one-thing"></a>

تبدأ الحزمة المعيارية بملف `plugin.json` في الجذر، ويمكن أن تضم `skills/<name>/SKILL.md` أو `mcp.json` أو كليهما. يمكنك كتابة هذه الملفات يدويًا والتحقق من الحزمة بالمثبّت المنشور:

```bash
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin --target codex --dry-run
```

تفترض هذه الأمثلة أن `./my-plugin` يحتوي بالفعل على حزمة كاملة. يشرح [دليل البناء](../build/) إنشاء حزمة Skill صغيرة يدويًا وأدوات توليد الهيكل المبنية من المصدر.

**توفر التأليف:** لا يتضمن إصدار المثبّت الحالي من npm أو الإصدار الأصلي `agentplugins author`. ينفّذ المصدر الحالي توليد الهياكل والفحوص الساكنة واختبارات تشغيل MCP الصريحة ودورات التطوير وتهيئة الاعتماديات وصيانة JSON. استخدم [إعداد البناء من المصدر](../build/#build-from-source) لتلك الأوامر. تبقى أدلة إصدارات التأليف الأقدم مرتبطة بإصدارها الخاص؛ ولا تَعِد بما يتوفر في تنزيل المثبّت الحالي.

لا يثبت التحقق الساكن التشغيل أو OAuth أو التفعيل. يختلف دعم وسائل نقل MCP كذلك بحسب العميل؛ فمثلًا لا يدعم Codex نقل MCP المعلن عبر SSE. أبقِ متطلبات الحزمة والفحوص المتبقية واضحة في README.

[مواصفة Agent Plugins 1.0](https://agent-plugins.org/specification)
