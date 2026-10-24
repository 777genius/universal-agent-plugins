# UAP Management SDK for TypeScript

A typed client for the existing Go installer. Plugin resolution, ownership,
prepared authority, manifest/lock publication and recovery stay in Go.
The package's pure entry point has no filesystem, process, Electron or UI dependency.
The separate `/node` entry point owns an explicitly provisioned Go process.

This source package has the development version `0.0.0-development`. It is consumed
as a built, integrity-pinned tarball until separately authorized public publication.
The SDK never downloads or updates an engine executable.

## Start an owned engine

```ts
import { startUap } from '@uap/management-sdk/node';

const host = await startUap({
  executable: '/absolute/path/to/pinned/agentplugins',
  config: {
    stateRoot: '/absolute/test-host/uap-state',
    dependenciesRoot: '/absolute/test-host/dependencies',
    scope: 'user',
    targets: [{
      id: 'claude-test-profile',
      client: 'claude',
      configRoot: '/absolute/test-host/claude-profile',
      executable: '/absolute/path/to/claude',
    }],
  },
});
try {
  const environment = await host.client.environment();
  // Render any pendingOperations before accepting more work or launching a runtime.
  console.log(environment.entries.map((entry) => entry.alias));
} finally {
  await host.close();
}
```

The host captures explicit roots and executable paths; ambient HOME does not select
target profiles. The authoritative `session.contextId` is derived by Go from captured
facts. It is separate from an application's navigation ID or optional startup label.
Unsupported scope/target capabilities are refused instead of falling back to user scope.

Startup sends one bounded JSON line to `agentplugins manage serve` on stdin. Stdin
stays open for this owned process. The private stdout bootstrap supplies an ephemeral
IPv4 loopback URL and bearer capability; neither is forwarded to UI or logs. The
helper verifies the authenticated session before returning. Optional
`requiredCapabilities` makes unsupported engine features fail during startup.

`close()` is idempotent. It closes stdin to request graceful shutdown, waits up to
`shutdownTimeoutMs` (10 seconds by default), then terminates only its owned process
if necessary. Interrupted accepted work requires subsequent inspection/recovery.
Closing a view or aborting a wait must not close a shared host instance.

## Prepare, accept and observe

```ts
const plan = await host.client.prepare({
  action: 'install', source: 'example/plugin', alias: 'example',
  targetIds: ['claude-test-profile'],
});
if (!plan.confirmable) throw new Error(plan.conflicts.map((item) => item.message).join('\n'));
// Display the plan's targets, actions and warnings. Persist both IDs in host state
// before sending apply. Retain the same request ID if its response is lost.
const requestId = crypto.randomUUID();
const accepted = await host.client.apply(plan.planId, requestId);
const operation = await host.client.wait(accepted.operationId, {
  onProgress: (snapshot) => console.log(snapshot.phase),
  timeoutMs: 120_000,
});
// Render succeeded, partial, failed, cancelled or interrupted explicitly.
// Materialized configuration can still require authentication/manual activation.
console.log(operation.state, operation.result?.nextActions);
```

A plan is diagnostic data plus an opaque retained handle. The client never serializes
it back as authority, reprepares automatically or retries a mutation with a new ID.
A repeated accepted request is resolved before plan expiry checks, including after
process restart. Conflicting input under the same request ID is refused.

`wait()` returns terminal failed/partial/interrupted snapshots as data. HTTP errors
throw `ManagementError` with `code`, `status` and `retryable`; network failures remain
network errors. Neither implies that an earlier accepted operation had no effects.
The server's outstanding `environment.pendingOperations` survives app restart.

An AbortSignal or waiting deadline stops observation only. Explicitly request
cooperative cancellation with `client.cancel(operationId)`, then observe the actual
outcome. Completion can win; cancellation does not promise rollback. Progress has
real phases and item counts, not invented completion percentages.

## Use an existing endpoint

```ts
import { createClient } from '@uap/management-sdk';
const client = createClient({ baseUrl, origin, token, fetch: customFetch });
```

Supply the exact bootstrap Origin when reaching a native loopback instance.
Browser applications should use their trusted backend; do not expose the native
UAP bearer capability to a renderer or browser storage. The pure SDK is useful in
any fetch-capable trusted host; it does not grant browser filesystem authority.

## Development and qualification

From this directory:

```sh
npm ci --ignore-scripts
npm run build
npm run typecheck
npm test
npm run test:consumer
```

New code is checked with pinned TypeScript 7.0.2. Node 22.18 or newer is required.
The child-process tests compile a tiny Go transport fixture using the repository's
Go toolchain. These tests prove SDK protocol/lifetime behavior, **not installation**.
Separate integration/Electron tests run the real Go engine and inspect native output.
The clean packed-consumer check installs the actual `.tgz`, checks browser-safe
public types, both export entry points and runtime imports without monorepo aliases.

Regenerate wire types from the repository root:

```sh
node contracts/management/generate.ts
node contracts/management/generate.ts --check
```

Never edit `src/generated/schema.d.ts` directly. A source pin, SDK tarball digest and
engine digest belong in the consuming app's build evidence; public npm or binary
release publication is a separate authorized action.
