---
title: "بناء حزمة هجينة"
description: "اجمع Skill واحدة وإعداد MCP واحدًا تحت هوية حزمة واحدة."
canonicalId: "page:build:hybrid"
section: "build"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# بناء حزمة هجينة {#build-a-hybrid-package}

**التأليف من المصدر:** تتطلب أوامر هذه الصفحة [بناءً من المصدر يدعم التأليف](./#build-from-source). لا يتضمن إصدار المثبّت الحالي `agentplugins author`.

تفيد الحزمة الهجينة عندما تشرح التعليمات كيف تُستخدم الأدوات ومتى. يقع المكوّنان تحت ملف بيان جذري واحد. وقد يدعمهما العميل بدرجات مختلفة، لذا أبقِ نتائج التوافق لكل مكوّن واضحة في التسليم.

## اختر جانب MCP صراحةً {#choose-the-mcp-side-explicitly}

علم الحزمة الهجينة هو `--mcp-template`. اختر `mcp-remote` أو `mcp-stdio`، ثم قدّم مدخلات القالب المطلوبة. لا تستنتج الحزمة الهجينة وسيلة النقل من الرابط أو الملفات المحلية أو بيئات التشغيل المثبّتة.

مثال لحزمة هجينة بعيدة:

```bash
agentplugins author init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Guide research using the team MCP service' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

الرابط قيمة توضيحية، وليس وعدًا بوجود خدمة. استبدله بنقطة النهاية الفعلية. اقرأ [متطلبات MCP البعيد](./mcp-remote).

## أو اختر عملية محلية {#or-choose-a-local-process}

لوجهة أخرى غير موجودة، يكون شكل stdio كما يلي:

```bash
agentplugins author init ./local-research \
  --template hybrid \
  --name local-research \
  --description 'Guide research using local Node tools' \
  --skill-name research-guide \
  --mcp-template mcp-stdio \
  --runtime node
```

لا تضف `--url` إلى خيار stdio. راجع [المتطلبات المسبقة وحدود تشغيل stdio](./mcp-stdio).

## اربط التعليمات بالأدوات {#connect-instructions-to-tools}

هذه البنية الأساسية لحزمة هجينة بعيدة:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

يُنشأ README و`.gitignore` أيضًا. تضيف الحزمة الهجينة عبر stdio مصدر Node وملفات الحزمة الموضحة في مسار stdio. راجع `SKILL.md` ليبيّن متى تُستخدم الأدوات وما المدخلات المطلوبة وكيفية الإبلاغ عن غياب الوصول. لا تطلب من الوكيل اعتبار نص Skill تفويضًا بالصلاحيات.

## افحص المكوّنين {#check-both-components}

```bash
agentplugins author skills validate ./research-helper
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper --target codex,claude
agentplugins author test ./research-helper
agentplugins author compat ./research-helper --target codex,claude
```

للنسخة المحلية، اختر `./local-research` بدلًا من ذلك. احتفظ بنتائج كل مكوّن: دعم العميل لـ Skill لا يعني تلقائيًا دعم وسيلة نقل MCP أو تفعيل الخدمة. ولا تلغي الحزمة المدمجة هذا القيد.

تابع إلى [إضافة Skills](./skills) أو [التسليم](./handoff).
