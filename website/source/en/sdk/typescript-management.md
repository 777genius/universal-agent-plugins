---
title: "TypeScript Management SDK"
description: "Manage plugins from a trusted TypeScript host using the Go management engine."
canonicalId: "page:sdk:typescript-management"
section: "sdk"
locale: "en"
generated: false
translationRequired: true
---

# TypeScript Management SDK

The TypeScript SDK embeds Universal Agent Plugins management in a **trusted Node/desktop host**. The Go engine still owns package resolution, state, user/project authority, installation, removal and recovery; the TS client calls its authenticated **Management API v1**.

## Versions and prerequisites

| Component | Baseline |
| --- | --- |
| TypeScript package | `universal-agent-plugins-management-sdk`, source version **0.1.1** |
| Node.js | **22.18+** |
| Go engine | Released `agentplugins` **0.1.95+** for `manage serve` / `manage discover` |
| Protocol | Management API **v1**, checked at startup (not inferred from a CLI version string) |

SDK versions are independent of CLI releases. Verify the exact npm version before installation; if it has not been published, [build a reviewed packed source checkout](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md#install-or-consume-a-packed-build) instead of presenting a local tarball as an npm release.

```bash
npm view universal-agent-plugins-management-sdk@0.1.1 version
npm install --save-exact universal-agent-plugins-management-sdk@0.1.1
```

Pin and verify a suitable **engine executable** independently. The SDK does **not** fetch or update it. The [engine provisioning guide](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md#provision-the-engine) explains the native installer checksum and source-build alternatives.

## Two entry points

- `universal-agent-plugins-management-sdk`: pure, typed `createClient` for a trusted caller that already owns a private API endpoint and bearer capability; no filesystem, process or UI dependencies.
- `universal-agent-plugins-management-sdk/node`: `startUap` and `discoverTargets`; owns a **pinned** native Go executable and private profile detection. Do not expose its loopback token, bootstrap stdout or private profile facts to browser/renderer code.

## Start a managed engine

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

Paths are **illustrative absolute host-owned paths**. For ordinary integration, omit `stateRoot` and `dependenciesRoot` to share the engine's canonical state and mutation locks with the CLI; use overrides only for deliberately isolated environments. Host-provided `targets` capture exact profile authority. `startUap` starts `manage serve`, validates the private bootstrap and API version, and returns `{ client, session, close, hasExited }`. `close()` stops only its owned process. An HTTP failure alone does not mean the engine exited.

## Discover installed clients (without installing)

`discoverTargets({ executable, env?, signal?, timeoutMs? })` runs the pinned engine's `manage discover`. It returns native client presence, optional reasons and **private** canonical user-profile locators. This is not the [hook payload detector](./host-detection). Discovery never downloads source, runs agents or mutates native config. A disappeared client can still have a retained profile for inspection/removal; never auto-select it for a new install. Keep these facts in the trusted host.

## Prepare, accept, observe

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

`wait` only observes; a timeout/AbortSignal does **not** cancel an accepted mutation. To request cancellation use `client.cancel(operationId)`, then inspect its actual result. On lost responses, retain and reuse the **same** plan and request ID, not a new plan/ID. `environment().pendingOperations` contains durable work after app restarts. Terminal `partial`, `failed` and `interrupted` states are data, not automatic rollback. When an **unaccepted** review closes, call `client.plans.discard(plan.planId)`.

## API surface and limits

| Operation | Public method |
| --- | --- |
| Session, catalog, status | `session()`, `catalog()`, `catalogEntry()`, `environment()` |
| Plugin lifecycle | `prepare()`, `apply(planId, requestId)`, `operation(id)`, `wait(id)`, `cancel(id)` |
| Skill discovery/preview | `discoverSkills()`, `skills.preview()`, `skills.discard()` |
| Local Skill edits | `skills.local.prepare()`; then the usual review/apply flow |
| Explicit source freshness check | `checkUpdates()` — **not served by CLI 0.1.95 or older** |

`checkUpdates` is read-only and does not update installed files. Use a qualified **newer engine** or a pinned source build which implements `POST /api/v1/environment/check-updates`. A 0.1.95 engine returns HTTP 404; this is **not** an SDK startup failure. Required capabilities can reject unsupported engines during `startUap`, but this endpoint has no capability entry in SDK 0.1.1.

Some removals require manual native uninstall: show `EXTERNAL_UNINSTALL_REQUIRED`, ask the user to uninstall externally, then prepare a **new** plan with explicit `externalUninstalled: true` only for that action. Never assume prior confirmation applies to later operations.

Local Skill authoring requires host-captured `authoringRoots` and offers digest-checked files, explicit plan/apply, and retained backups. This does **not** silently grant project installation rights.

## Verify the integration

From `sdks/typescript` run `npm ci --ignore-scripts`, `npm run build`, `npm run typecheck`, `npm test` and `npm run test:consumer`. Wire and lifecycle unit checks are not native-client E2E proof; test the pinned engine and a real agent profile separately. Read the full [SDK README](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md) and [Management API contract](https://github.com/777genius/universal-agent-plugins/blob/main/contracts/management/README.md).

[Go Installer SDK](./go-installer) · [Host detection](./host-detection)
