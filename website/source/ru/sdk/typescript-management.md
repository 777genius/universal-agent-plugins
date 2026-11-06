---
title: "TypeScript Management SDK"
description: "Управление плагинами через отдельный Go-движок и Management API v1."
canonicalId: "page:sdk:typescript-management"
section: "sdk"
locale: "ru"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# TypeScript Management SDK

SDK предназначен для **доверенного Node.js/Electron/desktop-хоста**, которому нужно обнаруживать агенты и управлять плагинами. За установку, конфигурацию, права, транзакции и восстановление отвечает **Go-движок**. TypeScript — типизированный клиент Management API v1.

## Совместимость

| Компонент | Минимум |
| --- | --- |
| Пакет | `universal-agent-plugins-management-sdk`, версия исходников **0.1.1** |
| Node.js | **22.18+** |
| Go CLI с управлением | `agentplugins` **0.1.95+** с `manage serve` и `manage discover` |
| Протокол | **Management API v1**; несовместимый движок отклоняется при запуске |

Версии SDK и CLI не совпадают. Сначала проверьте публикацию конкретной версии npm:

```bash
npm view universal-agent-plugins-management-sdk@0.1.1 version
npm install --save-exact universal-agent-plugins-management-sdk@0.1.1
```

Если версии нет в npm, соберите и зафиксируйте [пакет из проверенного исходного кода](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md#install-or-consume-a-packed-build). SDK **не скачивает Go-движок**. Его версию и SHA-256 проверяйте отдельно.

## Два импорта

- Основной `universal-agent-plugins-management-sdk` экспортирует `createClient` для доверенного хоста с уже известным приватным HTTP endpoint и bearer capability. Он не использует filesystem/process API.
- `universal-agent-plugins-management-sdk/node` экспортирует `startUap` и `discoverTargets`. Этот слой запускает закреплённый `agentplugins manage serve` и управляет его жизненным циклом.

**Не передавайте bearer token, stdout bootstrap и приватные пути профилей в renderer, браузерное хранилище или журналы.**

## Подключение к движку

```ts
import { startUap } from 'universal-agent-plugins-management-sdk/node';

const host = await startUap({
  executable: '/absolute/path/to/pinned/agentplugins',
  config: {
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
  // Перед дальнейшими мутациями покажите environment.pendingOperations.
} finally {
  await host.close();
}
```

Указывайте **точный абсолютный executable** и явные профили. По умолчанию Go использует обычное состояние и блокировки общего CLI; `stateRoot` / `dependenciesRoot` нужны лишь для намеренно изолированного окружения. `startUap` проверяет приватный bootstrap, версию API и capabilities. `close()` завершает только собственный процесс; HTTP-ошибка не доказывает его завершение.

## Обнаружение доступных агентов

`discoverTargets({ executable, env?, signal?, timeoutMs? })` выполняет `manage discover`, не устанавливая файлы, не скачивая источники и не запуская агент. Результат включает `client`, `detectionStatus`, `available`, возможную причину и приватный `profile`. Пропавший клиент может оставаться доступен для просмотра старого binding, но не для новой установки. Не путайте это с [детектором hook payload](./host-detection).

## Изменение установленного пакета

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
// Покажите plan. Сохраните plan.planId и requestId ДО отправки apply.
const requestId = crypto.randomUUID();
const accepted = await host.client.apply(plan.planId, requestId);
const operation = await host.client.wait(accepted.operationId, {
  timeoutMs: 120_000,
  onProgress: (snapshot) => console.log(snapshot.phase),
});
console.log(operation.state, operation.result?.nextActions);
```

`prepare` возвращает план для просмотра, `apply` принимает его после подтверждения пользователя, `wait` наблюдает результат. Повтор после потери ответа использует **те же** `planId` и `requestId`. Таймаут `wait` / AbortSignal только прекращает наблюдение, **не** отменяет мутацию. Для отмены есть `cancel(operationId)`; завершение может успеть раньше. Учитывайте состояния `partial`, `failed`, `interrupted` и `environment.pendingOperations`. Закрывая **непринятый** план, вызовите `plans.discard`.

## Доступные операции

`session`, `catalog`, `catalogEntry` и `environment` читают состояние. `discoverSkills`, `skills.preview` и `skills.discard` помогают выбирать Skills; `skills.local.prepare` редактирует только явно разрешённые локальные Skills с проверкой дайджестов и backup. Для изменения плагина применяйте `prepare/apply/operation/wait/cancel`.

**Важно:** `checkUpdates()` из SDK 0.1.1 вызывает `POST /api/v1/environment/check-updates`, **которого нет в CLI 0.1.95 и ниже** (ответ HTTP 404). Нужен новый совместимый Go-движок или собранная из закреплённой ревизии версия. Этот вызов только проверяет свежесть источников, ничего не обновляет.

Если требуется удаление внутри нативного клиента, сначала покажите `EXTERNAL_UNINSTALL_REQUIRED`, попросите пользователя выполнить действие и **заново** создайте план с явным `externalUninstalled: true`. Не считайте это глобальным разрешением.

## Проверка

В `sdks/typescript` выполните `npm ci --ignore-scripts`, `npm run build`, `npm run typecheck`, `npm test`, `npm run test:consumer`. Протокольные тесты не заменяют E2E с настоящим движком и профилем. См. [полный README](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md) и [контракт Management API](https://github.com/777genius/universal-agent-plugins/blob/main/contracts/management/README.md).
