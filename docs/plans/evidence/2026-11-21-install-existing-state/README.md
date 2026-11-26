# Existing-state plan: executable evidence and review closure

Research date: 2026-11-21. Product baseline: `5f63d48bf4461600e6bb916a6e4997f9830545e1` (fresh `main` when the planning worktree was created; checked again before publication).
Toolchain: Go 1.26.11, Linux amd64. No production source or dependency changes.
The [selected plan](../../2026-11-21-install-existing-state-plan.md) specifies future work; these checks establish current behavior only.

## What ran

The final replay result is recorded in [results.txt](results.txt), a compact, filtered report of Go JSON test events. It includes commands, test names, exit codes, RUN/PASS/FAIL/SKIP counts and identity/parser evidence. Parent tests and subtests are counted separately; top-level counts are also reported. Raw JSON/stderr are generated locally by every replay; the repository does not duplicate their large state dumps.

Final result: **43 top-level tests passed; 95 RUN and 95 PASS events including nested subtests; 0 FAIL, 0 SKIP; both commands exited 0.** CLI contributes 9 top-level / 30 total events; reliability contributes 34 top-level / 65 total events. This is a bounded selection, not the whole test suite.

| Suite | What it exercises | Evidence boundary |
| --- | --- | --- |
| CLI state spikes | Imperative install → remove bindings → saved add; source identities, pins, retained data; root requirements; unrelated unmanaged source; owned no-save refusal | Real Cobra commands, local Git commits/acquisition, installer transactions and files; existing Gemini native-response/scanner/helper fixtures and `directGitTestRunner` |
| CLI UX | Public target spellings, repeated target flag behavior, explicit project root; existing selection/cancel/deferred-host tests | The new parser spike parses argv only: it does not execute remove, validate its full command lifecycle, or prove shell quoting |
| Existing reliability tests | Intent ordering, publication failure, rechecks, binding-local continue versus unsafe stop, ownership/receipts/recovery and lifecycle facts | Existing unit/fixture suites, not cross-OS/native qualification |

The research fixture uses real disposable repositories through the existing `directGitTestRunner`, defined in `source_directory_test.go` and also used by `deps_identity_e2e_test.go`. It does **not** qualify production OSRunner containment. No refusal or production guard was weakened.

## Findings that changed the plan

### 1. Remove-all + exact same Git pin is not a demonstrated migration

For the executed legacy `github:acme/demo@SHA` route:

| Field | Imperative retained installation | Saved resolver candidate |
| --- | --- | --- |
| SourceKey | `github:acme/demo:` | Same |
| Repository, subpath, SourceKind | Legacy Git coordinates | Same |
| Commit and TreeDigest | Actual installed values | Equal in the same-pin case |
| SourceBindingHint | `direct-github-full-sha` | `uap-dependency-git` |
| Computed SourceBindingID | Direct-source identity | Different identity |
| Saved add result | — | `ErrUpdateRequired` with the specific sticky-source refusal |

The spike independently reacquires the direct source, verifies it matches installed identity/digest, and gets the saved candidate through `resolveSavedAddContext` and production `identityFromLocked`. It then asserts both the typed error and its sticky-source detail. The first generic headline alone would not establish the cause.

Three cases run: exact same commit/bytes, a new empty commit with identical bytes, and a new commit with changed bytes. All hit sticky SourceBindingID refusal. The changed-byte case therefore **does not independently exercise the digest guard**. Directory and other source routes were not exercised.

Remove preserves the PLUGIN_DATA sentinel. Each subsequent refusal preserves manifest/lock/state/sentinel bytes, mode and modification time, plus the full loaded state. This does not claim equality of every file in the native client tree.

Production explanation: [direct acquisition](../../../../install/integrationctl/agentplugins/adapters/sourceacquisition/acquirer.go), [saved identity](../../../../cli/internal/agentpluginscli/deps_source.go), [saved staging](../../../../cli/internal/agentpluginscli/deps_staging.go), [binding-ID normalization](../../../../install/integrationctl/agentplugins/domain/identity.go), [sticky guard](../../../../install/integrationctl/agentplugins/usecase/apply.go), [retained-install ownership exception](../../../../install/integrationctl/agentplugins/transaction/environment.go).

**Decision:** preserve the refusal and explain the missing migration path. Do not sell remove+add, data purge, or no-save as automatic adoption.

### 2. Ownership is scoped to the desired source; no-save has real limits

- A live unmanaged binding of the same source rejects saved add without changing manifest/lock/state.
- A root declaring UAP requirements rejects `--no-save`; installer state remains unchanged.
- An unrelated unmanaged source with distinct package/MCP/skill identities coexists with a successful public `deps lock` + `deps sync --locked --yes`. The original installation still exists and is exactly unchanged; the selected package has the new environment receipt.
- `--no-save` over that saved source is explicitly refused; manifest/state remain unchanged.

**Decision:** do not block every saved sync just because some unmanaged package exists; do not offer no-save when it would lose dependency or ownership semantics.

### 3. Runnable guidance needs command and public-target semantics

The real Cobra parser confirms:

| Input | Canonical selection |
| --- | --- |
| `--target codex,chatgptweb` | Codex + ChatGPT Web |
| `--target chatgpt` | Desktop/Codex, not Web |
| `--target codex --target cursor` | Cursor only |
| `--target codex,cursor` | Both |

An argv project root containing spaces remains exact. This is parser evidence, not a shell-quoting test. The plan preserves the existing cross-shell safe-argument gate: when a runnable command cannot safely represent the exact source/root, show those values and the next step as text. Remove/info selectors and add SOURCE are different contracts; unknown targets are never inferred from conflicts.

### 4. Publication is intentionally not an all-or-nothing install

Existing tests exercise manifest-first publication and independent binding outcomes. A lock write failure can leave saved manifest intent; an unsafe failure stops subsequent actions. The plan keeps these semantics and clarifies the output/retry scope.

Static inspection also found that `Result.Published = before.Check(files) != nil` includes unreadable preimages, not just observed changes. A true boolean cannot independently prove durable write. This read-error nuance was found by code review; no new failure-injection test for it is claimed here. See [application](../../../../install/integrationctl/agentplugins/environmentapp/application.go) and [intent](../../../../install/integrationctl/agentplugins/environmentapp/intent.go).

## Honest limits and unsuccessful attempts

The initial production OSRunner acquisition attempt failed before the retained-state scenario: `phase=init class=ordinary_unsupported exit_status=unavailable`. Three broader existing UX tests hit the same host limitation: `TestSavedAddDeferredHostQualifiesOnlyAfterSelection`, `TestSavedAddDeferredOwnedHostOfflineNoOpDoesNotProbe`, and `TestSavedAddPromptsForTargetsInTerminal`. Those runs failed; they are **not** included among passing final selections and are not product PASS evidence.

An early hypothesis expected same-digest migration to work; the actual sticky-source error disproved it. An initial unrelated-package fixture reused native MCP/skill names and correctly collided; the final fixture gives the two sources distinct identities. These were research corrections, not product changes.

The first combined replay failed to build the large CLI test package (`compile: signal: killed`); no CLI test ran. The next replay used `GOGC=25 GOMEMLIMIT=768MiB` to reduce compilation memory pressure. Compiler failure is not a passed test. The report records the final settings and results.

No claims are made for real native-client install/uninstall, Windows/macOS, production containment, Directory migration, future UX features, or a full `make test-core`/`make lint` run. These remain relevant gates for the later Go implementation.

## Reproduce

Requires an existing Go toolchain compatible with the baseline, Git, Python 3.9+, and populated Go module cache. The baseline commit must exist locally. The script never fetches a toolchain/dependencies, changes product code, or writes into the caller's active worktree.

From the repository root, choose a new output directory outside the checkout:

```bash
GOGC=25 GOMEMLIMIT=768MiB python3 \
  docs/plans/evidence/2026-11-21-install-existing-state/replay.py \
  --go /absolute/path/to/go \
  --output /absolute/path/to/new-output-directory
```

If needed, set `GOMODCACHE` and `GOCACHE` to existing caches. [replay.py](replay.py) creates its own detached worktree at the pinned baseline, copies only [state](plan_state_spike_test.go.txt) and [parser](plan_ux_spike_test.go.txt) research sources into the existing test package, runs the bounded selections offline, records raw events/stderr and a compact report, and removes only its private worktree. The `.go.txt` extension keeps these research sources outside normal package discovery.

Missing research sources, missing expected spike pass events, nonzero exit, empty selections, failures or skips reject the replay. Assertions pin behavior; temporary Git commits, installation IDs, paths and event timestamps vary between runs.

## Critique rounds and closure

| Round | Review | Decisions / corrections |
| --- | --- | --- |
| 1 | Independent state/ownership, UX/automation, architecture/scope reviews against fresh main; ToolHive comparison retained at its pinned commit | Separate diagnosis from migration; same-source versus unrelated ownership; no implicit no-save; source-before-target caveat; narrow retry; shared predicate and existing inspection boundary |
| 2 | The three reviewers examined the selected specification after executed spikes | Bound Git evidence to the actual legacy route; conservative Published semantics; SOURCE versus UUID; public targets; safe command fallback; existing envelope plus narrowly typed conflicts |
| 3 | Final plan/evidence closure and reproducibility review | Verify no production diff, preserve native/OSRunner limits, strengthen the unrelated-install assertion to require its continued presence, and require the exact research sources/pass events in replay |

The plan's future acceptance cases are requirements, not a claim of implemented functionality. The research resolves the critical design questions without adding a second installer, resolver, state model or adoption subsystem.
