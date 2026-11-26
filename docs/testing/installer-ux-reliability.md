# Installer outcomes and console qualification

This guide describes the source-built installer qualification added with
the [implementation plan](../plans/2026-11-26-installer-ux-reliability.md).
It records how to exercise the behavior; the PR's actual checks and retained
reports determine whether a particular revision passed.

## Observable contracts

Every reviewed binding has an outcome. A planned no-op is only `unchanged`
after the coordinator observes it. A pre-publication refusal leaves the action
`not_attempted` with its classified cause. A successful peer keeps its observed
outcome when another agent fails. Native mutation or a completed native call
does not turn a failed installer transaction into success.

Human output explains the affected agent and next step even when recording is
disabled or the log folder is unwritable. A failed write is distinguished from
disabled recording. The fallback uses fixed classifications and never publishes
arbitrary provider errors, log-write paths or response bodies. Original typed
errors remain available through `errors.Is` and `errors.As`. JSON/native facts
retain their established content; the human view is a separate projection.

An incomplete result write ends the interactive attempt without starting a
retry. Recovery and unconfirmed process cleanup continue to stop every retry
in the affected namespace. Neither an unavailable log nor a broken terminal
changes an already committed installation into an uncommitted one.

Declared `install --format json` returns one document across recorded-pin repair
and the subsequent full convergence. Its existing action/result arrays retain
the completed repairs as well as later work. A failure while preparing the later
phase still reports those committed repair effects and the overall failure.
A partially written result is never followed by a second JSON document.

### Console behavior

| Situation | Expected behavior |
| --- | --- |
| `--plain` with no explicit browser request | Numeric terminal selection; no automatic browser opener |
| Explicit `--plain --ui` | Browser selection when an interaction host is available |
| Stdout redirected, stderr usable | Selection, full review and confirmation share the visible interaction stream |
| Both outputs redirected, input still a terminal | Required interaction refuses before reading hidden input |
| Explicit selections and target with `--yes` | No confirmation input is acquired |
| Explicit selections and target with `--dry-run` | Plan remains on stdout and no effects occur |
| No eligible detected target | Reasons are shown; no empty selector or apply is entered |
| Short or failed authority-review write | No confirmation or apply follows that incomplete review |
| Rich review exceeds the screen budget | Complete review is written before a fresh fallback confirmation |

Numeric catalogs show 50 entries per page. `next`/`prev` browse the current
view; `/query` searches names, paths and descriptions; `/` clears the query.
Numbers toggle only displayed entries. `selected` clears the current query and
shows the whole retained basket; `all` returns to the catalog. `clear` removes
the basket, `done` reviews it, and `q`, an empty line, EOF or cancellation never
authorize installation. Canonical candidate IDs survive every view change.

Navigation labels retain both the beginning and distinguishing path suffix.
The authority review retains complete sanitized source and destination paths,
including source moves. Unavailable catalog entries have bounded explanations;
an incomplete discovery result does not pretend to be a complete catalog.

### Browser ownership

Terminal takeover closes an unused browser session to new admission under the
existing server mutex. A scan or unaccepted review can be discarded. A running
preparation or any accepted operation retains browser ownership, including
after that operation completes. Closing waits outside the mutex.

The opener has its own bounded context. An opener timeout, failed status write
or failed fallback prompt cannot cancel accepted browser work. Explicit command
cancellation still cancels the session. The local capability URL is not an
installation result or a provider diagnostic.

## Build the actual qualification binary

Use the repository's pinned Go toolchain and Node version from CI. Browser
routing must be tested with production assets: the development placeholder
reports `Bundled()==false` and can hide a routing defect.

From a clean checkout at the revision under test:

```bash
uap_ux_dir="$(mktemp -d)"
npm --prefix ui/skills-picker ci
npm --prefix ui/skills-picker run bundle:go
go build -o "$uap_ux_dir/agentplugins.exe" ./cli/cmd/agentplugins
python -B -m unittest discover -s scripts/terminal-ui -p 'test_skills*.py'
```

The bundle command generates the embedded assets. Do not edit or commit the
generated bundle by hand. The evidence records the harness working-tree state
as well as the committed source revision and actual binary digest.

Provision the existing pinned scanner for the runner's platform. For Linux
amd64, the invocation is:

```bash
python scripts/terminal-ui/provision_scanner.py \
  --target linux-amd64 --output "$uap_ux_dir/scanner"
python scripts/terminal-ui/skills_matrix.py \
  --binary "$uap_ux_dir/agentplugins.exe" \
  --product-sha "$(git rev-parse HEAD)" \
  --build-go "$(command -v go)" \
  --scanner-archive "$uap_ux_dir/scanner/lintai-v0.1.3-x86_64-unknown-linux-gnu.tar.gz" \
  --artifacts "$uap_ux_dir/evidence" \
  --suite all --timeout 20 --acquisition-timeout 90
```

Use the existing platform mapping in `terminal-ui.yml` for macOS and Windows.
The artifacts directory must be new. `--case` selects a named diagnostic case;
an explicitly requested unsupported case fails instead of being omitted.
`--suite lifecycle` and `--suite browser` are useful during development; merge
qualification runs the declared complete platform lane with `--suite all`.

Acquisition has its own deadline. A network fetch does not consume the shorter
interaction deadline after a review or menu becomes ready. Missing sources,
scanners, native prerequisites and forced cleanup are failures, not passes.

## Sources and process boundaries

Linux lifecycle cases use a disposable HTTPS publisher with a freshly generated
trusted CA supplied through standard `SSL_CERT_FILE`. TLS verification stays
enabled. The fixture serves a complete index, `SKILL.md` and its adjacent
reference. Evidence compares delivered bytes with the original resource bytes.

macOS and Windows use the same pinned, inert public GitHub Skill:

```text
777genius/universal-agent-plugins@ce789138a393bffd7e6d001c0e4f8aec05e47773
--path tests/fixtures/uap_apm/skills/benchmark-note
```

The expected `SKILL.md` Git blob is
`1341ca3c10621a9e5e25304cca19144c6b54fa3b`. The full commit belongs in the source
selector; `--ref` has different branch/tag semantics. GitHub acquisition keeps
its normal isolated Git policy, without ambient credentials or URL rewriting.
Catalog and browser-routing cases use the pinned repository without an explicit
path that would bypass the routing decision.

The compiled native responder implements a small, explicit Codex protocol
surface in a private profile. It exercises installation, inventory and cached
Skill observation without credentials, model calls or real vendor accounts.
Its results qualify installer behavior; the existing genuine native-client
workflows retain responsibility for vendor acceptance.

| Evidence | Linux | macOS | Windows |
| --- | --- | --- | --- |
| Fresh, unchanged, partial, retry, unavailable log | Actual CLI and native protocol fixture | Same | Same |
| Decline, Ctrl+C and EOF before apply | Same-descriptor PTY restoration | Same-descriptor PTY restoration | Native ConPTY owner and restoration |
| Redirected review/catalog and both outputs redirected | Actual descriptor routing | Actual descriptor routing | Covered by the separate native-console contracts, not claimed as Unix descriptor evidence |
| Plain and explicit browser mode | Production bundled CLI | Production bundled CLI | Production bundled CLI |
| Interrupted operation refusal | Real journal and identity-bound Linux pidfd cleanup | Existing transaction crash/recovery gate | Existing transaction crash/recovery gate |
| 2,000-candidate navigation and failed review writes | Composed production command/renderer tests | Same tests in native CI | Same tests in native CI |

The Linux crash case waits for an actual native side effect and real installer
journal. It validates a nonce, executable path and parent identity, opens a
stable pidfd before terminating the owned processes, and verifies exit before
attempting another installation. A bare PID or process-group assumption is not
accepted as ownership evidence.

## Inspect evidence rather than only an exit code

Each case retains its command, original stdout/stderr or terminal capture,
source metadata, native calls and durable state. The top-level report separates
the product build revision, harness revision, pinned GitHub fixture revision
and actual CLI SHA-256. It also identifies the resolved native responder binary,
its SHA-256, whether it was built or supplied, and the embedded responder source
digest. Both input binaries must remain unchanged throughout each successful
case. Per-case source records identify the source actually used and publisher
resource hashes where applicable.

The success oracle checks selected targets, delivered bytes, environment owner,
immutable lock, committed receipt authority and native discovery. Codex must be
verified active rather than merely prepared. A repeated install must have no
native mutation and no durable rewrite. Retrying a partial failure must retain
the complete healthy peer binding and receipts.

The failure-log oracle requires the versioned Skills run header and a matching
failed binding with a code and summary. A nonempty file, header-only file,
success-only log or another binding's failure cannot qualify the case.

The parser rejects multiple JSON envelopes and disagreement between JSON and
process exit. Pre-apply acquisition cancellation keeps its existing output
contract; these tests do not create a new unconditional JSON failure envelope.

## Required gates and review

Run `make lint` and `make test-core`, plus the compatibility, polyglot,
package-manager and affected native/rollback gates in `CONTRIBUTING.md`.
The Skills picker workflow adds race coverage for loopback admission and the
CLI launcher/session lifetime. Terminal UI adds one production-bundled Skills
job for each of Linux, macOS and Windows without replacing its existing matrix.

Draft-skipped workflows are not evidence. Use the normal Ready transition for
the complete CI lane. Review actual final-revision results and independent code
findings, then verify the merged revision. Environment limitations belong in
the record: do not disable containment, skip a declared case or relabel a
synthetic native responder as genuine vendor qualification to obtain a pass.
