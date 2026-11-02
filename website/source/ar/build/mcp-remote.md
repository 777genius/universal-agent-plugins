---
title: "بناء حزمة MCP بعيدة"
description: "صِف نقطة نهاية MCP موجودة مع التمييز بين الإعداد والاتصال."
canonicalId: "page:build:mcp-remote"
section: "build"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# بناء حزمة MCP بعيدة {#build-a-remote-mcp-package}

**التأليف من المصدر:** تتطلب أوامر هذه الصفحة [بناءً من المصدر يدعم التأليف](./#build-from-source). لا يتضمن إصدار المثبّت الحالي `agentplugins author`.

اختر MCP البعيد عندما يكون الخادم موجودًا بالفعل وتحتاج الحزمة إلى وصف كيفية اتصال الوكيل به. يسجل التأليف الرابط دون الاتصال به. وتظل مسؤولًا عن معرفة متطلبات النقل والمصادقة للخدمة قبل تسليم الحزمة للمستخدمين.

## قدّم نقطة النهاية صراحةً {#supply-the-endpoint-explicitly}

تستخدم الأمثلة `https://mcp.example.com/mcp` كقيمة توضيحية في الوثائق. استبدلها بنقطة نهاية MCP الحقيقية لخدمتك عند إعداد الحزمة. يقبل أمر التأليف الرابط بوصفه إعدادًا؛ ولا يتحقق من أن العنوان يقدم MCP أو أن حسابك يستطيع استخدامه.

باستخدام بناء المصدر الداعم للتأليف:

```bash
agentplugins author init ./docs-service \
  --template mcp-remote \
  --name docs-service \
  --description 'Connect to the team documentation MCP service' \
  --url https://mcp.example.com/mcp
```

استخدم وجهة غير موجودة تحت مجلد أب موجود. يتطلب القالب رابط HTTPS مطلقًا، أو HTTP على عنوان loopback. لا تُقبل بيانات الاعتماد أو الأجزاء اللاحقة لـ `#` أو رموز الرابط غير المحسومة كمدخلات للقالب. لا تضع كلمة مرور في الرابط.

## افهم الملفات {#understand-the-files}

```text
docs-service/
  plugin.json
  mcp.json
  README.md
  .gitignore
```

يحدد البيان الجذري هوية الحزمة. يصف `mcp.json` الخادم تحت `mcpServers`؛ ويستخدم هذا القالب `type: streamable-http` والرابط المقدم. لا ينشئ خادمًا أو يضيف Skill. استخدم [الحزمة الهجينة](./hybrid) إذا احتاجت الحزمة أيضًا إلى تعليمات تشرح متى تُستدعى أدواتها.

راجع README ووثّق الغرض من نقطة النهاية ومتطلبات الحساب ومكان إرشادات المصادقة للمستخدمين. أبقِ الأسرار خارج الحزمة. وجود رابط في ملف مطابق ليس دليلًا على خدمة تعمل.

## افحص الإعداد ودعم الأهداف {#check-configuration-and-target-support}

```bash
agentplugins author validate ./docs-service
agentplugins author inspect ./docs-service --target codex,claude
agentplugins author test ./docs-service
agentplugins author compat ./docs-service --target codex,claude
agentplugins author doctor ./docs-service
```

يمكن أن يعرض `inspect` المكونات والمتطلبات غير المحسومة. يقيّم `compat` الدعم الساكن للمحوّلات لدى العملاء المختارين؛ ولا يتطلب أي منهما تثبيت أولئك العملاء. لا يتصل Doctor بنقطة النهاية.

## أبقِ العمل المتبقي واضحًا {#keep-the-remaining-work-visible}

ينبغي أن تحدد أدلة المؤلف الساكنة وسيلة النقل والعملاء المقصودين. ما زال على المثبّت لدى المستلم تقييم سياسة التثبيت وعرض تعليمات التفعيل. قد يحتاج المستخدم إلى تسجيل الدخول أو الموافقة على الوصول أو بدء جلسة عميل جديدة. ويحتاج اكتشاف الأدوات واستدعاؤها وقت التشغيل إلى أدلة منفصلة.

إذا فشلت الفحوص الساكنة، فصحح الإعداد المقروء أولًا. وإذا تعذر الوصول إلى نقطة النهاية لاحقًا، فتقصّ الخدمة أو اتصال العميل بدل تفسير نجاح اختبار ساكن آخر كفحص للاتصال.

تابع إلى [تفسير التقارير](./checks) و[التسليم إلى المخطّط](./handoff).
