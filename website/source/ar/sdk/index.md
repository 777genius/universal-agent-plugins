---
title: "SDK والتكاملات"
description: "اختيار واجهة SDK المناسبة للخطافات والاكتشاف والتثبيت والإدارة."
canonicalId: "page:sdk:index"
section: "sdk"
locale: "ar"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK والتكاملات

استخدم حزم SDK عندما تريد **دمج Universal Agent Plugins داخل تطبيقك** أو معالجة أحداث الخطافات الأصلية. أما التثبيت المعتاد للإضافات فموضح في [استخدام الإضافات](../use/).

## اختر واجهة SDK

| المهمة | الدليل | الواجهة |
| --- | --- | --- |
| معالجة خطافات الوكيل | [Go Hooks](./go-hooks) | `github.com/777genius/plugin-kit-ai/sdk` |
| معرفة الوكيل الذي استدعى الخطاف | [اكتشاف المضيف](./host-detection) | `sdk/hostdetect` |
| تضمين التثبيت والتحديث والإصلاح في Go | [Go Installer](./go-installer) | `install/integrationctl/agentplugins/installer` |
| إدارة الإضافات من Node/Electron | [TypeScript Management](./typescript-management) | `universal-agent-plugins-management-sdk` |

## الإصدارات والحدود

- **Go Hooks:** الوحدة المنشورة `github.com/777genius/plugin-kit-ai/sdk@v1.3.0`؛ يُحدَّد الاستقرار **لكل حدث** على حدة.
- **اكتشاف المضيف:** تتوافر `sdk/hostdetect` في المصدر الحالي؛ تأكد من وجودها في **وسم الإصدار المحدد** الذي تعتمد عليه.
- **Go Installer:** وحدة مستقلة؛ ثبّت مراجعة المصدر والاعتماديات، ولا تفترض أنها تحمل إصدار hooks نفسه.
- **TypeScript Management:** إصدار المصدر **0.1.1** ويتطلب Node.js **22.18+** وManagement API **v1**؛ تأكد من النشر الفعلي على npm.
- **محرك الإدارة Go:** يوفر CLI **0.1.95+** الأمرين `manage serve` و`manage discover`؛ لكن `checkUpdates` **غير متاح في CLI 0.1.95 والإصدارات الأقدم**.

تتعرف `hostdetect.Detect` على **الوكيل الذي استدعى الخطاف الحالي**، بينما تفحص `discoverTargets` **وجود العملاء وملفاتهم الخاصة**؛ وتعرض `Engine.Discover` المحولات المسجلة صراحةً في Go. لا تثبّت أي منها إضافات تلقائيًا. التحقق من الحزمة وكتابة الملفات وتفعيل العميل ومصادقة MCP خطوات مستقلة.

تابع إلى [Go Hooks](./go-hooks) أو [الاكتشاف](./host-detection) أو [Go Installer](./go-installer) أو [TypeScript SDK](./typescript-management).
