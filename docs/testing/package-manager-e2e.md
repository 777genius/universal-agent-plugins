# Portable package-manager E2E

Run the same package-manager acceptance suite on Linux, Windows and macOS:

```sh
python scripts/package-manager-e2e.py
```

Use Python 3.10 or newer, Git, and Go with the toolchain specified by this
repository. On macOS/Linux, use `python3` if that is the name of your Python 3
interpreter. Run from an up-to-date checkout. The command uses the local source,
so finish edits before running it; a source change during execution fails the
result. No globally installed `agentplugins` executable is used.

The runner builds the actual CLI, executes the selected Go scenarios, and prints
the path to `report.json` and raw logs. It creates disposable user, client,
configuration and temporary directories. The user's installed plugins, agent
accounts and Git hooks are not used. Existing Go build/module caches are reused;
the first run may need to download Go dependencies. Fixture package downloads
use local HTTPS servers. The unchanged-executable installation phase also needs
public network access: production security checks its configured feed and
downloads the pinned LintAI scanner into a fresh disposable profile. It uses no
fabricated scan result or security bypass. A failed download/scan fails the run;
independent composed scenarios still execute and retain their results.

To choose a persistent evidence directory outside the checkout:

```sh
python scripts/package-manager-e2e.py --output /path/to/new-e2e-results
```

The output directory must be new. On Windows, an ordinary absolute Windows path
works with the same option. Results contain source commit/tree, dirty-source
status, a hash of working changes, platform/tool versions, exact command
arguments, the built executable's SHA-256, required cases, completed cases,
durations, inventory hashes and log hashes. Schema version 2 records every
required Go scenario both in its phase and in the aggregate inventory, including
the production installation/composition phases. Keep `report.json` together with the `.stdout` and
`.stderr` files when sharing a result. `status: pass` is accepted only after the
selected cases and package completion are observed with no skips.

## What runs

The shared case inventory is
[`package_manager_e2e_cases.json`](../../cli/internal/agentpluginscli/testdata/package_manager_e2e_cases.json).
The separate production-composed installation inventory is
[`package_manager_install_cases.json`](../../cli/cmd/agentplugins/testdata/package_manager_install_cases.json).
Go verifies that every listed case has an implementation, and the Python runner
requires an actual terminal pass for every selected case. An empty test selection,
renamed/missing case, skipped test, failed command, truncated JSON or timeout
cannot produce a passing result. The `uappm_e2e` test build tag keeps this catalog
from duplicating existing scenarios during ordinary `go test ./...` runs.

| Group | Coverage |
| --- | --- |
| Archive graphs | A chain of 32 packages; one root with 64 children; a 19-node, 78-edge shared DAG; self and three-node cycles; ordering determinism; direct/transitive ownership; shared removal; target inheritance |
| Archive replay and refusal | Real HTTPS ZIP acquisition, exact installed payloads, cold restore from the two agreement files, zero-request warm replay, changed remote bytes, missing sources, damaged store/lock and preview/cancellation boundaries |
| Registry and Git | Compatible and incompatible ranges, forced candidate backtracking, discarded-branch absence, integrity error without fallback, ineligible candidates, partial commit and locked retry, mixed-source target union and cold restore |
| Agreement | Manifest/version intent, transfer and rollback, confirmation-time changes, external lock/manifest edits, missing versus empty files, unchanged-lock store hydration and read-only resolution |
| Graph evolution | Replacing dependency edges; pruning unreachable nodes while retaining shared nodes; failed new-leaf acquisition and retry; cascading removal and strict sync with user drift; two actual competing lock commands; selective missing-cache hydration |
| Adversarial installed state | Foreign/native edits, hostile otherwise-valid ZIPs on fresh install and update, damaged cache metadata, corrupt active state, cross-source component collisions, stale and obstructed lock files |
| Transport breadth | Existing ZIP, TAR, TAR.GZ, TGZ and extensionless installation/update/cold replay, strengthened with independently expected resource/skill bytes and complete normalized metadata values; nested literal-plus package selection on first and cold install |
| Production installation catalog | Nine new real ZIP journeys through the main application composer: unsaved install/repeat/remove; two-target ownership; component replacement; foreign skills and disabled MCP collisions; malformed native configuration; exact-pin repair; preview/decline; rich cold transfer and user/project isolation |
| Existing production composition | Gemini ZIP user/project isolation, update, repair, retained data, removal and cold agreement transfer; Cursor ZIP lifecycle with its distinct native configuration and ownership layout |
| Executable smoke | The newly built executable: help, manifest initialization, missing-lock refusal, malformed-package refusal and an empty-environment lock/install/status lifecycle |
| Unchanged executable installation | Five actual subprocess commands: nonempty local-plugin installation with the real cold production scanner, repeat/cache/no mutation, read-only info, missing-native-skill repair and owned removal; exact source/managed/native/foreign byte checks |

Each graph checks expected source identities, nodes, dependency edges, target
assignments and distinct installed file contents. A lock that agrees with an
equally incorrect state is insufficient. The large fixtures check correctness
and bounded completion; they are not a throughput benchmark or a promise about
arbitrary graph size.

### Evidence boundaries

The composed scenarios use real CLI parsing, acquisition, package loading,
immutable storage, resolution and installer transactions. Test security
assessments, verified Directory inputs and native agent responses are explicit
fixtures. The production-composed ZIP journeys exercise the application wiring
with fixture detection/security. New rich installations target Gemini and Cursor;
they validate projected skills, remote MCP headers, stdio launch configuration,
scoped persistent data and truthful Prepared/Active facts without launching MCP.

The unchanged executable has separate smoke and successful nonempty installation
phases. The latter uses normal Gemini directory detection and the actual pinned
scanner, records its executable hash and assessment identity, and verifies exact
installed skill bytes. Native executables are absent from its disposable PATH.
Its local `--no-save` source proves imperative installation, not saved archive
dependency resolution. Saved source/graph behavior is qualified by the composed
scenarios. Skill-only binary removal checks the reported retained-data outcome;
actual allocated data continuity is asserted in the rich composed journeys.

None of these results certifies native-agent discovery/reload, execution of a
configured MCP server, authentication, or live model behavior. Production scanner
execution is proved only by the dedicated unchanged-executable phase; fixture
security responses cannot supply that evidence.

The suite preserves UAP's current dependency rules:

- One selected version per canonical source within an environment; compatible
  cycles represent co-installation, not service startup order.
- Compatible existing pins are retained. Fresh Registry resolution uses the
  existing minimum-compatible stable preference and backtracking.
- `--locked` forbids choosing a new solution. It can download recorded immutable
  bytes for a cold restore; it is not an offline mode.
- Existing malformed/stale agreements fail closed. Errors before installation
  preserve active effects. An activation error after a committed binding reports
  partial state and can resume from the same pins.
- Verified cache objects can remain after a later publication failure. The suite
  does not assume global rollback across every native target.
- `install` can converge/repair an existing managed projection. The strict drift
  refusal journey uses `deps sync --locked`; those commands are not conflated.
- Cross-source MCP name collisions can be discovered during native activation,
  after a new binding and desired agreement have committed. The suite requires
  preserved earlier ownership, truthful partial/failed state, stable pinned retry
  and clean removal of the failed addition. Earlier conflict detection remains
  a useful improvement; passing this case is not a global-rollback claim.

The archive graph harness deliberately leaves optional native observation
disabled, exercising the supported installer composition where adapters enforce
ownership at activation/removal. The late-collision observation belongs to that
composition. Production-composed journeys separately exercise the application's
preflight wiring; the suite does not claim every CLI path detects conflicts at
the same stage.

The separate saved Registry update program in [#563](https://github.com/777genius/universal-agent-plugins/pull/563)
owns its additional version-selection changes. This catalog does not redefine
that policy or add support for side-by-side versions.

## Restricted Linux environments

Production Git acquisition requires process-tree observation from the operating
system. Some containers do not expose the required procfs interfaces. The full
suite must fail on such a host; it must not replace the process runner, disable a
guard or convert the scenario into a skip.

To execute the explicit archive-safe subset there (archive, agreement, transport,
evolution, adversarial, both production-composed catalogs and the binary phases):

```sh
python scripts/package-manager-e2e.py --suite archive
```

This selection still requires Git for repository metadata and existing fixture
setup, but it does not invoke production Git acquisition. It also requires
permission to bind a local HTTPS server and to download/run the production
scanner. The report lists the excluded Registry
cases and keeps `full_qualification: false`, even when the subset passes. A full
run on a supported CI host is separate evidence for that host.

## CI and local macOS execution

[`Package manager E2E`](../../.github/workflows/package-manager-e2e.yml) runs the
full command on hosted Ubuntu 24.04, Windows 2025, macOS 15 on Apple Silicon
(`macos-15`) and macOS 15 on Intel (`macos-15-intel`) when relevant source/test files change.
It also supports manual dispatch. Superseded PR runs cancel automatically; jobs
have a generous hang limit, retain reports and logs, and need no account secrets.

The two macOS jobs exercise the same suite and assertions. Their runner labels
select an OS generation and architecture; GitHub updates the underlying images.
For an additional local macOS run, check out the reviewed PR branch and run:

```sh
git fetch origin test/package-manager-conformance-20261030
git switch --track origin/test/package-manager-conformance-20261030
python3 scripts/package-manager-e2e.py
```

If the branch already exists locally, switch to it and update it normally before
running the command. Success is exit code 0 with `suite: full`, `status: pass`,
`full_qualification: true` and `source_unchanged_during_run: true` in the report.
The reported platform must be Darwin. A Linux or Windows result cannot substitute
for this macOS execution.

## Adding a scenario

Add an ordinary Go case to the appropriate tagged `package_manager_*_test.go`
file, register its ID in that group's map and in the shared JSON inventory, and
write assertions for the actual expected graph/files. Reuse the TLS archive
fixture and existing transaction helpers. Keep expected outcomes independent of
the resolver output. Add a default regression test outside the tag when fixing a
production defect that should remain covered in the ordinary test lane.

For a production-composed installation journey, add its implementation and ID to
the installation catalog instead. New groups require explicit runner
classification so a future native/Git-only group cannot silently enter the
restricted-host archive subset. Timeout, malformed/incomplete evidence, skips
and missing required cases remain failures on every platform.

For a focused developer rerun, use Go's existing subtest selector, for example:

```sh
go test -tags=uappm_e2e -count=1 -timeout=12m ./cli/internal/agentpluginscli -run '^TestPackageManagerE2E/archive/self-cycle$'
```

The Python command remains the qualification entry point because it provides
environment isolation, complete-case verification and retained evidence.

Research decisions and independent plan reviews are recorded in
[`package-manager-e2e-20261030.md`](../plans/package-manager-e2e-20261030.md).
