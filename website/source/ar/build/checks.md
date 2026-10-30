---
title: "فحص الحزمة وأدلتها"
description: "ميّز بين الفحوص الساكنة وتنفيذ MCP الصريح من المصدر وأدلة تفعيل العميل."
canonicalId: "page:build:checks"
section: "build"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# فحص الحزمة وأدلتها {#check-a-package-and-its-evidence}

**التأليف من المصدر:** تتطلب أوامر `agentplugins author` هذه [بناءً من المصدر يدعم التأليف](./#build-from-source). لا يتضمنها إصدار المثبّت الحالي. شغّل الأمثلة ذات المسارات النسبية من المجلد الأب للحزمة، واستخدم الجذر الدقيق نفسه طوال العملية.

## شغّل دورة التأليف غير المتصلة {#run-the-offline-authoring-loop}

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

يفحص التحقق الإعدادات المعيارية المقروءة وجاهزية التأليف. ويعرض الفحص المكونات والمتطلبات غير المحسومة. يفحص `test` دون أعلام التشغيل الإعدادات وسلامة الحزمة وSkills وMCP بصورة ساكنة. لا تبدأ هذه الأوامر خوادم أو تنفّذ تعليمات Skill.

## اقرأ طبقات الأدلة {#read-the-evidence-layers}

| الطبقة | ما تثبته | ما يحتاج إلى فحص آخر |
| --- | --- | --- |
| المطابقة | البيانات المعيارية المقروءة تستوفي المخططات والملف المعياري المضمّن. | السلوك المفيد في وكيل معين. |
| سلامة المضيف | تستوفي المدخلات فحوص نظام الملفات والمضيف المحدودة. | تفويض المستخدم للتثبيت أو التنفيذ. |
| الجاهزية | تستوفي الأدلة المتاحة السياسة التي جرى فحصها. | الموافقة على الإصدار العام أو قبول المنصة. |
| التوافق | الدعم الساكن للأهداف والمكونات المختارة. | الاكتشاف والتفعيل الفعليان في عميل حقيقي. |
| سلسلة الأدوات | بيانات وصفية محدودة للمشروع والملفات الأصلية والملفات التنفيذية. | حسم الاعتماديات وبدء الخادم. |
| التشغيل | يعرض فحص MCP المنفّذ صراحةً المراحل التي رصدها. | الأدوات أو العملاء أو مسارات OAuth أو البيئات الأخرى غير المختبرة. |

اقرأ المطابقة والجاهزية العامة منفصلتين. فقد يكون البيان صالحًا مع بقاء أدلة المكونات أو المضيف غير مكتملة. أبقِ حالة «لم يُقيّم» ظاهرة بدل تحويلها إلى نجاح.

## حدّد العملاء صراحةً {#select-clients-explicitly}

```bash
agentplugins author compat ./review-helper --target codex,claude
agentplugins author inspect ./review-helper --target codex,claude
agentplugins author doctor ./review-helper
agentplugins author capabilities
```

يحتاج `compat` إلى معرّفات أهداف مختلفة مفصولة بفواصل، ولا يقبل `all`. لا تتطلب هذه الفحوص الساكنة تثبيت العملاء. يفحص Doctor الملفات الأصلية المقروءة وبيانات الملفات التنفيذية الوصفية دون عمليات أو شبكة. ويعرض Capabilities مخططات هذا المحرك وبيانات العملاء والأوامر المنفذة؛ ولا يعتمد حزمة أو يثبت تأهيل إصدار عام.

## قيّم تشغيل MCP صراحةً {#evaluate-mcp-runtime-explicitly}

يتطلب تشغيل stdio المحلي حاليًا Linux وbubblewrap على مستوى النظام وNode ضمن مجلدات تشغيل النظام. يجب نجاح فحوص احتواء العمليات، بما فيها cgroup v2 المفوّض، وفحوص نطاقات أسماء bubblewrap قبل البدء. راجع [متطلبات stdio](./mcp-stdio). لا يستخدم تشغيل Streamable HTTP صندوق عزل العمليات المحلي هذا، ويتطلب موافقة صريحة منفصلة على الشبكة.

للحزمة المنشأة وفق [درس stdio](./mcp-stdio)، ثبّت اعتمادياتها المقفلة التي يتعرف عليها النظام بصورة منفصلة، ثم اختر اسم الخادم الدقيق من `mcp.json`:

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

ينفّذ هذا الأمر من المصدر عملية MCP المختارة ويفحص التهيئة واكتشاف الأدوات. لاستدعاء أداة، قدّم `--tool` و`--fixture` معًا. ملف الاختبار هو كائن JSON بمسار نسبي إلى الحزمة يحتوي على وسائط الأداة. لأداة `hello` في القالب، أنشئ `fixtures/hello.json` بمحتوى `{}`:

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools \
  --tool hello --fixture fixtures/hello.json
```

اسمح بالوصول إلى الشبكة صراحةً عند استخدام Streamable HTTP:

```bash
agentplugins author test ./docs-service --runtime mcp --server docs-service \
  --allow-network
```

استخدم هوية الخادم الحقيقية من حزمتك. المهلة الافتراضية عشر ثوانٍ، ويمكن ضبط `--deadline` على مدة موجبة لا تتجاوز دقيقة. افحص أدلة التهيئة والأدوات واستدعاء الأداة الاختياري والتنظيف كلًا على حدة. النجاح هنا ليس تنفيذًا عبر وكيل مثبّت، ولا يثبت OAuth أو أدوات أخرى أو نشرًا عامًا.

## كرر التطوير عبر دورة المصدر {#iterate-with-the-source-development-loop}

```bash
agentplugins author dev ./local-tools --server local-tools
```

يشغّل `dev` دورات MCP محدودة مع تغيّر الحزمة؛ أوقفه باستخدام Ctrl+C. استخدم `--once` لدورة واحدة وعند طلب مخرجات JSON. تتطلب الخوادم البعيدة `--allow-network`، وتظل استدعاءات الأدوات بحاجة إلى زوج tool/fixture صريح. لا يثبّت الأمر شيئًا في عميل أصلي.

## التقط تقريرًا قابلًا للمراجعة {#capture-a-reviewable-report}

```bash
agentplugins author validate ./review-helper --format json
agentplugins author inspect ./review-helper --format json
```

استخدم `--format human` للمخرجات المقروءة و`--no-color` عند الحاجة. يكشف `--include-root` الجذر المختار؛ تجنب المسارات الخاصة غير الضرورية في التشخيصات المشتركة. يضيف `--release-policy` فحوصًا محدودة لسلامة الإصدار إلى فحوص الحزمة، وليس موافقة على النشر. تدعم أوامر Skills الفرعية مجموعة أصغر من الأعلام؛ راجع المساعدة الخاصة بها.

## نفّذ صيانة JSON عن قصد {#maintain-json-deliberately}

يمكن للمصدر الحالي تخطيط تطبيع حتمي لملف معياري واحد مختار، ثم الكتابة إليه فقط مع `--write` الصريح:

```bash
agentplugins author normalize ./review-helper --document plugin.json
agentplugins author normalize ./review-helper --document plugin.json --write
```

اختر `mcp.json` بدلًا منه إذا كان هو المستند المراد تطبيعه. يقرأ الاستيراد الأصلي ملف Claude MCP واحدًا محددًا بصيغة JSON صارمة، ولا يبحث في ملفات التعريف. لا يقبل المستورد الحالي سوى إدخالات stdio الآمنة التي تضم `command` و`args` اختياريًا؛ ويتجاوز إدخالات HTTP/URL و`env` والحقول الإضافية للخادم، حتى عندما يقبلها العميل المصدر. خطّط لوجهة مطلقة جديدة غير موجودة قبل التطبيق:

```bash
agentplugins author import native ./claude.json --from claude \
  --output /absolute/path/to/new-plugin --name new-plugin \
  --description 'Imported Claude MCP package'
```

راجع الخوادم الآمنة والمتجاوزة والحقول غير المدعومة. أضف `--write` لكتابة الحزمة التي تم التحقق منها إلى الوجهة غير الموجودة. تُتجاوز الخوادم غير الآمنة أو المحتوية على بيانات اعتماد؛ افحص النتيجة بدل افتراض انتقال كل خادم أصلي. لا يتوفر هنا استيراد من عميل أصلي آخر.

## اختر حد الأوامر المناسب {#choose-the-right-command-boundary}

يقرأ `agentplugins validate ./review-helper` الخاص بالمثبّت حزمة بغرض التثبيت؛ ويشخّص `agentplugins doctor` الحالة المُدارة. وهما منفصلان عن عمليات التأليف من المصدر. لا تصبح أعلام النطاق والأمان الخاصة بالمثبّت أعلام تأليف. ينشئ init ملفات فعلية؛ ويتطلب التطبيع والاستيراد `--write` لتطبيق خطتيهما. تابع إلى [تسليم الحزمة](./handoff).
