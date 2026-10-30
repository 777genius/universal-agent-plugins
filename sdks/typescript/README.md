# UAP Management SDK for TypeScript

A typed client for the existing Go installer. Plugin resolution, ownership,
prepared authority, manifest/lock publication and recovery stay in Go.
The package's pure entry point has no filesystem, process, Electron or UI dependency.
The separate `/node` entry point owns an explicitly provisioned Go process.
The SDK never downloads or updates an engine executable.

## Install or consume a packed build

The source package is `universal-agent-plugins-management-sdk` version `0.1.0`.
SDK versions and npm publication are independent of CLI releases. Verify that the
exact SDK version is available on npm:

```sh
npm view universal-agent-plugins-management-sdk@0.1.0 version
```

Only after that check returns `0.1.0`, install it from the registry:

```sh
npm install --save-exact universal-agent-plugins-management-sdk@0.1.0
```

If that version has not been published, build and pack it from a clean, reviewed
source checkout instead:

```sh
(
  set -euo pipefail
  cd sdks/typescript
  npm ci --ignore-scripts --no-audit --no-fund
  npm run build
  npm run typecheck
  npm test
  npm run test:consumer
  npm pack --ignore-scripts
)
```

Install the resulting `universal-agent-plugins-management-sdk-0.1.0.tgz` in the
consuming project using its absolute path, and retain its digest and source commit
with the lockfile. A local build is not evidence of a public npm publication.

The release workflow builds versioned tarballs from immutable
`management-sdk-vX.Y.Z` tags and attests their exact bytes. After the initial
owner bootstrap, public publication uses npm trusted publishing with npm
provenance. See the
[changelog](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/CHANGELOG.md)
for release notes and migration from the earlier `@uap/management-sdk`
development tarball.

## Compatibility

| SDK | Engine (`agentplugins` CLI) | Management API | Node.js |
| --- | --- | --- | --- |
| 0.1.x | 0.1.95 or newer | v1 | 22.18 or newer |

CLI 0.1.95 is the first published release with `manage serve` and
`manage discover`; CLI 0.1.94 and older do not have `manage`. The management API
version is independent of the engine and package versions. Startup and discovery
refuse an engine that does not report management API v1
(`UNSUPPORTED_API_VERSION`); the SDK does not compare engine version strings.
Use `requiredCapabilities` to refuse an engine without a feature your host needs.

`checkUpdates` is a newer development-source capability. It requires an SDK
and engine built from a reviewed revision containing that operation; CLI 0.1.95
predates it. See [Explicit update checks](#explicit-update-checks-development-source)
below before choosing the engine and SDK for that method.

## Provision the engine

Provision an exact released engine and pin it in the consuming application's
build evidence. The repository installer accepts an explicit version, verifies
the release asset against the release `checksums.txt` and can record the
verified SHA-256 (see [native installation](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)).
A Unix-shell example:

```sh
curl -fsSLo ./agentplugins-install.sh \
  https://raw.githubusercontent.com/777genius/universal-agent-plugins/agentplugins-v0.1.95/install.sh
AGENTPLUGINS_VERSION=0.1.95 AGENTPLUGINS_BIN_DIR="$PWD/bin" \
  AGENTPLUGINS_OUTPUT_FILE="$PWD/agentplugins-engine.env" sh ./agentplugins-install.sh
./bin/agentplugins version
./bin/agentplugins manage serve --help
./bin/agentplugins manage discover --help
```

`agentplugins-engine.env` records the exact version, release tag, asset and
verified SHA-256. Keep it with the SDK version and its lockfile integrity. On
Windows, `install.ps1` accepts `-Version` and `-BinDir`. Package-manager channels
may lag; check the installed version. Both help checks must succeed before supplying the executable to `startUap` or `discoverTargets`.
They establish command availability, not installation or native runtime success.

To use engine changes that are not released yet, build the CLI from a clean,
reviewed repository checkout with the repository's Go 1.26.10 toolchain and pin
the source revision and executable digest instead:

```sh
uap_engine_revision=$(git rev-parse HEAD)
mkdir -p bin
(cd cli && go build -trimpath -o ../bin/agentplugins-management ./cmd/agentplugins)
./bin/agentplugins-management manage serve --help
./bin/agentplugins-management manage discover --help
sha256sum ./bin/agentplugins-management
```

For a source build, record `uap_engine_revision` alongside the executable digest
and the SDK tarball digest.
On macOS, use `shasum -a 256` for the executable digest. On Windows, build an
`.exe` output and use `Get-FileHash -Algorithm SHA256`.

The CLI root registers `manage` directly; no authoring linker flag is needed.
Its management startup captures profile authority and uses the shared installer
composition. The [wire contract](https://github.com/777genius/universal-agent-plugins/blob/main/contracts/management/README.md)
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

## Explicit update checks (development source)

The current development engine adds `client.checkUpdates({ all: true })` or
`client.checkUpdates({ aliases: ['docs', 'tools'] })`. Use an SDK tarball and engine
executable built from the same reviewed source revision containing this operation;
CLI 0.1.95 predates it. The method performs a disposable read-only network check
and does not save freshness, change the agreement, or install files.
Ordinary `client.environment()` stays passive and reports unchecked moving intent
as `check_unavailable`.

A check returns `complete`, `changed`, per-entry `installed`/`available` immutable
identities, requested selectors, and shared dependency owner edges. `current`
means the selected source was successfully checked; `pinned` means exact intent
is retained. `check_unavailable` and `blocked` cannot be treated as current.
A current root can still have changed shared dependencies, so inspect the overall
`changed` flag and every returned entry. Canceling this read-only HTTP request
stops checking; it does not create a retained mutation operation. An available
candidate always needs a fresh `prepare({ action: 'update', ... })` review before
apply, including drift, profile, source eligibility and stale-input checks.

## Start an owned engine

Use the pinned management-enabled executable verified above in `executable`.

```ts
import { startUap } from 'universal-agent-plugins-management-sdk/node';

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
import { createClient } from 'universal-agent-plugins-management-sdk';
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
The clean packed-consumer check installs the actual `.tgz`, checks that it ships
only `package.json`, `README.md`, `LICENSE` and compiled `dist` files, and checks
browser-safe public types, both export entry points and runtime imports without
monorepo aliases. Required CI runs the build, typecheck, lifecycle tests and
packed-consumer check on Linux, macOS and Windows through the shared
`management-sdk.yml` workflow. Generated-contract drift is checked on Linux.

Regenerate wire types from the repository root:

```sh
node contracts/management/generate.ts
node contracts/management/generate.ts --check
```

Never edit `src/generated/schema.d.ts` directly. The SDK version with its lockfile
integrity and the engine digest belong in the consuming app's build evidence.

## Release

Maintainers release from an immutable tag on a `main` commit whose `package.json`
already has the release version, and add the release notes to `CHANGELOG.md`
in the same change. Set `SDK_RELEASE_VERSION` to that stable version (for example,
`0.1.1`) and `SDK_RELEASE_COMMIT` to the full reviewed `main` commit SHA:

```sh
(
  set -euo pipefail
  version="${SDK_RELEASE_VERSION:?Set SDK_RELEASE_VERSION to the package version}"
  commit="${SDK_RELEASE_COMMIT:?Set SDK_RELEASE_COMMIT to the reviewed main commit SHA}"
  git tag -a "management-sdk-v${version}" -m "Management SDK ${version}" "$commit"
  git push origin "management-sdk-v${version}"
  gh workflow run management-sdk-release.yml --repo 777genius/universal-agent-plugins \
    --ref "management-sdk-v${version}" -f "version=${version}" -f publish=false
)
```

The [release workflow](https://github.com/777genius/universal-agent-plugins/blob/main/.github/workflows/management-sdk-release.yml)
refuses a ref that is not that tag, a tag that is not on `main`, a package name
or version that differs, a version that already exists on npm, and a version
lower than the current `latest`. It runs the same qualification as required CI
on Linux, macOS and Windows. A build job without signing or publishing
credentials then packs and checks one tarball and uploads it as an artifact; a
separate job that runs no repository code attests those exact bytes. Only
stable `X.Y.Z` versions are accepted; they are published with the `latest`
dist-tag.

`publish=true` additionally publishes those exact bytes through npm trusted
publishing after the `npm-management-sdk` environment approves the job. Before
publishing, the job verifies the tarball digest and its GitHub attestation and
refuses an OIDC identity that is not this repository, workflow, tag and
environment. Afterwards it checks the registry metadata, `latest` tag,
provenance (tag, commit and workflow), `npm audit signatures`, the downloaded
bytes, and a clean registry install with both entry points. The same consumer
check is available locally after a release:
`npm run test:consumer -- --registry-version="${SDK_RELEASE_VERSION:?Set SDK_RELEASE_VERSION to the published version}"`.

Preconditions for `publish=true`, checked by the workflow where possible:

- The GitHub environment `npm-management-sdk` exists with required reviewers,
  no administrator bypass and a deployment policy that allows only the tag
  pattern `management-sdk-v*`. Without it the workflow refuses to publish.
- A tag ruleset protects `management-sdk-v*` from updates and deletion.
- The package already exists on npm, and its trusted publisher names GitHub
  Actions, `777genius` / `universal-agent-plugins`, the workflow filename
  `management-sdk-release.yml` and the environment `npm-management-sdk`, with
  direct `npm publish` allowed.

npm can configure a trusted publisher only for an existing package, so the
first version is a one-time bootstrap by the npm package owner. Run the
workflow with `publish=false`, then publish that run's attested tarball once.
Take the expected commit from your own up-to-date clone, not from the artifact.
Set `SDK_RELEASE_VERSION` to the intended version and `SDK_RELEASE_RUN_ID` to
the successful `publish=false` workflow run ID:

```sh
(
  set -euo pipefail
  version="${SDK_RELEASE_VERSION:?Set SDK_RELEASE_VERSION to the package version}"
  run="${SDK_RELEASE_RUN_ID:?Set SDK_RELEASE_RUN_ID to the successful publish=false run ID}"
  git fetch origin main "refs/tags/management-sdk-v${version}:refs/tags/management-sdk-v${version}"
  commit=$(git rev-parse "management-sdk-v${version}^{commit}")
  git merge-base --is-ancestor "$commit" origin/main
  test "$(gh run view "$run" --repo 777genius/universal-agent-plugins --json headSha,conclusion,event \
    --jq 'select(.conclusion == "success" and .event == "workflow_dispatch") | .headSha')" = "$commit"
  gh run download "$run" --repo 777genius/universal-agent-plugins \
    -n "management-sdk-${version}" -D "management-sdk-${version}"
  cd "management-sdk-${version}"
  sha256sum --check checksums.txt
  test "$(cat source-commit.txt)" = "$commit"
  gh attestation verify "universal-agent-plugins-management-sdk-${version}.tgz" \
    --repo 777genius/universal-agent-plugins \
    --signer-workflow github.com/777genius/universal-agent-plugins/.github/workflows/management-sdk-release.yml \
    --source-ref "refs/tags/management-sdk-v${version}" --source-digest "$commit" --deny-self-hosted-runners
  npm publish "./universal-agent-plugins-management-sdk-${version}.tgz" \
    --access public --tag latest --provenance=false
)
```

The subshell stops at the first failed check, before `npm publish`. On macOS,
replace `sha256sum --check` with `shasum -a 256 --check`.

npm provenance can only be generated on a supported CI provider, so this one
local publication passes `--provenance=false`; the GitHub attestation still
binds its bytes to the tag commit. Configure the trusted publisher when the
next version is ready: a new configuration expires unless a publication through
it succeeds within two days. After that publication succeeds, set the package's
publishing access to "Require two-factor authentication and disallow tokens".
The workflow never creates the package or uses a token; if the package is
missing it fails closed.

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
the other native matrix cells. Set `SDK_SMOKE_REF` to the committed branch
whose native journeys should run:

```sh
gh workflow run agentplugins-native-clients.yml \
  --repo 777genius/universal-agent-plugins \
  --ref "${SDK_SMOKE_REF:?Set SDK_SMOKE_REF to the committed branch}" \
  -f sdk_linux_only=true -f sdk_target=linux-amd64
```

If Linux runners are queued, select `-f sdk_target=darwin-arm64` to run the same
two isolated SDK journeys on macOS 15. The default remains `linux-amd64`; the
receipt and uploaded source identity record the selected platform.
