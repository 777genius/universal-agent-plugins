---
title: "SDK de gestão TypeScript"
description: "Gerenciar plugins com o motor Go a partir do Node."
canonicalId: "page:sdk:typescript-management"
section: "sdk"
locale: "pt"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK de gestão TypeScript

Cliente tipado da **Management API v1** para um host Node/Electron **confiável**. O engine Go controla o estado, os perfis, as instalações e a recuperação.

## Compatibilidade

O pacote-fonte `universal-agent-plugins-management-sdk` está em **0.1.1**, exige Node **22.18+**. O CLI **0.1.95+** fornece `manage serve` e `manage discover`. Verifique a publicação npm antes de instalar:

```bash
npm view universal-agent-plugins-management-sdk@0.1.1 version
npm install --save-exact universal-agent-plugins-management-sdk@0.1.1
```

O SDK não baixa o engine; fixe seu executável e SHA-256. `createClient` é a entrada sem dependências de processo para um endpoint privado; `/node` exporta `startUap` e `discoverTargets`.

## Iniciar um engine próprio

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

Os caminhos são exemplos absolutos controlados pelo host. Em instalações normais omita `stateRoot` e `dependenciesRoot` para compartilhar o estado canônico e locks do CLI. Nunca exponha token bearer, perfis privados ou bootstrap stdout no renderer. `close()` encerra só o processo sob controle do host.

## Preparar e acompanhar

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

`prepare` cria um plano, `apply` o confirma após revisão humana e `wait` observa seu resultado. Persista `planId` e `requestId` **antes** do apply e reutilize os mesmos IDs se perder a resposta. Timeout/AbortSignal em `wait` **não cancela** a mutação; `cancel` solicita cancelamento, mas não garante rollback. Mostre `partial`, `failed`, `interrupted` e `environment.pendingOperations`.

`discoverTargets` detecta clientes e perfis sem instalar; `discoverSkills` / `skills.preview` / `skills.discard` gerem sessões de descoberta; `skills.local.prepare` precisa de raízes de autoria autorizadas. Algumas remoções nativas exigem desinstalação manual e **novo plano** com `externalUninstalled: true`.

**Atenção:** `checkUpdates` existe no SDK 0.1.1, mas o **CLI 0.1.95 e anteriores respondem HTTP 404**. Use um engine mais novo ou build de fonte fixada; essa verificação não instala nada. Teste `npm run build`, `npm run typecheck`, `npm test` e `npm run test:consumer`, além do E2E nativo.

[README do SDK](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md).
