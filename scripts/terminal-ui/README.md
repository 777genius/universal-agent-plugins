# Terminal UI evidence harness

This standalone Python 3.10+ stdlib harness drives a supplied **native** CLI
through a real Unix controlling PTY. It does not import production code, install
Python dependencies, download binaries, run real agents, or modify the source
tree. Run on a disposable Linux/macOS machine with no installed desktop agents:
the production detector also inspects system application directories. Synthetic
Codex and Cursor must be the only compatible choices. Unknown ambient choices
must be investigated, not accepted by updating expected targets.

```sh
# From repository root; Go 1.25.13 or the integration's required toolchain.
(cd cli/plugin-kit-ai && GOTOOLCHAIN=local \
  GOMODCACHE=/tmp/uap-go-modcache GOCACHE=/tmp/uap-go-buildcache \
  /tmp/uap-go-toolchain/go/bin/go build -o /tmp/agentplugins-ui ./cmd/agentplugins)
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s scripts/terminal-ui -p 'test_*.py'
python3 scripts/terminal-ui/harness.py --binary /tmp/agentplugins-ui \
  --case detection --case baseline-lifecycle --artifacts /tmp/pty-baseline
# Separate invocation against the integrated native binary; no baseline cases.
python3 scripts/terminal-ui/harness.py --binary /tmp/agentplugins-integrated \
  --artifacts /tmp/pty-integrated
```

Artifact directories must be new. Repeat a **specific** case only after a relevant
fix, using another directory. Failures are real nonzero exits, never expected-pass
wrappers or mutation retries. `--timeout 12` bounds each semantic wait and process
exit. `--selection` and `--confirmation` accept regexes for actual prompt wording;
changing these must not substitute an earlier progress message for form readiness.
Default confirmation matches install/apply/proceed questions or “Confirm
installation”. If integration uses different text, pass its precise question.

Each case receives a fresh project, Unicode HOME, USERPROFILE, XDG roots, client
config roots, AGENTPLUGINS_HOME, temp directory, and PATH containing only two
version-only shell stubs. Parent environment/config/credentials are not read or
copied. The package is a minimal standard `plugin.json`, without remote URLs,
hooks or executable components. Every fixture is deleted after evidence is saved.
No real client subprocess can resolve through PATH; attempts to invoke the stubs
for anything other than `--version` fail and appear in `stub.log`.

The default scanner is a labeled **synthetic** empty-findings protocol stub,
seeded only into the fixture's existing Lintai cache. It is UI-only evidence.
To exercise a real scanner, supply the pinned archive (no download occurs):

```sh
python3 scripts/terminal-ui/harness.py --binary /tmp/frozen-agentplugins \
  --scanner-archive /tmp/lintai-v0.1.3-x86_64-unknown-linux-gnu.tar.gz \
  --artifacts /tmp/pty-real-scanner
```

The archive SHA256 must equal
`2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da`.
Only its regular `lintai` member is copied; archive paths are never extracted.
Alternatively use `--scanner-binary PATH --scanner-sha256 TRUSTED_DIGEST` for a
native executable from a separately verified source. The frozen scanner and both
archive/executable hashes are saved in evidence. Real scanner runs still use only
the synthetic package; they are not general scanner/security release proof.
Dead proxies and local fixtures are not firewall-level network attestation.
Linux here means glibc, not musl. Freeze a private copy of a changing CLI build
output before invoking the Unix harness; its hash is recorded in `results.json`.

## Cases and evidence

| Cases | Required behavior |
| --- | --- |
| `detection`, `baseline-lifecycle` (explicit only) | Existing line selector discovers Codex/Cursor; dry-run leaves no mutations; selecting Cursor materializes its package, preserves manual activation after No, and restores terminal. These are baseline fixture proofs, not new consent acceptance. |
| `default-no`, `no` | All selected; full visible plan before default Enter/explicit No; exit 0 and unchanged state/client/project/journals. |
| `yes-lifecycle` | Space deselects Codex, arrow moves to Cursor, Enter submits; preflight identity/version visible before Yes; exactly Cursor materializes; plain activation No is consumed and auth is not asked. |
| `empty` | Space/arrows deselect both; Enter produces inline validation, never defaults back to all; Esc exits 1 without mutation. |
| `escape`, `ctrl-c`, `ctrl-d`, `confirm-*` | Cancellation at each form exits 1 without mutation and restores terminal. Raw Ctrl+D is a key, not Unix EOF. |
| `plain`, `dumb`, `term-unset` | Full selection/default-No path without escape sequences; same consent policy. |
| `no-color`, `NO_COLOR` | Same keyboard UI, no color SGR; cursor controls are allowed. |
| `plain-eof`, `plain-partial-eof` | Canonical VEOF at empty selection and partial `y` plus EOF at confirm fail closed; neither equals Enter/Yes. |
| `stdin-pipe` | Pipe stays open with no data; CLI exits with required-target error without reading it. |
| `json`, `json-tty`, `json-explicit` | No-target JSON fails promptly without input; all-three-TTY JSON emits no controls; separate redirected stdout parses as exactly one versioned JSON envelope; explicit-target dry-run succeeds. |
| `stdout-redirect`, `stderr-redirect`, `both-redirect` | One visible output gives Plain selection/No; both redirected fail without reading an invisible prompt. |
| `resize`, `tiny` | Change terminal dimensions during selection, then default No without mutation. |
| `queued`, `paste` | Consecutive Enters reach default No without hanging; bracketed multiline paste cannot silently grant consent, then Esc cancels. |

For every successful exit/cancellation check, the harness compares the complete
pre/post termios attributes, checks that the final cursor-hide is followed by
cursor-show, and runs a second line reader on the **same slave PTY** without
resetting it. Canonical line delivery and kernel echo must work. Forced cleanup
is recorded and never treated as terminal restoration proof. Cleanup kills the
CLI process group on timeout and resets only the harness-owned PTY afterward.
A dedicated session leader (`pty_owner.py`) stays alive throughout CLI exit,
termios inspection and the subsequent foreground probe. Only after evidence and
the cleanup reset does that owner exit. This avoids macOS revoking the terminal
when the CLI exits. The status socket is independent of terminal bytes; a probe
cannot replace the CLI's exit status. Nonce-based probe markers cannot be
satisfied by earlier CLI output. ENOTTY is an error, never a restoration pass.

The inspected Huh keymap uses Space/arrows and Enter for confirmation; `y`/`n`
are disabled. Yes uses Space then Enter; explicit No moves left then right and
submits. Normal rich cases wait for the rendered Yes/No/Enter controls, beyond
the preprinted question. The initially tiny viewport selects Plain under the
inspected size gate (width < 40 or height < 10), and must emit no ANSI. `queued` deliberately sends two Enters together and
requires completion without another key; its queued-input assertion is retained.

Each case saves `terminal.ansi` (raw synthetic capture), `transcript.txt`, semantic
`*.frame.txt` snapshots, `events.json` (offsets/timing/exit/restoration), and
`mutations.json` with before/after hashes and full synthetic state. The summary
records native OS, Python version, binary SHA256 and failures. The frame renderer
supports common inline CSI and incremental UTF-8; it is not a full VT emulator,
CJK width oracle, screenshot assertion or terminal-injection sanitizer. Unknown
CSI is recorded. Inspect raw captures when rendering differs. Cursor checks
establish the emitted restoration protocol, not physical display hardware state.

Mutation assertions include client config trees, fresh project, installer state,
managed tree and operation journals. Scanner/acquisition caches are deliberately
excluded. Yes asserts exact persisted client IDs, materialized binding, a real
native `plugin.json` under the fixture, committed receipt, and activation/auth
remaining unconfirmed. It does not equate text “success” with persisted success.

Self-tests exercise the harness using synthetic subprocesses, including a broken
raw-mode child. They are **not CLI/Huh evidence**. Integrated failures against the
old base are expected gaps but remain failures. Baseline runs must never qualify
new `--plain`, default-No or Huh behavior.

## Automated native Windows (implementation; native execution pending)

`windows_conpty.py` uses Python stdlib ctypes and Windows 10 1809+ ConPTY APIs.
It creates pipes/HPCON, supplies an explicit isolated environment and Unicode cwd
through `STARTUPINFOEX`, and launches `windows_job.py` as a persistent console
owner. That owner runs the native CLI with inherited console handles. **TERM is
unset and no `--plain` flag is supplied**: numbered selection and `[y/N]` markers
must prove the proposed conservative Plain fallback actually ran.

On the GitHub Windows runner, stage Python, the candidate native Windows binary
and this verified archive, then execute (no workflow file outside this lane):

```powershell
python scripts/terminal-ui/windows_conpty.py `
  --binary C:\ui-inputs\agentplugins.exe `
  --scanner-archive C:\ui-inputs\lintai-v0.1.3-x86_64-pc-windows-msvc.zip `
  --artifacts "$env:RUNNER_TEMP\huh-conpty-evidence" --timeout 15
exit $LASTEXITCODE
```

Pinned Windows archive SHA256:
`2f61f6a83a160afa3feed9ea1722b82d0d938ebff865a4e20d39b5f55270c911`.
Origin: `https://github.com/777genius/lintai/releases/download/v0.1.3/`.
A raw executable also works with the required trusted `--scanner-sha256`.
The runner freezes its own CLI copy and records native OS/Python and hashes.
There are no Windows runtime stubs: discovery must find exactly the two synthetic
config-only profiles with an empty PATH. No real agent is installed or launched.

Default cases: default No, explicit No, Cursor Yes → activation No, selection and
confirmation Ctrl+C, Ctrl+Z/Enter EOF, and resize. Every case gets a new fixture.
The owner samples all three `GetConsoleMode` values and cursor visibility before
and after CLI exit, then reads a unique echoed canonical line on the same console.
Mode equality, cursor restoration, successful next read, echo, expected CLI exit,
mutation fences and owner exit are assertions. Conhost itself emits VT bytes, so
raw ANSI absence is not used to infer Plain mode. Raw captures, status, mode and
mutation evidence survive failure. The close path drains output concurrently,
bounds HPCON shutdown and thread joins, and records forced cleanup as failure.
Unsupported APIs, missed markers and EOF/cancel hangs fail the command; there is
no skip, simulated native pass, or automatic retry.

This implementation has only been syntax/CLI-argument checked on Linux. Actual
Windows execution remains required; cross-compilation and WSL do not qualify it.
Upload the complete artifact directory even when the runner exits nonzero.
A Windows EOF/control event may expose a product bug or a driver/platform
mismatch: inspect the saved transcript/status rather than changing it to a pass.

Remaining focused scope: npm inherited-stdio launch, external SIGTERM, failed
output, init/panic injection, single-discovered-client auto-selection, auth-Yes,
zero targets, control-sequence/CJK layout and other release architectures.

Review regression coverage includes LF (Ctrl+J) followed by Esc at both rich
forms, rejection of any preflight/confirmation advancement after bracketed
paste, queued confirmation-to-Plain lifecycle input, and external SIGTERM at
both forms. All require unchanged state on cancellation and terminal reuse.
`test_windows_cleanup.py` injects termination failure and the owner-exit race;
these are resource-ownership tests, not native Windows evidence.

The real npm launcher can be exercised without network release resolution:

```sh
python3 scripts/terminal-ui/harness.py --binary /absolute/frozen/agentplugins \
  --scanner-archive /absolute/pinned/lintai-archive.tar.gz \
  --npm-launcher --case escape --case json-tty --case json-explicit \
  --artifacts /tmp/new-npm-terminal-evidence --timeout 15
```

This copies the unchanged launcher/bootstrap into each disposable fixture and
uses the existing local-frozen-release-asset digest-verification seam. The
synthetic manifest is not release provenance; its historical evidence metadata
retains its original identity and claims. The native child retains
`stdio: inherit`; proof environment keys are removed by the real launcher.

Each Unix semantic checkpoint saves `.frame.txt` and `.frame.svg` alongside
`terminal.raw`. SVG files are rendered synthetic transcript images from the
harness's small Screen model, not native terminal-emulator screenshots; raw
captures and restoration/state checks remain authoritative.
