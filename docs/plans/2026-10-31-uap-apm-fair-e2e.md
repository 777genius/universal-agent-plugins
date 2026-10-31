# UAP vs Microsoft APM: equivalent-observation E2E

Date: 2026-10-31. Base: `4d1f9e7655fad92ed3a3bc99fa25c92aefa4278c`.
Status: protocol proposal before execution and ratings.

## Goal and non-goals

Determine **observed** behavior for the *published* Agent Plugins 1.0 and
Agent Skills 1.0 ecosystem, not who has more CLI verbs. Run each product with
its own **documented** installation route, but require the same fixture bytes,
client, scope and externally observable outcome. APM's `apm.yml` and UAP's
`uap.toml` are manager contracts, **not** extra standard requirements.
Agent Plugins 1.0 standardizes skill and MCP components, not client loading,
vendor auth, executable sandboxing or package-manager commands.

No edits to Microsoft APM or other active UAP PRs. No releases, uploads,
user credentials, live provider billing, uncontrolled remote scripts or claims
of universal support.

## Product/source pin

Record exact Git commit, tool version, build/package SHA-256, OS/architecture,
test fixture digests, driver source and timestamp in each machine-readable
evidence receipt. Initial candidates: UAP from a known source commit (not
implicitly equal to published 0.1.95), APM **0.35.0** from the verified
release/commit. Distinguish a *released-to-released* table from a
*latest-source-to-latest-source* development table. Never compare one product's
unreleased fixes against the other's stable binaries and call it a product win.

## Neutral cases (shared bytes, distinct documented command syntax)

| Lane | Observation required | Boundary |
| --- | --- | --- |
| S1: Agent Skills 1.0 | Exact SKILL.md metadata and bytes after safe installation for selected client | May use each manager's documented packaging adapter; record differences |
| P1: Agent Plugins 1.0 | Same plugin.json + one Skill at standard fixed location | No UAP-only extension required |
| P2: Agent Plugins MCP | One inert stdio and one inert HTTP MCP declaration, component-specific validity | No vendor tokens or arbitrary network |
| R1: reproducible pin | Repeat install; fresh second environment from same saved exact lock where supported | Check bytes, manifests and no silent version upgrade |
| R2: source evolution | v1 then intentional v2; same accepted update/approval policy where comparable | Record immutable source identities; unsupported is not FAIL |
| R3: collision & drift | Foreign neighbor; edited managed output; repair/reinstall/remove | Never remove foreign or user-modified content silently |
| R4: interrupted operation | Fault injection after durable point; restart and recover | Only if supported hooks/real process boundary proven |
| N1: native acceptance | Reopen actual pinned client, list Skill/MCP, optional initialize/tools/list | Separate from file delivery; per client/OS/version |
| A1: auth/tool call | Explicit test-account authorization and harmless real tool call | Optional; no credentials available is BLOCKED, not FAIL |

Target matrix candidates: Claude, Codex, OpenCode, Cursor and Copilot. Start
with one shared **supported** target/fixture, then expand only after proving both
tools can exercise comparable flows. APM documents Agent Plugins v1 whole-package
deployment for Copilot; do **not** treat decomposing plugins into raw skills on
other targets as identical to that whole-package feature. UAP may require
a native client to be present for certain lifecycle actions.

## Safety & isolation

- Execute only in new disposable project, HOME, XDG, temp/cache and git config
  roots. UAP and APM runs get separate roots. Never use the host's real agent
  profiles. Preserve foreign sentinel file hashes before and after operations.
- No login or secret injection. Offline input files and loopback-only inert
  test servers. Downloads limited to authenticated/pinned installers/fixtures.
- Distinct clean run per scenario, bounded process tree/time, structured capture
  with stripped home paths and no credentials. A timeout is a **failed/blocked**
  observation with retained diagnostics, never an implicit PASS.
- Do not weaken dependency validation, enable unsafe options globally, use
  `--force` as an automatic success path, or grant trust/auth automatically.

## Scoring and evidence

Each case reports product, eligible target/scope, command, exit code, observed
files and hashes, verified native facts, exact source SHA, runtime, and one of:
`PASS`, `FAIL`, `UNSUPPORTED`, `BLOCKED`, `NOT_RUN`.
`UNSUPPORTED` requires a cited product contract, not a guess from exit 1.
`BLOCKED` includes missing auth/runner/network prerequisites. A file write
alone cannot satisfy `N1` or `A1`.

Report results in *two* tables: common eligible tests (PASS rates and
per-scenario differences) and product-specific capabilities (coverage; no
percentage win). Keep a frozen known-evidence snapshot and do not invent numbers.
A normal green CI without actual APM/UAP binary execution is **not** E2E proof.

## Two review gates

1. Before implementation: critics for standards purity, competitive fairness,
   security/identity isolation, and experiment methodology challenge case
   equivalence and scoring, especially the tempting but invalid “APM lacks
   Agent Plugins for all clients” assumption.
2. After first real run: independent reviewers inspect source-pinned receipts,
   commands, hashes, unsupported reasons and negative paths. Revise harness,
   then rerun **both** tools from clean roots. Publish comparison numbers only
   from admitted reproducible output.

## Execution scope

Use a bounded GitHub Actions workflow triggered only when comparison files change
in its PR, plus manual dispatch; this is a **path-scoped automatic experiment**,
not a globally required gate or a manual-only/opt-in trigger. Implement an executable small first-case harness,
then expand by evidence without compromising fairness. Never infer native-loaded
from copied files. All work in this branch only; keep user UX PR separate.

APM references:
- https://microsoft.github.io/apm/reference/cli/install/
- https://microsoft.github.io/apm/consumer/copilot-agent-plugins/
- https://microsoft.github.io/apm/reference/cli/deps/
Standards:
- https://agent-plugins.org/specification
- https://agentskills.io/specification

## First executable S1 admission (2026-10-31)

The initial workflow is an **evidence-only experiment**, not a scored leaderboard.
The comparison workflow runs automatically only for changes to its own harness,
fixture or workflow files, and is manually dispatchable; no project-wide new
required check, unattended recurring benchmark or user telemetry is introduced.
UAP is built from published `agentplugins-v0.1.95`, and APM is installed as
published `apm-cli==0.35.0`; the report records the UAP source commit,
both executable SHA-256 values and the APM installed Python dependency tree.
The frozen input is `tests/fixtures/uap_apm/skills/benchmark-note/` at Git
commit `a92e2a3c35a08327937b3facbf4b22791ae50eea`. Both original files
are hashed independently of manager manifests and output artifacts.

The first admitted target is **Codex project scope on a fresh ephemeral
GitHub-hosted Linux VM**. The VM is isolated from user profiles, but the two
product processes on it are not confined to separate writable mount namespaces;
therefore **host filesystem confinement is unverified** and cannot be scored.
A later stronger isolation benchmark needs a separately reviewed VM/container
sandbox and explicit, restricted mounts. UAP uses the released `skills add` command with exact
Git commit and path; APM uses its documented GitHub Skill subpath in `apm.yml`
followed by `apm install --target codex`. The runner records commands and
checks the **delivered** `.agents/skills/benchmark-note/` files; cached
downloads under `apm_modules` or `.uap` cannot qualify. Tool-specific
manifests and lockfiles need not have identical bytes.

Each product uses its own new HOME/XDG/cache/project/temporary/git-config root,
a constant inert foreign Skill neighbor and no secrets/SSH agent. No actual
Codex process is launched; native activation/auth/tool calls are `NOT_RUN`.
The baseline suite observes initial install, replay, fresh locked transfer when
the documented route actually produces one, and a separate foreign collision.
A non-zero collision attempt that leaves foreign bytes intact is `BLOCKED`
until independent inspection classifies the cause; it is **not** an automatic
safety PASS. Product timeouts are recorded with their process-group cleanup
and require manual analysis before labeling them as tool defects.

Before interpreting a result, check that both selected release commands are
actually supported by those release versions, that the selected client/target
is comparable and that no host/network prerequisite was missing. A green
benchmark workflow only certifies evidence capture. Other clients and whole
Agent Plugins 1.0 remain `NOT_RUN` until separately qualified. Report all
eligible/blocked/not-run counts; do not divide by only the successes.
