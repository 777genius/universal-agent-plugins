# Grok pinned hosted project-source research before adapter code

2026-10-23, codex_claude. The existing Grok project client plan and its two
independent plan GO results remain in force. This supplement addresses only
the plan's prerequisite to observe exact native project-directory identity
before freezing that delivery lane. No Grok adapter code has been added.

## Existing genuine evidence and missing fact

Official stable metadata pins **Grok Build 1.0.67**. The official versioned URL
https://x.ai/cli/grok-1.0.67-linux-x86_64.gz produced 76,217,006 compressed bytes,
SHA256 `6bbea96cde807c13e4c66d9583d8acdd155e446d93131c2820999cf5523ed5a5`,
and a 194,169,184-byte binary,
SHA256 `ce33b1a29e37901f56fe59916cda7a1149e6fd377118f918e90d89f9abb52a51`.
These are measured hashes of bytes from the official HTTPS versioned endpoint;
the captured vendor installer does not publish a separate SHA256. The installer
was read as metadata and never executed.

All genuine task-host version/plugin-help/plugin-list-help/inspect-help/MCP-help
probes succeeded in a networkless empty-profile namespace with empty /proc;
version output was `grok 1.0.67 (2fe064390d07)`. Actual list/inspect/discovery has
not run on this host. All logs/provenance are retained separately; the verified
reproducible binary copy was reclaimed after checking its recorded hash.

Official sources reread:
- https://docs.x.ai/build/features/skills-plugins-marketplaces
- https://docs.x.ai/build/features/mcp-servers

The docs identify `.grok/plugins`, separate user plugins and Claude-compatible
roots. Successful help confirms read-only list/inspect --json grammar. It does
not establish populated record spelling, exact project source, effective
precedence, shadowing/disablement, skill execution or bundled MCP behavior.
Those facts must not be guessed from the old user parser.

## Planned source and reuse

The separate Grok leaf starts from the reviewed Qoder research checkpoint
`febd633d`, so it can reuse that reviewed research implementation while keeping
client PRs separate and stacked. After two independent GO reviews of this
supplement, add `scripts/research-grok-native.py`, its inert tests and
`.github/workflows/grok-native-research.yml`. Mechanically move only Qoder's
already reviewed bounded-read and child output/deadline/process-group helpers
to small `scripts/native_client_research.py`; both research runners import the
same implementation. Preserve the existing Qoder CLI allowlist, pins, guard
ordering, environment, result validation and workflow. This is research utility
sharing, not another installer engine, native guard or project adapter.

Both runners continue importing the actual adjacent native matrix and call its
unchanged require_hosted('linux-amd64') before output/probe paths, acquisition
or execution. Grok's workflow uses only disposable GitHub-hosted ubuntu-24.04,
contents:read, exact source checkout with persist-credentials:false, explicit
native disposable opt-in and no secrets. It runs inert research tests first,
then captures research; always() uploads bounded evidence even on failure.

## Exact acquisition and runtime authority

Download only the exact Grok URL above with a 90s request timeout and actual
96MiB compressed cap. Check its measured exact compressed hash before gzip
parsing, then bound expanded bytes to 256MiB and require the exact binary hash
before writing one fixed 0700 executable. Never run the vendor installer.

Create a fresh temporary parent with separate binary/home/tmp and project-A,
project-B roots. Initialize each project as an independent empty Git repository
using a fixed system Git executable and explicit fresh-profile
environment, so ancestor discovery stops at its selected project. The Git
operation is only local `git init --quiet` inside these two new roots; it has
no network, source acquisition, remote, hook or authentication action.

Use the existing profile_environment helper, plus literal PATH=/usr/bin:/bin,
TMPDIR and LANG. No ambient HOME/profile, XDG state, workflow token, proxy,
credentials, keys, native trust or permission variables reach Grok/Git.
Native processes receive DEVNULL stdin, explicit selected project cwd,
30s deadlines, child-enforced 1MiB stdout/stderr file caps and process-group
cleanup, exactly as in the reviewed Qoder helper. The collector has an independent hard 8min total wall-clock budget covering
acquisition, fixture preparation, all processes and inventory work. A Linux
main-thread interval timer raises a dedicated collector-deadline exception;
monotonic checks also stop scheduling work when the budget is exhausted.
Each process receives at most min(30s, remaining collector budget), and every
read/inventory loop checks the collector deadline. On expiry, process-group
cleanup has at most its existing 5s wait, then the admitted run writes a failed
summary. Disable the timer before writing that bounded summary. The outer job
has a 10min timeout, leaving nearly 2min for cleanup/summary/artifact upload;
worst-case individual probe timeouts may exhaust the collector and must fail
truthfully rather than require all nineteen probes to run beyond that budget.
Runtime network isolation is explicitly unclaimed.

## Inert text fixtures and finite read-only observations

Fixtures contain only `.grok-plugin/plugin.json` with name/version/skills
metadata and `skills/<unique-marker>/SKILL.md` text. No hooks, agents, commands,
executables, MCP declarations, trust settings or account files exist. The fixture
name is a reserved unique research name in the otherwise fresh profile. Native
install, uninstall, enable/disable, global --trust, /hooks-trust, login/model
and MCP commands are absent. Creating these test-owned directories is fixture
data preparation, not a native plugin-registry mutation.

First record Grok --version, plugin list --help and inspect --help. Then record
fresh `grok plugin list --json` and `grok inspect --json` processes at each cwd
for these four stages:

1. Empty user/project-A/project-B roots.
2. Text-only user plugin version 0.9.0, with both projects otherwise empty.
3. Add project-A plugin with the same declared name, version 1.0.0 and distinct
   skill marker; project-B still has only the user discovery candidate.
4. Add project-B same-name plugin version 2.0.0 and a third marker; project-A
   remains unchanged.

Use the exact directory layout from the existing projector as the candidate
being investigated. Capture actual output rather than inventing native fields.
After fixture preparation, snapshot the complete fixture configuration/text
files and compare them after the read-only processes; no native-created scratch
or registry files may silently replace those baseline files. Bound each full
combined profile/project inventory to 4096 entries, 1MiB per regular file and
64MiB aggregate actual content read, with depth 32. Walk with lstat/no-follow
semantics and open regular files with no-follow admission; recheck type/identity
and before/after metadata. Record symlink target text and special-file metadata
without following symlinks, opening special files or traversing aliased
directories. Bound link text and metadata strings too. Refuse multiply linked
regular content rather than read an unproved external alias. Any overflow,
volatility or unavailable baseline makes collection fail; no partial inventory
becomes complete proof. Record native-created files separately with their
bounded hashes/metadata. Preserve A/B/user
native inventory differences as observations, never inferred ownership.

Every expected probe must complete with exit 0, nonempty help/exact 1.0.67 version
token where applicable, and strict complete JSON for list/inspect. Signals,
timeouts, overflow, error, missing/skipped probes or malformed JSON fail
collection; preserve per-probe stdout/stderr hashes, sizes and exit plus the
post-admission failure summary. A guard rejection prints a bounded diagnostic
and nonzero exit with zero output/probe/acquisition/execution effects.

The summary explicitly keeps `project_source_qualified:false` until a human
reviews the genuine populated outputs. Valid JSON collection alone cannot
establish exact source identity or discovery, even if the workflow is green.
If candidates are absent or the schema cannot attribute the selected project,
the adapter stays gated. Root/reviewers must inspect actual source spelling,
version/marker attribution and A/B/user precedence before approving the existing
single directory lane. Disabled/alias/ancestor and Claude-compatible conflicts,
MCP initialization, tool result, skill execution, native restart and actual UAP
project operations remain separate stronger native E2E gates. No direct TOML
fallback is added.

## Pre-publication verification

Commit this plan and both independent review/fix GO reports before source.
Keep the original matrix runner/require_hosted unchanged. Inert tests cover
Grok exact acquisition pins/caps, pure fixture boundaries, fixed argv/cwd,
credential-free environment, complete observation accounting and failure
classification; real-guard refusal creates no outputs. Reuse the existing
Qoder live output-limit/descendant deadline tests to verify the mechanical
shared helper move without executing a native client or spoofing hosted flags.
Python syntax, workflow YAML and both inert suites must pass. Obtain independent
implementation review before root publishes the stacked research PR.

Hosted output interpretation is a subsequent evidence step. This plan/code,
inert tests and genuine help do not constitute Grok project installation or
runtime proof, and they do not make the all 14 client E2E task complete.
