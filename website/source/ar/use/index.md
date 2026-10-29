---
title: "استخدام الإضافات"
description: "اختر التثبيت أو التأليف، ثم اتبع المسار المناسب."
canonicalId: "page:use:index"
section: "use"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# استخدام الإضافات {#use-plugins}

يثبّت Universal Agent Plugins حزمًا معيارية لوكلاء الذكاء الاصطناعي الذين تختارهم. يمكن أن تضم الحزمة Skills أو إعدادات خادم MCP أو كليهما. يمكنك التثبيت لحسابك، أو إبقاء اعتماديات مشروع بعينه منفصلة.

## احصل على CLI {#get-the-cli}

باستخدام Node.js 22 أو أحدث، شغّل غلاف npm مباشرةً:

```bash
npx universal-agent-plugins search docs
```

للاستخدام المتكرر، ثبّته تثبيتًا عامًا:

```bash
npm install -g universal-agent-plugins
agentplugins version
```

يمكنك أيضًا استخدام Homebrew:

```bash
brew install 777genius/agentplugins/agentplugins
```

يوفر [دليل التثبيت الأصلي](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) تعليمات macOS وLinux وWindows. لا يحتاج المدير الأصلي إلى Node.js، لكن الإضافات قد تحتاج إلى بيئات تشغيل خاصة بها. تستخدم أمثلة هذا المسار `agentplugins`؛ ويمكن تشغيل أوامر المثبّت نفسها عبر `npx universal-agent-plugins`.

## اتبع خطوات التثبيت {#follow-the-installation-journey}

1. [ابحث عن حزمة وافحصها](./install). راجع مصدرها ومتطلباتها المسبقة.
2. عاين الأهداف والنطاق المقصودين باستخدام `--dry-run`.
3. طبّق التثبيت الذي راجعته واتبع تعليمات تفعيله.
4. [حافظ على الحالة المثبّتة](./manage) باستخدام أوامر العرض والتحديث والإصلاح والإزالة.
5. استعد مجموعة اعتماديات مشتركة من `uap.toml` و`uap.lock` عند الحاجة.

[CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94) هو المثبّت الأصلي الحالي. قد تتأخر قنوات مديري الحزم؛ تحقّق من `agentplugins version` قبل الاعتماد على سلوك 0.1.94. استخدم المثبّت الأصلي أو Homebrew إذا كانت القناة المختارة أقدم. يتوفر نطاق المشروع الصريح منذ 0.1.93. تعمل معظم الأوامر افتراضيًا ضمن نطاق المستخدم. منذ 0.1.94، يختار `install` من دون وسيطة مصدر أو نطاق صريح مشروعًا فقط إذا وُجد ملف عادي باسم `uap.toml` في المجلد الحالي نفسه؛ ولا يبحث في المجلدات الأب. حدّد `--scope project` في أوامر صيانة المشروع. يختلف دعم المشروع وتفعيل العميل بحسب الهدف.

## اقرأ النتيجة على المستوى الصحيح {#read-the-result-at-the-right-level}

قد تبقى بعد التثبيت إعادة تحميل العميل، أو مطالبة بالثقة، أو تفعيل يدوي، أو خطوة OAuth. تعني نتيجة **Prepared** أن هناك عملًا متبقيًا داخل العميل. اقرأ نتيجة كل هدف بدل اعتبار العملية بأكملها دليلًا على نجاح التشغيل. راجع [توافق العملاء](../reference/client-compatibility) للاطلاع على الأدلة وراء الملاحظات الأصلية المرتبطة بإصدارات محددة.

## إذا كنت تؤلف حزمة {#if-you-are-writing-a-package}

ابدأ من [بناء الإضافات](../build/). ما تسلّمه هو مجلد يحتوي على `plugin.json` في جذره ومكونات الحزمة. ولا يحتاج مستخدم المثبّت إلى تشغيل أدوات التأليف المبنية من المصدر.

| الأمر | الغرض |
| --- | --- |
| `agentplugins validate ./my-plugin` | قراءة حزمة محلية والتحقق منها بغرض التثبيت. |
| `agentplugins doctor` | تشخيص الحالة التي يديرها المثبّت. |
| `agentplugins author validate ./my-plugin` | فحص المطابقة وجاهزية التأليف في بناء من المصدر يدعم التأليف. |
| `agentplugins author doctor ./my-plugin` | فحص أدلة المشروع وسلسلة الأدوات في ذلك البناء. |

لا يتضمن إصدار المثبّت الحالي من npm أو الإصدار الأصلي `agentplugins author`.
