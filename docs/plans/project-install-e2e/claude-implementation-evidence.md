# Claude project skills-directory plugin evidence

Pre-code checkpoint, 2026-10-23. The all-client plan and both independent plan
review/fix rounds select one native skills-directory plugin at the selected
project's `.claude/skills/<physical-artifact>`. This extends the existing
projection/listing lifecycle; marketplace installation is not an alternate
fallback. Existing user behavior and `CLAUDE_CONFIG_DIR` authority stay intact.

## Pinned evidence inspected before code

- Official Linux x64 package `@anthropic-ai/claude-code-linux-x64` **2.1.263**:
  https://registry.npmjs.org/@anthropic-ai/claude-code-linux-x64/-/claude-code-linux-x64-2.1.263.tgz
- Pinned archive integrity:
  `sha512-0IrvpLd/0FP0acQw59T4Cvx/r4nwAXKBrW0WyhIXymzYWurPCLztB+Icu9MkeewAUI+p3PTXsSfmilv/n6XlAQ==`.
- Acquired binary SHA256
  `26d020351e8112f4006790f3cfce43b4c9df0c1bb1d0e542364d64151b81d5ba`.
- Local read-only version, plugin-help and plugin-list-help probes in the
  restricted disposable namespace all aborted with Bun exit **134**. Their
  logs are crash evidence, not successful CLI help or native project proof.
  No private host `/proc`, guard bypass, login, trust or model call was used.
- Existing committed hosted user evidence is indexed by
  `docs/evidence/client-compatibility-linux-amd64-0.1.53.json`: exact pinned
  version, plugin-list observations and user lifecycle/runtime records. That
  historical user evidence does not qualify this new project lane.
- Current official plugin CLI and installation documentation were opened before
  implementation:
  https://code.claude.com/docs/en/plugins/cli-reference and
  https://code.claude.com/docs/en/plugins/install . The CLI reference documents
  skills-directory list entries with id, scope, enabled and installPath, and
  project/user skills-directory sources. This is the inspected listing schema;
  a fixture runner is not claimed as native CLI help or execution.

## Implementation boundaries

The adapter will validate core's TargetContext, select exact `.claude` native
authority and `.claude/skills` watched layout, retain the original detected user
configuration environment, and run every project listing at the immutable
selected project cwd. Staging remains outside the watched skills directory.
Existing package projection, managed stdio launcher, command rendering,
transaction, receipt and ownership machinery are reused.

Listing accepts only the expected project `name@skills-dir` identity, enabled
at the exact managed path; another scope/path, disabled or unknown listing
cannot establish installation. Project activation attestation cannot override
recognized negative or unavailable native discovery evidence. Removal validates
the same selected scope/root and exact native identity before the shared owned
directory removal. It never invokes a user uninstall or edits native settings.

Contract/fixture evidence and new genuine hosted project native observations
will be recorded separately. The local Bun failure remains a pending native
probe boundary; no model task, account mutation, auto-trust, scanner upload or
credential creation is authorized to remove that boundary.

## Initial implementation checkpoint

The adapter now selects the exact project skills-directory plugin, validates
original user config/project/native root authority and fixed project cwd, and
retains the user-only listing entry point. Project listing additionally refuses
load errors and a mismatching optional projectPath. Exact enabled native listing
can establish installed activation; MCP initialization, skill execution and model
use remain separate observations. Project attestation is refused before generic
provider shortcuts, and project removal probes the same scope/cwd/config roots.

Production loader/planner/rendering/staging/provider contracts passed (0.074s)
with an inert command runner. Public installer contracts passed (0.251s) across
user/project-A/project-B install, unchanged update, exact supplied-pin repair
after drift, moving local source update, status and exact directory removal.
The public SDK requires its host to supply exact pinned bytes for repair; it
does not silently follow moved source. User/B state and native package bytes
remain unchanged by the selected project journey. These are fixture contracts,
not native-client evidence.

Full domain/planner packages passed (0.027s/0.044s). Existing Claude client
permission fixtures intentionally require an unprivileged process: under task
root, mode-bit denial checks fail because root can read those directories. The
compiled full client suite passed as uid 65534 without source/test changes or
skips. Existing user `TestClaude` provider contracts passed (0.166s).

The first exact-code lint found only capitalization/import-shadow issues, which
were corrected. Full-file size/architecture lint passed on the initial adapter
and provider scope without suppressions. The expanded public-installer lint
checkpoint encountered a task-host cache write ENOSPC; its final rerun and both
independent implementation reviews remain to be recorded. The actual native
project fixture gate is still pending; local help/version crash evidence and
historical hosted user evidence must not be relabeled as project discovery.

## Authored source-only native project gate

`TestAgentpluginsClaudeNativeProjectPluginLifecycle` adds a separate source-only
opt-in behind the unchanged disposable-host guard. It uses genuine pinned
`plugin list --json` and `plugin details` commands at selected project cwd after
actual UAP add, unchanged update, command-rule update, repair and removal. The
listing must be project-scoped, enabled and directly under selected
`.claude/skills`; fresh project-B processes must not see the fixture plugin.
Installed rendered command bytes are checked and foreign user/cache/A/B files
retain their hashes. No session, model request, trust edit, MCP initialization,
login or credential operation is added. A dedicated
`claude-project-plugin-evidence.json` records the separate native facts and
unobserved runtime boundaries.

The native matrix's explicit `--project-components` source-only selection also
requires this test for Claude. Released, draft, prepared and prepare-only inputs
refuse that selection; required-test accounting rejects missing/skipped tests.
The Python matrix contracts pass (17 tests). The authored Go native fixture has
not run: its first compile attempt was blocked before package setup by the full
task-host Go cache. This remains an authored gate, not native discovery evidence.
On integration, the shared selector/workflow delta must retain the union of the
independently authored Codex and Claude project rows and source conditions.

## Final author verification after bounded resource recovery

The completed Codex research binary intermediate was removed only after its
preserved binary SHA256 was reverified; all pinned provenance and actual help
outputs remain intact, and hosted jobs acquire independent official artifacts.
That recovered bounded memory-backed cache headroom without broad cache pruning
or deleting unrelated work. The guarded Claude native fixture now compiles;
the selected inert `TestClaudeDiscoveredSkills` parser contract passes (0.008s).
No native project client was executed by that compile/parser run.

The public SDK fixture now includes stdio projection with an inert trusted
launcher and explicit persistent-data references. User/A/B each receive separate
data roots; update, exact-pin repair and removal retain all marker bytes and
original data authority. Its postcommit fault seam makes native listing
unavailable only after the package/binding commit: the outcome stays incomplete
and its owned binding does not manufacture active status. A fresh public
installer instance then repairs the recorded project pin and reobserves exact
project identity. The complete scoped SDK journey passes (0.171s).

The final full Claude client suite, including strict project scope/path/load
errors, passes as uid 65534 with a short isolated temporary root; this preserves
both permission-denial and Unix-socket path-limit fixture behavior. Exact
changed-code lint and full-file size/architecture gates pass for the adapter,
domain, providers and installer packages, and both gates pass for the root native
fixtures. No new suppression rules were introduced. Genuine hosted project
listing/discovery results and both independent final adapter reviews remain
separate pending gates.

## Independent R1 correction: exact native listing spelling

R1 identified that cleaning a native-reported project path before comparison
could admit `alias/../artifact` even when resolving a symlink would reach a
different project's artifact. Project listing now requires literal installPath
equality and exact optional projectPath spelling; legacy user parsing retains
its prior normalization contract. Regressions cover both nonclean fields and a
real symlink traversal whose lexical clean path names A but physical resolution
reaches B. The authored native gate also refuses nonclean reported paths.
Focused client/provider/public-SDK contracts pass (0.007s/0.061s/0.253s), and
changed-code plus full-file client lint gates report zero issues.

## Final independent implementation review closure

The final adapter checkpoint `32dd485e789b57c7825c6c4fb2128263901be78f`
received independent R1 and R2 **GO** decisions. Both reviewers independently
rechecked the exact native path correction and the composed client, provider
and public SDK contracts; their reports are committed beside this evidence.
The R1 reviewer also ran the native runner's complete Python safety/evidence
suite (56 PASS), while R2 reran the selected native matrix contracts (17 PASS).
These implementation reviews close the earlier pending adapter-review gate.
Genuine hosted project observations remain pending and must be reported apart
from fixture execution, authored gate code and historical user evidence.

## Genuine project failure and diagnostic-only checkpoint

The actual macOS arm64 Claude2.1.263 hosted run40011624952 failed project add
with unrecognized native plugin-list output; user suites passed independently.
Artifact12528049546 ZIP SHA256 is
`c45098c244379dd4e0b62f2dfb7e235b415959428e19cd235cf083652fb5caf7`.
The project prefix has installer failure/version records but no raw list,
because add failed before its later read-only probe. Project qualification is
therefore failed/pending, not implied by prior fixture or user-suite success.

The supplemental diagnostic plan423d912f and both independent plan GO reports
were committed before source atc53e030e. This test-only change captures at most
two fixed A/B `plugin list --json` commands after the first installer error,
with the same verified binary and fresh explicit original profile. Each has
live1MiB-per-stream caps/cancellation,10s context deadline, empty stdin, no shell
and250ms WaitDelay; raw separate streams plus bounded exit/error/hash metadata
remain artifact evidence. The original installer error and failed stage remain
unchanged. Production parser, client success criteria, opt-in/hosted guard,
acquisition and successful lifecycle paths are unchanged. No diagnostic output
can become an installed/active result.

Two inert diagnostic contracts PASS0.007s without a Claude process; exact
changed-code lint reports zero issues after routine naming/preallocation fixes.
Genuine hosted replay and independent raw-output interpretation are required
before proposing any parser correction. The direct-child timeout/pipe bounds
are not OS isolation or general descendant sandboxing, and no model/trust/MCP
initialization or project success is claimed.
