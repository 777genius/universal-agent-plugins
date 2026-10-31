---
title: "بناء حزمة MCP عبر stdio"
description: "جهّز خادم Node MCP محليًا مع إبقاء إعداد التشغيل والتنفيذ منفصلين."
canonicalId: "page:build:mcp-stdio"
section: "build"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# بناء حزمة MCP عبر stdio {#build-a-stdio-mcp-package}

**التأليف من المصدر:** تتطلب الأوامر التالية [بناءً من المصدر يدعم التأليف](./#build-from-source). وهي غير مضمّنة في إصدار المثبّت الحالي.

اختر stdio عندما توفر الحزمة عملية محلية تتواصل عبر الإدخال والإخراج القياسيين. يدعم الهيكل Node.js 22 أو أحدث ويستخدم SDK الرسمي لـ MCP. ولا يؤدي إنشاؤه إلى تثبيت الاعتماديات أو بدء خادم.

## أنشئ الهيكل {#create-the-scaffold}

اختر وجهة غير موجودة تحت مجلد أب موجود:

```bash
agentplugins author init ./local-tools \
  --template mcp-stdio \
  --name local-tools \
  --description 'Provide local tools through a Node MCP server' \
  --runtime node
```

`node` هو مدخل بيئة التشغيل المدعوم لهذا القالب القابل للنقل. ليس علم تشغيل Python أو Go جزءًا من عقد هذا الأمر الحالي.

## راجع الملفات {#review-the-files}

```text
local-tools/
  plugin.json
  mcp.json
  package.json
  package-lock.json
  src/
    server.mjs
  README.md
  .gitignore
```

يعلن `mcp.json` الخادم `local-tools` بالأمر `node` والوسيطة `${PLUGIN_ROOT}/src/server.mjs`. يُحل رمز الحزمة داخل الحزمة المختارة؛ ولا يعني مجلد shell الحالي. يبدأ المصدر بأداة `hello` صغيرة. كيّفها مع مهمتك وأرسل سجلات التطبيق إلى stderr لإبقاء stdout متاحًا لبروتوكول MCP.

راجع بيان الاعتماديات وملف القفل. يكتبهما init، لكنه لا يشغّل npm أو يثبت قدرة المضيف على استيراد SDK.

## اجمع أدلة ساكنة {#gather-static-evidence}

```bash
agentplugins author validate ./local-tools
agentplugins author inspect ./local-tools
agentplugins author test ./local-tools
agentplugins author compat ./local-tools --target codex,claude
agentplugins author doctor ./local-tools
```

هذه فحوص ساكنة. يفحص Doctor بيانات وصفية محدودة للمشروع والملفات التنفيذية؛ ولا يثبت تثبيت الاعتماديات أو نجاح المصافحة أو عمل أداة. فسّر [طبقات الأدلة](./checks) منفصلةً.

## هيّئ القالب الذي يتعرف عليه النظام {#bootstrap-the-recognized-template}

لبنية Node المولّدة مع ملف القفل المطابق، يمكن للمصدر الحالي معاينة الاعتماديات وتثبيتها:

```bash
agentplugins author bootstrap ./local-tools --dry-run
agentplugins author bootstrap ./local-tools
```

يتطلب تطبيق bootstrap حاليًا Linux ودعم نظام الملفات المرتبط بمقابضه؛ وتبلغ المنصات الأخرى عن `bootstrap_platform_unsupported`. يستطيع التشغيل التجريبي فحص بنية معروفة دون تثبيت الاعتماديات.

راجع المعاينة قبل التطبيق. يستخدم bootstrap تثبيت npm المقفل مع تعطيل برامج lifecycle النصية. ويقبل البنية المولّدة المعروفة وملفات الحزمة المطابقة وغياب `node_modules`؛ وليس بديلًا عامًا لمدير الحزم. قد تؤدي ملفات اعتماديات معدلة أو ملفات قفل متنافسة أو مخرجات موجودة إلى الرفض. عالج متطلب البنية المبلّغ عنه بدل إضافة علم للكتابة فوق الملفات. قد يستخدم جلب الاعتماديات الشبكة.

## شغّل فحص MCP صريحًا واحدًا {#run-one-explicit-mcp-check}

يتطلب تنفيذ stdio المحلي حاليًا Linux و`bwrap` ‏(bubblewrap) على مستوى النظام، واحتواء عمليات يعمل، بما فيه cgroup v2 المفوّض، ودعم نطاقات الأسماء، وNode يُحل مساره ضمن بيئة تشغيل النظام (`/usr` أو `/bin` أو `/lib` أو `/lib64`). يشترط الأمر نجاح فحوص احتواء العمليات وbubblewrap قبل تشغيل كود الحزمة، ويرفض المضيفين الذين لا يستطيع إثبات الاحتواء عليهم. لا يستوفي تثبيت Node في مجلد مدير إصدارات خاص بالمستخدم عقد التشغيل هذا. تبقى الفحوص الساكنة متاحة بصورة منفصلة.

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

ينفّذ هذا العملية المحلية المختارة ويفحص تهيئة MCP واكتشاف الأدوات. يتطلب استدعاء `hello` ملفًا صريحًا لوسائط JSON؛ راجع [مثال فحص التشغيل](./checks#evaluate-mcp-runtime-explicitly). وللتطوير المتواصل:

```bash
agentplugins author dev ./local-tools --server local-tools
```

احتفظ بهوية المحرك والحزمة والمنصة الدقيقة في نتيجة التشغيل. لا يثبت فحص تأليف مباشر أن الخادم نفسه يعمل عبر كل وكيل أصلي، أو أن عميلًا منح الثقة أو أكمل المصادقة.

## جهّز التسليم {#prepare-the-handoff}

اشرح الأدوات ومتطلب Node.js وملفات الاعتماديات وأدلة التشغيل في README. حدّد الخادم المختار وأي أدوات أو بيئات غير مختبرة. ثم شارك الحزمة عبر [التسليم إلى المثبّت](./handoff). أضف تعليمات عبر [Skills إضافية](./skills) عندما يحتاج المستخدمون إلى إرشاد إلى جانب الأدوات.
