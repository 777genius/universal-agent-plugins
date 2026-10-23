# Copilot CLI project component pre-code evidence

Recorded 2026-10-23. The original 14-client plan has two independent GO rounds;
this narrows its already approved Copilot component lane with pinned artifact
and observed help/list contracts. No Copilot product implementation has been
changed at this checkpoint.

## Official pinned artifact

- Release: **1.0.103**, published 2026-10-22,
  https://github.com/github/copilot-cli/releases/tag/v1.0.103
- Official archive:
  https://github.com/github/copilot-cli/releases/download/v1.0.103/copilot-linux-x64.tar.gz
- GitHub release API asset digest and independently measured archive SHA256:
  `9aeb4430e9ac9523546db07b227c7aba179a4710a55f9ac29ad143b0f9e1b325`.
  Exact archive length: **129,018,878 bytes**.
- The archive contains one regular `copilot` executable, **198,445,888 bytes**.
  Independently measured executable SHA256:
  `f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d`.

Version/help/list probes used the managed network-disabled filesystem sandbox
with fresh HOME, XDG roots and project cwd, an explicit executable and minimal
environment without inherited credentials, proxy, account or product settings.
An attempted explicit network namespace was refused by the kernel before any
client launch; no namespace isolation result is claimed. The successful managed
sandbox probes neither enabled a native-suite opt-in nor bypassed its hosted
guard. These are pre-code observations, not hosted native qualification.

## Observed native contracts

`--version` exited zero and reported `GitHub Copilot CLI 1.0.103.`. Top-level,
`mcp`, `mcp list`, `skill` and `skill list` help all exited zero. The pinned CLI
documents machine-readable listing via `mcp list --json` and `skill list --json`.
MCP commands name user, workspace and installed-plugin sources; skill commands
name project, personal, plugin and custom roots.

In a fresh real Git repository, `skill list --json` returned the inert
`.github/skills/uap-precode-observer/SKILL.md` marker with its name, description,
`source: project`, exact **directory** path and `enabled: true`. Other native
builtin rows remained distinct. This observation does not execute the skill.

With a distinct user HTTP config marker and root `.mcp.json` marker,
`mcp list --json` returned a `mcpServers` object containing the exact user
marker with `source: user` and `enabled: true`; the untrusted workspace marker
was absent. Both URLs were inert loopback port 9 and outbound network syscalls
were blocked by the managed sandbox. No trust, account, model task, configured
server invocation, permission grant or tool execution was requested. The absent
workspace marker is consistent with the documented trust boundary; it supplies
no project MCP discovery or initialization proof.

## Official configuration and authority references

- https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers
  documents repository `.mcp.json` and `.github/mcp.json`, rootward Git-boundary
  discovery, nearest-directory and same-directory `.mcp.json` precedence, nested
  or bare server maps, trust requirements and exclusion of `.vscode/mcp.json`.
- https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference
  documents additional/session config and plugin precedence over workspace
  definitions. Current official search retrieval supplied this reference;
  direct page retrieval timed out, so no independent full-page snapshot is
  claimed.
- https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-skills
  documents project `.github/skills`, `.agents/skills`, `.claude/skills` and
  separate personal paths plus explicit enable/disable and discovery commands.

## Implementation direction within the approved plan

Use the validated original user/project context and reviewed shared exact
component transaction. Project mutable/package/data paths stay in `.uap`;
discovered skills use `.github/skills`. Select root `.mcp.json` when present,
otherwise `.github/mcp.json`, while fencing aliases and any changed effective
selection against recorded exact ownership. Reject preexisting bare maps before
effects rather than silently wrap them and lose foreign server meaning.

Add the narrow Copilot transport codec to the existing kernel: native stdio
command/args/env and explicit HTTP/SSE tags. Reuse the existing managed stdio
launcher to establish cwd and package/data locators. Retain user global
marketplace behavior unchanged; project add/remove must never run global plugin
or user MCP mutation commands. Existing unsupported portable command policy
remains in force.

Successful exact configuration stays **prepared plus installed**. Fresh native
project discovery, folder trust, plugin/session/admin precedence, disabled
selection, MCP initialization and tool use remain separate observations.
Project activation attestation cannot supply those observations. Copilot and
VS Code have distinct project physical backend identities and native config
paths; shared user backend behavior remains unchanged.

## Worktree checkpoint

`feat/project-copilot-e2e` at the pre-code merge `e3d0f475` contains integration
`d9f3f757` plus reviewed helper ancestry `c1b523b9`, with no other new client
adapter. Root publication alignment and final core corrections remain separate.
No real Copilot project install, native runtime certification, hosted runner,
release, remote publication or user account operation was performed.

## Implemented project component contract

The project branch now uses the existing scoped planner, managed artifact/data
paths, exact native ownership kernel and reviewed component transaction. It
selects a regular root `.mcp.json` when present and otherwise
`.github/mcp.json`; owned receipts fence subsequent selection changes. Existing
bare maps and malformed/ambiguous objects fail before native effects. Foreign
servers (including disabled entries), settings and skill directories remain
outside ownership. Copilot's narrow codec emits explicit `stdio`/`http`/`sse`
and native `tools:["*"]`, preserving exact member receipts rather than replacing
whole foreign documents.

Selected stdio uses the existing managed launcher and explicit package/data/cwd
binding. Read-only active projection recomputes the selected source contract
against the current package and selected data locator; a missing, relative or
different data locator refuses with `ErrProjectionRefreshRequired`, with no
immutable bytes rewritten. Project registry inspection does not launch the
global Copilot CLI. Prepared inspection catches exact native collisions even
without native execution, and stale recorded scope/artifact/locator/config
selection fails closed.

All project activation attestations are rejected before automatic/manual
fallback. Exact successful configuration remains prepared plus installed and
not attested. Native project discovery, folder trust, plugin/session/admin
precedence, disabled selection, MCP initialization and tool use stay unobserved.
The user marketplace/global CLI and unsupported portable command policy remain
unchanged. The project physical backend is `copilot`, distinct from editor
project delivery; user backend sharing remains intact.

## Executed local verification

Production loader/planner/stager/provider/kernel contracts cover both effective
MCP paths; foreign settings and user/B isolation; exact install, verify, update,
missing-skill repair, dropped MCP and repeated removal; bare/malformed maps,
disabled same-name collision, skill collision, changed selection after staging,
native-writer failure with rollback, context/alias refusal, stale ownership,
attestation refusal and selected-data immutability. Focused provider suite:
**PASS 0.078s**. Copilot client **PASS 0.008s**, native kernel selection **PASS
0.006s**, domain policy **PASS 0.005s**, compatibility goldens **PASS 0.016s**.
The only four golden differences add Copilot's project scope.

`TestProjectCopilotComposedSavedLifecycle` uses the public CLI and production
ForScope/solver/store/planner/installer/native-file transactions with real TLS
Git acquisition: saved A/B add, locked no-op ownership stability, exact old-pin
repair after moving source, actual pending directory-journal recovery and
repeat recovery, moving update retaining binding/data authority, conservative
saved status, exact remove and absent repeat without effects. The complete user
Copilot profile and project B stay byte-identical. **PASS 6.620s**. Host detection
and package-security assessment are inert seams. The trusted test executable is
copied as managed launcher bytes and never executed; no fake Copilot process or
native proof is involved. Pending-journal recovery is a bounded production
journal fixture, not an observed native-process crash/restart.

Exact changed-code lint and whole-file size/architecture gates for the component
packages passed with **0 issues**; public CLI changed-code lint also passed with
**0 issues**. The full existing native-config kernel suite then passed in
**3.337s**, the full Copilot client suite passed in **0.009s**, and the public
CLI whole-file size/architecture gate passed with **0 issues**. Two independent implementation reviews are pending at this
checkpoint. Genuine pinned pre-code observations above remain distinct from
the new production project contract. No native-suite guard, hosted environment,
authentication, trust, vendor model, release or publication operation changed.

## Independent R1 correction: exact physical artifact

R1 found that initial project admission checked project containment but did not
require `ActivePath` to be the selected Copilot physical artifact. The corrected
plan admission validates the artifact leaf and literal
`.uap/managed/clients/copilot/<PhysicalArtifactID>` path before projection,
activation, inspection and active reuse. The generic precondition still checks
scope/root before the planner resolves its physical artifact; subsequent
refinement and lifecycle use the complete exact validation.

New regressions change either the package to another existing in-project child
or only the selected artifact ID. Composed activation, direct activation,
direct projection and read-only reuse all refuse with identical complete
filesystem inventories. Focused providers **PASS 0.073s**, client **PASS
0.006s**, domain **PASS 0.006s**, compatibility **PASS 0.014s**. The independent
reviewers must close the correction before final publication/merge approval.
Public saved lifecycle recheck **PASS 5.614s**; exact changed-code and full-file
size/architecture correction lint gates both **0 issues**.

R1 additionally required the direct adapter to enforce delivered-path equality,
which the composed provider already checks. Project preflight now rejects any
nonempty delivery path different from the selected plan; pre-staging empty
delivery remains admissible. Direct activation requires exact nonempty equality.
The new delivery-only mismatch regression verifies the existing foreign child
and empty direct delivery both refuse without any base inventory changes.
Full project provider suite **PASS 0.072s** at this follow-up.

## Final independent implementation reviews

Both independent implementation reviewers closed **GO** at
`b98f0c57983433e5c7f2cde76255f94f4da1a079`. R1 independently reran final project
providers **PASS 0.079s** and complete public saved CLI **PASS 5.378s**; R2
independently reran exact artifact/direct delivery refusal contracts **PASS
0.033s**, retaining earlier full project/provider/codec/domain/client/public
saved CLI checks. The reports are committed beside this evidence. No blocking
implementation finding remains.

This is a reviewed production configuration/ownership checkpoint. Genuine
Copilot project native discovery, trust, MCP initialization/tool use and restart
qualification remain pending; neither source tests nor the pinned pre-code
untrusted listing is promoted to those claims. The extracted vendor executable
was hash-checked against the recorded binary pin and reclaimed to restore task
disk headroom; its original archive, provenance and all research logs remain.
