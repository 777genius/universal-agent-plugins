---
title: "SDK de gestión en TypeScript"
description: "Gestionar plugins desde una aplicación Node de confianza."
canonicalId: "page:sdk:typescript-management"
section: "sdk"
locale: "es"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK de gestión en TypeScript

Cliente tipado para la **Management API v1** desde un host Node/Electron **de confianza**. El engine Go mantiene la autoridad de los perfiles, las instalaciones, el estado y la recuperación.

## Compatibilidad

Versión de fuente del paquete `universal-agent-plugins-management-sdk`: **0.1.1**, Node **22.18+**. El CLI **0.1.95+** incluye `manage serve` y `manage discover`. Verifica la publicación de npm antes de instalar:

```bash
npm view universal-agent-plugins-management-sdk@0.1.1 version
npm install --save-exact universal-agent-plugins-management-sdk@0.1.1
```

La biblioteca no descarga el engine: fija el ejecutable y su SHA-256. `createClient` es el punto de entrada puro para endpoints privados; el subpath `/node` expone `startUap` y `discoverTargets`.

## Iniciar el proceso

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

Las rutas son ejemplos de rutas absolutas controladas por el host. En una integración normal omite `stateRoot` y `dependenciesRoot` para compartir los locks y el estado canónico del CLI. No expongas el bearer token, los perfiles privados ni el bootstrap stdout al renderer o al navegador. `close()` termina solo el proceso propio.

## Revisar y ejecutar

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

`prepare` crea un plan, `apply` lo acepta tras revisar los cambios y `wait` observa el resultado. Persiste `planId` y `requestId` **antes** de aplicar; tras perder una respuesta reutiliza ambos ID. Timeout/AbortSignal de `wait` **no cancela** una operación aceptada; `cancel` solicita una cancelación que no garantiza rollback. Muestra `partial`, `failed`, `interrupted` y `environment.pendingOperations`.

`discoverTargets` observa clientes y perfiles sin instalar nada. `discoverSkills`/`skills.preview`/`skills.discard` gestionan sesiones de descubrimiento; `skills.local.prepare` necesita raíces de autoría explícitas. Algunas desinstalaciones exigen una acción nativa manual y **un plan nuevo** con `externalUninstalled: true`.

**Importante:** `checkUpdates` existe en SDK 0.1.1, pero **CLI 0.1.95 y anteriores responden 404** a ese endpoint. Necesitas un engine más reciente o fuente fijada; la comprobación no instala ni actualiza. Verifica `npm run build`, `npm run typecheck`, `npm test` y `npm run test:consumer` además del E2E nativo.

[README completo](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md).
