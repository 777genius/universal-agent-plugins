# UAP Management SDK for TypeScript

A typed client for the existing Go installer. Plugin resolution, ownership,
prepared authority, manifest/lock publication and recovery stay in Go.
The package's pure entry point has no filesystem, process, Electron or UI dependency.
The separate `/node` entry point owns an explicitly provisioned Go process.

This source package has the development version `0.0.0-development`. It is consumed
as a built, integrity-pinned tarball until separately authorized public publication.
The SDK never downloads or updates an engine executable.

## Provision a management-enabled source engine

The SDK needs the **current management-enabled source engine** as well as its
own development tarball. The published CLI 0.1.94 does not expose `manage` and
cannot run these examples. A generic pinned installer download is insufficient.
Pin a reviewed source commit and the SHA-256 of the resulting executable in the
consuming application's build evidence; provision that exact executable yourself.

From a clean, reviewed repository checkout, use Git and the repository's Go
1.26.10 toolchain. The following Unix-shell example starts at the repository root:

```sh
uap_engine_revision=$(git rev-parse HEAD)
mkdir -p bin
(cd cli && go build -trimpath -o ../bin/agentplugins-management ./cmd/agentplugins)
./bin/agentplugins-management manage serve --help
./bin/agentplugins-management manage discover --help
sha256sum ./bin/agentplugins-management
```

Record `uap_engine_revision` alongside that digest and the SDK tarball digest.
On macOS, use `shasum -a 256` for the executable digest. On Windows, build an
`.exe` output and use `Get-FileHash -Algorithm SHA256`. Both help checks must
succeed before supplying the executable to `startUap` or `discoverTargets`.
They establish command availability, not installation or native runtime success.

The current development CLI root registers `manage` directly; this build needs
no authoring linker flag. Its management startup captures profile authority and
uses the shared installer composition. The [wire contract](../../contracts/management/README.md)
defines that protocol and its independent capability versioning.

## Discover private user-profile facts

The Node-only `discoverTargets({ executable, env?, signal?, timeoutMs? })` runs the
same pinned engine's `manage discover` command and waits for its exit. It reads
native client presence and canonical profile locators without version probes,
client execution, source acquisition or filesystem mutations. The 10-second default
deadline and 64 KiB output limit bound the owned child. Both default and explicit
environments are filtered to OS variables and native profile locators; credentials,
proxy secrets and launch options are not inherited.

The versioned result contains `targets` with `client`, `label`, `detectionStatus`,
`available`, optional `reason`, and optional private `profile` startup facts. Only
qualified, safely inspected user profiles have `profile`. A profile remains present
when `detectionStatus` is `not_detected`, so a host can retain existing binding
inspection/removal authority after the client disappears. Such a target cannot be
selected for a new installation. Detected unsupported clients are display-only
observations with no profile. Never derive authority from their labels or display
paths, forward private profile facts to a renderer, or turn discovery into automatic
installation. Hosts preserve their own explicitly selected native profiles and can
merge additional discovered profiles into the captured startup context.

## Start an owned engine

Use the management-enabled source executable verified above in `executable`.

```ts
import { startUap } from '@uap/management-sdk/node';

const host = await startUap({
  executable: '/absolute/path/to/pinned/agentplugins-management',
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

The host captures explicit target roots and executable paths; ambient HOME does not select
target profiles. Omit stateRoot/dependenciesRoot in normal integration to share the canonical
existing UAP state and mutation locks with the CLI; Go resolves those defaults. Explicit
overrides are for isolated or alternate environments and must not silently split ownership. The authoritative `session.contextId` is derived by Go from captured
facts. It is separate from an application's navigation ID or optional startup label.
`session.environmentId` remains stable across executable/label changes, so unresolved
operations remain visible. Adding captured project authoring roots changes the context revision, not the stable environment identity. Unsupported scope/target capabilities are refused instead of falling back to user scope.

The default child environment preserves `AGENTPLUGINS_HOME` alongside OS/home/locale/proxy settings, so custom CLI state is shared. It inherits one spelling of each case-insensitive environment key, preferring uppercase proxy names. An explicit `env` is passed exactly as supplied and remains the host's responsibility.

Startup sends one bounded JSON line to `agentplugins manage serve` on stdin. Stdin
stays open for this owned process. The private stdout bootstrap supplies an ephemeral
IPv4 loopback URL and bearer capability; neither is forwarded to UI or logs. The
helper verifies the authenticated session before returning. Optional
`requiredCapabilities` makes unsupported engine features fail during startup.

`close()` is idempotent. It closes stdin to request graceful shutdown, waits up to
`shutdownTimeoutMs` (10 seconds by default), then terminates only its owned process
if necessary. A denied signal is not proof of exit: cleanup continues to SIGKILL
and rejects with `SHUTDOWN_TIMEOUT` if termination remains unconfirmed.
Interrupted accepted work requires subsequent inspection/recovery.
Closing a view or aborting a wait must not close a shared host instance.

The read-only `hasExited` property reports confirmed termination of that exact
owned child. It stays false for HTTP failures while the child is alive. The SDK
does not reconnect or restart automatically. A host may reconnect after confirmed
exit using the same captured authority, but must validate the new session and
inspect durable outstanding operations before admitting more work. Keep mutation
fences and original accepted/uncertain request identities until that inspection
and request-specific observation settle them. Unaccepted in-memory review handles
expire with the old process; do not silently reprepare or replay them.

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
// Render succeeded, partial, failed, canceled or interrupted explicitly.
// Materialized configuration can still require authentication/manual activation.
console.log(operation.state, operation.result?.nextActions);
```

A plan is diagnostic data plus an opaque retained handle. The client never serializes
it back as authority, reprepares automatically or retries a mutation with a new ID.
When a review dialog closes or its selection changes, call `client.plans.discard(plan.planId)`
to release its source resources. Discard is idempotent for absent/unaccepted plans and
refuses accepted plans; it does not cancel an accepted operation.

A repeated accepted request is resolved before plan expiry checks, including after
process restart. Conflicting input under the same request ID is refused.

`wait()` returns terminal failed/partial/interrupted snapshots as data. HTTP errors
throw `ManagementError` with `code`, `status` and `retryable`; network failures remain
network errors. Neither implies that an earlier accepted operation had no effects.
The server's outstanding `environment.pendingOperations` survives app restart. Environment entries include `declared`: only direct manifest dependencies can be updated or removed independently; transitive entries are observed and changed through their declared parent.

An AbortSignal or waiting deadline stops observation only. Explicitly request
cooperative cancellation with `client.cancel(operationId)`, then observe the actual
outcome. Completion can win; cancellation does not promise rollback. Progress has
real phases and item counts, not invented completion percentages.


## Skill discovery lifetime

`client.discoverSkills({ source })` retains a bounded source session for candidate
preview and selection. Keep that catalog while its discovery dialog is active.
When the source changes or the dialog closes, call `client.skills.discard(catalogId)`.
It is idempotent and releases source workspaces, allowing a long-lived host to
continue beyond the 16 simultaneously retained discovery sessions. Closing a
candidate preview alone should keep its containing catalog. An already prepared
installation plan owns an independent source snapshot and survives catalog discard.

An HTTPS candidate keeps the bytes established by its first successful preview.
If the publisher changes those bytes, another preview returns `PLAN_STALE` and
preparation refuses the changed source. Discard and rediscover the catalog before
reviewing that new content.

## External uninstall confirmation

Some native clients require the user to uninstall the owned package in that client
before UAP removes its binding. The first `prepare` returns `confirmable: false`
with `EXTERNAL_UNINSTALL_REQUIRED` conflicts naming the affected aliases and targets.
Show that review and let the user complete the external action. After their explicit
confirmation, prepare a **new** review with `externalUninstalled: true`;
`EXTERNAL_UNINSTALL_ATTESTED` warnings identify the exact removal bindings covered.
Only the newly reviewed plan can be applied. Discard the superseded unaccepted plan.

Applicability follows the actual prepared removal actions, not the intent's name.
An `update` or `sync` that prunes a native dependency can require the same explicit
confirmation, as can a complete environment `recover` convergence. A receipt-only
recovery or a request without an applicable removal refuses it with HTTP 422
`EXTERNAL_UNINSTALL_NOT_APPLICABLE`. Complete receipt recovery without the flag,
then request a separate convergence review. The SDK never supplies this attestation:
it must not be a default, a remembered preference, or automatically forwarded to a
later recovery. An accepted operation keeps its original request identity; changing
an attestation requires a fresh preparation and a new apply request ID.

## Local Skill authoring

Capture optional `authoringRoots: [{ id, path, scope, targetId? }]` from the trusted
host before startup. A project authoring root is a bounded local-file edit permission;
it does not enable native UAP project-scope installation. No HTTP request supplies an
absolute mutation path.

`client.skills.local.prepare({ action: 'upsert', rootId, skillName, files })` returns an
ordinary retained plan. Each file has `relativePath`, `expectedSHA256` (the exact old
bytes, or null for absence) and `contentBase64` (new bytes, or null for deletion).
The limit is 200 changes and 10 MiB total replacement bytes. The engine captures the
complete current tree and rechecks it plus authoritative ownership while holding
the same environment/native mutation locks as the CLI. Direct edits to managed
projections are refused; copy reviewed bytes to a different unowned Skill directory.

`action: 'delete'` prepares a complete directory removal. The result's
`localSkillBackup` identifies a retained reversible copy in private engine state, outside
native Skill discovery. `environment.localSkillBackups` exposes the latest retained
backups after host restart. Restore with a fresh
`skills.local.prepare({ action: 'restore', rootId, skillName, restoreOperationId })`
and the usual apply/observe flow. Uncertain `operation.action === 'local_skill'`
uses that same local restore route and `operation.localSkill`; generic native
recovery cannot settle unknown local file effects. Changed live files or backups
are retained and block unsafe restoration. Undo is retained for at least 30 days. Later acceptance may prune only verified
private backups and receipts with settled observations; changed backups and pending
journals are preserved. A maximum of 1024 retained observations bounds growth and
refuses new work when full rather than discarding unresolved identity.

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
Required CI runs the build, typecheck, lifecycle tests and packed-consumer check on
Linux, macOS and Windows. Generated-contract drift is checked on Linux.

Regenerate wire types from the repository root:

```sh
node contracts/management/generate.ts
node contracts/management/generate.ts --check
```

Never edit `src/generated/schema.d.ts` directly. A source pin, SDK tarball digest and
engine digest belong in the consuming app's build evidence; public npm or binary
release publication is a separate authorized action.

## Real engine qualification

The Native Client Runtime workflow runs this example in its macOS OpenCode job,
using the exact engine and pinned native client from that checkout's native proof.
Its artifact includes the SDK receipt and bounded process logs; both the receipt
and successful process termination are required.

After building, run the example with explicitly provisioned UAP and native client
executables and the public fixture at an immutable source commit. Select
`--client claude`, `--client codex`, or `--client opencode` (the default):

```sh
node examples/real-engine-smoke.ts --client claude \
  --executable /absolute/path/to/agentplugins \
  --client-executable /absolute/path/to/claude \
  --source 777genius/universal-agent-plugins@4555aa53844ee0078e9036cd82638afd67f77c16//repotests/testdata/agentplugins_commands_fixture \
  --output /absolute/test-output/sdk-native-evidence.json
```

The example creates disposable state, profile, HOME and XDG roots. It installs
through the actual Go engine. Native observation uses Claude's `plugin list` and
`plugin details`, Codex's `plugin list` and fresh app-server `skills/list`, or
OpenCode's `debug skill`. It checks exact fixture bytes, portable command discovery,
and withheld explicit-only commands. It also checks targeted pinned update, repair, removal, local
create/edit/delete and restart-visible undo. No model or account login is used.

For Claude/Codex, it changes the disposable managed package and requires reviewed
repair to replace that drift, restore exact native fixture bytes and leave no
pending operation. For OpenCode, it edits one separately owned native Skill and
requires the typed preservation refusal, unchanged edited bytes and pending
observation. It explicitly moves that conflicting tree aside as **manual
remediation** before preparing fresh recovery; the SDK never performs this step
automatically.

Codex removal first verifies the SDK's external-uninstall refusal, invokes the
real native `plugin remove` for the exact installed identity, and independently
confirms its absence before submitting a fresh explicit removal attestation.

Missing Claude/Codex managed packages are separately reconstructed through explicit
reviewed repair, with assertions on both the restored artifact and native bytes.
Generic recovery does not grant permission to replace package drift.

For crash qualification it prepares repair, drops the real accepted HTTP response
and kills only the owned Go process. OpenCode additionally starts with one missing
separately owned native Skill; Claude/Codex use the intact managed package.
After restart it reuses the persisted request identity, observes interrupted work,
prepares fresh recovery and requires the real native consumer to load the exact
bytes. Transport success alone cannot satisfy that assertion. The output records
checks incrementally, including failed stages and executable SHA-256 digests.

Linux requires the engine's ordinary-process containment prerequisites, including
`/proc/<pid>/task/<tid>/children`. Restricted containers without that interface
cannot qualify the real installation journey; the production refusal remains.
The existing Native Client Runtime workflow also has a manual `sdk_linux_only`
mode that runs just the Claude and Codex SDK journeys on the selected platform, without running
the other native matrix cells:

```sh
gh workflow run agentplugins-native-clients.yml \
  --repo 777genius/universal-agent-plugins \
  --ref <committed-branch> -f sdk_linux_only=true -f sdk_target=linux-amd64
```

If Linux runners are queued, select `-f sdk_target=darwin-arm64` to run the same
two isolated SDK journeys on macOS 15. The default remains `linux-amd64`; the
receipt and uploaded source identity record the selected platform.

### Additional user-scope clients and local execution

The same real-engine journey also supports Cline 3.0.73, Gemini 0.65.0,
GitHub Copilot CLI 1.0.103, Grok 1.0.67 and Qoder 1.1.78. Cline's own
`config skills --json` and Gemini's `skills list` must observe the exact installed
Skill bytes. Copilot, Grok and Qoder are checked through fresh native plugin and
Skill inventories. Their version, enabled state, identity and owned package/cache
paths must agree. Copilot's admitted route supports Skills while portable commands
remain unsupported: the journey verifies the exact fixture Skill and absence of
both fixture commands. Other clients also verify the exact rendered portable
command bytes and withhold the explicit-only command.
Copilot also uses a captured profile that differs from the engine's inherited
`COPILOT_HOME` and `HOME/.copilot`. Native inspection and the full lifecycle must
use the captured profile; the unselected ambient profile and its sentinel must
remain unchanged through repair, restart, recovery, removal and local Skill undo.
When removal requires external uninstall, the example performs
the exact native uninstall, verifies fresh absence, then prepares a new SDK review
with the explicit attestation.

Cline and Gemini use the separate-projection repair journey: edits are preserved,
partial failure remains visible, and recovery follows explicit manual preservation.
The OpenCode-specific typed preservation diagnostic is not asserted for other
clients. All five additional clients retain the accepted-response loss, owned-engine
termination, restart, idempotent replay and local Skill undo checks.

From the repository root, the local runner creates a deterministic fixture archive,
serves it over owned loopback HTTPS, and pins its SHA-256. This exercises the normal
HTTPS source acquirer without requiring a Git subprocess. The certificate is trusted
only in the child test environment. It neither changes the engine's source policy
nor relaxes native process containment.

```sh
(cd cli && go build -trimpath -o /absolute/tools/agentplugins ./cmd/agentplugins)
(cd sdks/typescript && npm ci --ignore-scripts && npm run build)
python3 scripts/run-sdk-native-local.py --client cline \
  --executable /absolute/tools/agentplugins \
  --client-executable /absolute/tools/cline \
  --output /absolute/new-evidence-directory
```

Provide a new output directory for each attempt. It contains the incremental SDK
receipt, bounded process logs and a source identity record with the commit, dirty
worktree flag, fixture hashes and executable hashes. Qualification requires both
a successful complete receipt and clean process termination. A dirty worktree run
is preliminary evidence, not a published-source qualification.

For Gemini, supply `--client-archive /absolute/gemini-cli-bundle.zip` with the
official 0.65.0 ZIP. The launcher must contain exactly `#!/bin/sh` and
`exec /absolute/node /absolute/gemini.js "$@"`. The runner verifies the archive,
every extracted bundle file, entrypoint and Node identity; a launcher hash alone
does not identify the executed JavaScript.

| Client route | Example selected by the local runner | Qualification boundary |
| --- | --- | --- |
| Cline, Gemini, Copilot, Grok, Qoder | `real-engine-smoke.ts` | Actual SDK lifecycle and fresh native plugin/Skill discovery; no model or login. |
| Pi 1.4.0 | `real-pi-engine-smoke.ts` | Actual SDK extension lifecycle, native auto-discovery and inert registered RPC command; Pi Skills/MCP are not claimed. |
| Kimi 2.3.1, Kiro 2.33.1 | `real-engine-policy-smoke.ts` | SDK install/update/repair/restart/removal only; authenticated native acceptance remains unverified. |
| Windsurf user MCP | `real-engine-policy-smoke.ts` | Owned MCP configuration lifecycle and refresh guidance; editor loading remains unverified. No client executable is required for this existing configuration route. |
| Cursor user editor | Not qualified by these examples | Existing manual activation route; a CLI/project observation does not qualify the editor's user package. |

Pi runs offline without a model, session, tools or MCP. Its fresh process must
load the installed extension automatically, execute the fixture's registered
command, and stop discovering it after removal. The journey also checks exact
fixture bytes and locked package digest, missing-package repair, edited-package
refusal, manual preservation, fresh repair and durable accepted-operation replay.

Kimi, Kiro and Windsurf receipts explicitly use `qualification: sdk-lifecycle-only`
and `nativeLoaded: false`. Native journeys use `sdk-native-lifecycle`. Configuration
success must not be relabeled as native loading.

The **Management SDK Native Packages** workflow runs only the Copilot, Grok and
Qoder journeys on Linux. These three need the process observation interface absent
from some local containers. It checks out the exact PR head, provisions pinned
native bytes, builds that engine/SDK, and uploads receipts bound to the source and
executable hashes. Existing required checks remain authoritative; this focused
workflow does not run the full native platform matrix.
