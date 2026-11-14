---
title: "SDK de gestion TypeScript"
description: "Gérer les plugins depuis un hôte Node de confiance."
canonicalId: "page:sdk:typescript-management"
section: "sdk"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK de gestion TypeScript

Client typé de l'**API de gestion v1** pour un hôte Node/Electron **de confiance**. Le moteur Go conserve l'autorité sur l'état, les installations, les profils et la récupération.

## Versions

Le package source `universal-agent-plugins-management-sdk` est en version **0.1.1** et requiert Node **22.18+**. Le CLI **0.1.95+** fournit `manage serve` et `manage discover`. Vérifiez npm avant d'installer :

```bash
npm view universal-agent-plugins-management-sdk@0.1.1 version
npm install --save-exact universal-agent-plugins-management-sdk@0.1.1
```

Le SDK ne télécharge pas le moteur ; épinglez l'exécutable et son SHA-256. `createClient` est une entrée pure pour un endpoint privé, tandis que `/node` expose `startUap` et `discoverTargets`.

## Démarrer le moteur

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

Les chemins sont des exemples absolus contrôlés par l'hôte. En fonctionnement normal, omettez `stateRoot` et `dependenciesRoot` pour partager les verrous et l'état canonique du CLI. Ne transmettez jamais le bearer token, les profils privés ou le bootstrap stdout au renderer. `close()` termine uniquement le processus possédé.

## Revue et exécution

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

`prepare` génère un plan, `apply` l'accepte après l'accord de l'utilisateur et `wait` surveille le résultat. Sauvegardez `planId` et `requestId` **avant** l'appel ; réutilisez-les si la réponse est perdue. Un délai ou AbortSignal sur `wait` **n'annule pas** une opération acceptée ; `cancel` demande l'annulation sans garantie de rollback. Affichez `partial`, `failed`, `interrupted` et `environment.pendingOperations`.

`discoverTargets` inspecte les clients et profils sans les installer. `discoverSkills` / `skills.preview` / `skills.discard` encadrent la découverte ; `skills.local.prepare` exige des racines autorisées par l'hôte. Certaines suppressions nécessitent une désinstallation native manuelle et **un nouveau plan** avec `externalUninstalled: true`.

**Attention :** `checkUpdates` existe dans le SDK 0.1.1, mais **CLI 0.1.95 et antérieur renvoie HTTP 404**. Utilisez un moteur plus récent ou une build source épinglée ; la vérification n'installe rien. Vérifiez `npm run build`, `npm run typecheck`, `npm test`, `npm run test:consumer`, puis un E2E natif distinct.

[README complet](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md).
