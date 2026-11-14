---
title: "TypeScript प्रबंधन SDK"
description: "विश्वसनीय Node होस्ट से प्लगइन प्रबंधन करें।"
canonicalId: "page:sdk:typescript-management"
section: "sdk"
locale: "hi"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# TypeScript प्रबंधन SDK

विश्वसनीय **Node/Electron host** के लिए **Management API v1** का टाइप किया हुआ क्लाइंट। फ़ाइलों, प्रोफ़ाइल, इंस्टॉलेशन, state और recovery का अधिकार **Go engine** के पास रहता है।

## संगतता

`universal-agent-plugins-management-sdk` स्रोत पैकेज का संस्करण **0.1.1** है; Node.js **22.18+** चाहिए। CLI **0.1.95+** में `manage serve` और `manage discover` हैं। npm में संस्करण की वास्तविक उपलब्धता पहले जाँचें:

```bash
npm view universal-agent-plugins-management-sdk@0.1.1 version
npm install --save-exact universal-agent-plugins-management-sdk@0.1.1
```

SDK engine डाउनलोड नहीं करता; executable और SHA-256 अलग से pin करें। मुख्य `createClient` entry पहले से trusted private endpoint के लिए है; `/node` से `startUap` और `discoverTargets` मिलते हैं।

## अपना Go engine चलाएँ

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

उदाहरण के absolute paths host तय करता है। सामान्य उपयोग में `stateRoot` / `dependenciesRoot` न दें ताकि CLI के canonical state और locks साझा हों; अलग वातावरण केवल जानबूझकर बनाएँ। **Bearer token, private profiles या bootstrap stdout renderer/browser में न भेजें।** `close()` केवल अपने child process को बंद करता है।

## बदलाव की समीक्षा और परिणाम

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

`prepare` review plan बनाता है, `apply` मानवीय पुष्टि के बाद स्वीकार करता है, `wait` स्थिति देखता है। Apply **से पहले** `planId` और `requestId` सुरक्षित रखें; response खोने पर **वही ID** फिर उपयोग करें। `wait` timeout या AbortSignal से स्वीकृत बदलाव **रद्द नहीं होता**; `cancel` सिर्फ cancellation माँगता है, rollback की गारंटी नहीं। `partial`, `failed`, `interrupted` तथा `environment.pendingOperations` दिखाएँ।

`discoverTargets` केवल client और profile पहचानता है; `discoverSkills` / `skills.preview` / `skills.discard` discovery lifecycle संभालते हैं; `skills.local.prepare` सिर्फ अनुमत host roots बदलता है। कुछ native removals में पहले मैनुअल uninstall और फिर नए `externalUninstalled: true` plan की ज़रूरत होती है।

**संस्करण सीमा:** SDK 0.1.1 में `checkUpdates` है, लेकिन **CLI 0.1.95 और पुराने संस्करण HTTP 404 लौटाते हैं**। उचित नया engine या pinned source build लें; यह केवल read-only update check है। `npm run build`, `npm run typecheck`, `npm test` और `npm run test:consumer` के बाद अलग native E2E जाँचें।

[विस्तृत SDK README](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md)।
