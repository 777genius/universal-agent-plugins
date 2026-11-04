---
title: "TypeScript Management SDK"
description: "Керування плагінами з довіреного Node-хоста."
canonicalId: "page:sdk:typescript-management"
section: "sdk"
locale: "uk"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# TypeScript Management SDK

Це типізований клієнт **Management API v1** для довіреного Node/Electron-хоста. Усі операції над файлами, станом, профілями й відновленням виконує окремий Go engine.

## Версії

SDK `universal-agent-plugins-management-sdk` має версію вихідного пакета **0.1.1**, вимагає Node.js **22.18+**. CLI **0.1.95+** уперше надає `manage serve` та `manage discover`. Перевірте наявність npm-публікації, перш ніж встановлювати:

```bash
npm view universal-agent-plugins-management-sdk@0.1.1 version
npm install --save-exact universal-agent-plugins-management-sdk@0.1.1
```

SDK сам не завантажує engine. Зафіксуйте виконуваний файл і SHA-256, перевірте Management API **v1**. Основний імпорт `createClient` працює з уже відомим довіреним endpoint; `/node` експортує `startUap` і `discoverTargets`.

## Запуск engine

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

У прикладі шляхи умовні й абсолютні. За звичайної інтеграції **не** задавайте `stateRoot` / `dependenciesRoot`, щоб використовувати спільний стан CLI; ізольовані корені створюйте свідомо. Не передавайте bearer token, приватні профілі чи bootstrap stdout у renderer.

## План і результат

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

`prepare` готує план, `apply` приймає його після перегляду користувачем, `wait` спостерігає. Зберігайте `planId` і `requestId` перед apply; повтор використає ті самі ID. Таймаут wait **не** скасовує роботу: для спроби скасування є `cancel`. Покажіть стани `partial`, `failed`, `interrupted` та `environment.pendingOperations`.

`discoverTargets` тільки перевіряє нативні клієнти й приватні профілі. `discoverSkills` / `skills.preview` / `skills.discard` керують часом життя пошуку; `skills.local.prepare` редагує лише дозволені host roots. Для деяких native removal потрібні ручне видалення та **новий** план з явним `externalUninstalled:true`.

**Несумісність:** SDK `checkUpdates` вимагає endpoint, якого **немає в CLI 0.1.95 і старіших** (HTTP 404). Він тільки перевіряє оновлення; потрібен новіший engine або pinned source build. Тести: `npm run build`, `npm run typecheck`, `npm test`, `npm run test:consumer`; E2E з native клієнтом окремо.

[README SDK](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md).
