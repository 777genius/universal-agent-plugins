---
title: "TypeScript Management SDK"
description: "إدارة الإضافات من مضيف Node موثوق."
canonicalId: "page:sdk:typescript-management"
section: "sdk"
locale: "ar"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# TypeScript Management SDK

هذه حزمة عميل ذي أنواع محددة لـ**Management API v1** تعمل داخل مضيف **Node/Electron موثوق**. يحتفظ محرك **Go** بصلاحية حالة الملفات وملفات التعريف والتثبيت والاسترداد.

## التوافق

إصدار مصدر الحزمة `universal-agent-plugins-management-sdk` هو **0.1.1** ويتطلب Node.js **22.18+**. يوفر CLI **0.1.95+** الأمرين `manage serve` و`manage discover`. تحقق من نشر النسخة على npm قبل تثبيتها:

```bash
npm view universal-agent-plugins-management-sdk@0.1.1 version
npm install --save-exact universal-agent-plugins-management-sdk@0.1.1
```

لا تقوم الحزمة بتنزيل محرك Go؛ ثبت ملف التنفيذ وتحقق من SHA-256 مستقلاً. يوفّر المدخل الأساسي `createClient` واجهة لعنوان خاص موثوق، بينما يصدّر `/node` الدالتين `startUap` و`discoverTargets`.

## بدء المحرك

```ts
import { startUap } from 'universal-agent-plugins-management-sdk/node';

const host = await startUap({
  executable: '/absolute/path/to/pinned/agentplugins',
  config: {
    stateRoot: '/absolute/private/uap-state',
    dependenciesRoot: '/absolute/private/uap-dependencies',
    scope: 'user',
    targets: [{
      id: 'claude-profile',
      client: 'claude',
      configRoot: '/absolute/private/claude-profile',
      executable: '/absolute/path/to/claude',
    }],
  },
});

try {
  const environment = await host.client.environment();
  console.log(environment.entries.map((entry) => entry.alias));
  // Inspect environment.pendingOperations before admitting new work.
} finally {
  await host.close();
}
```

المسارات في المثال مطلقة ويحددها المضيف. عادةً احذف `stateRoot` و`dependenciesRoot` لمشاركة الحالة والأقفال المعتادة مع CLI؛ استخدم مسارات منفصلة للعزل المقصود فقط. **لا تكشف bearer token أو الملفات الخاصة أو stdout bootstrap لواجهة renderer أو المتصفح**. تغلق `close()` العملية التي يملكها المضيف فقط.

## مراجعة العملية ومتابعتها

```ts
const plan = await host.client.prepare({
  action: 'install',
  source: 'example/plugin',
  alias: 'example',
  targetIds: ['claude-profile'],
});
if (!plan.confirmable) {
  throw new Error(plan.conflicts.map((item) => item.message).join('\n'));
}
// Display exact planned actions, targets, warnings and conflicts first.
// Persist plan.planId AND requestId in durable host state before apply.
const requestId = crypto.randomUUID();
const accepted = await host.client.apply(plan.planId, requestId);
const operation = await host.client.wait(accepted.operationId, {
  timeoutMs: 120_000,
  onProgress: (snapshot) => console.log(snapshot.phase),
});
console.log(operation.state, operation.result?.nextActions);
```

تنشئ `prepare` خطة للمراجعة، وتقبلها `apply` بعد تأكيد المستخدم، ثم تراقب `wait` النتيجة. احفظ `planId` و`requestId` **قبل apply**، وأعد استخدام المعرفين نفسيهما إن فُقد الرد. انتهاء مهلة `wait` أو AbortSignal **لا يلغي العملية المقبولة**؛ تطلب `cancel` الإلغاء من دون ضمان التراجع. اعرض حالات `partial` و`failed` و`interrupted` و`environment.pendingOperations`.

تفحص `discoverTargets` العملاء وملفاتهم دون تثبيت. تدير `discoverSkills` و`skills.preview` و`skills.discard` جلسات اكتشاف Skills، بينما `skills.local.prepare` يحتاج إلى جذور مصرح بها من المضيف. تتطلب بعض عمليات الإزالة إزالةً يدويةً داخل العميل ثم **خطة جديدة** تحتوي `externalUninstalled: true`.

**قيد مهم:** تتضمن SDK 0.1.1 الدالة `checkUpdates`، لكن **CLI 0.1.95 والأقدم يعيد HTTP 404** لهذا المسار. استخدم محركًا أحدث مؤهلًا أو إصدارًا من مصدر مثبت؛ هذا الفحص للقراءة فقط ولا يثبت تحديثات. نفذ `npm run build` و`npm run typecheck` و`npm test` و`npm run test:consumer`، ثم اختبارات E2E أصلية مستقلة.

[README الكامل](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md).
