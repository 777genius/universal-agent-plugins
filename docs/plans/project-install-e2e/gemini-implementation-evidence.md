# Gemini project component implementation evidence

Date: 2026-10-23. Implemented frozen code: `65b8f17a`. The selected lane installs
native project skills and MCP configuration; it does not invent a project
extension registry. Independent implementation [R1](gemini-code-review-r1.md)
and [R2](gemini-code-review-r2.md) both grant GO for the bounded file and
lifecycle contracts.

## Pre-code official artifact and help research

Official [Gemini CLI v0.65.0 release bundle](https://github.com/google-gemini/gemini-cli/releases/download/v0.65.0/gemini-cli-bundle.zip):
21,215,960 bytes; SHA256
`bb5e7e879608bd9b3fe261930d3886fa56aacbf471da1655f2978eb35b705a73`.
The artifact checksum was verified before implementation. Actual read-only
version, skills help and MCP help probes ran with no network, no user mounts
and fresh isolated home/project directories. The version reports `0.65.0`;
skills help exposes list, enable, disable, install and uninstall; MCP help
exposes add, remove, list, enable and disable. These observations establish the
pinned help/schema research boundary, not project session discovery.

Primary documentation:
[configuration](https://geminicli.com/docs/reference/configuration/),
[skills](https://geminicli.com/docs/cli/skills/), and
[trusted folders](https://geminicli.com/docs/cli/trusted-folders/).

Read-only inspection of the exact released JavaScript bundle establishes:

- `chunk-AZOL6UFL.js`: project settings are `.gemini/settings.json`; trust gates
  workspace merging between user and system settings (`mergeSettings`, around
  line16338). The pinned schema defaults folder trust to true, resolving stale
  text in the separate trusted-folders page.
- The same chunk defines `mcpServers` named-entry shallow merge around
  line12455; `mcp.allowed`/`mcp.excluded` around line13987; skills enabled and
  union-merged disabled names around line14692; admin MCP/skills enabled fields
  around lines15098/15148. Admin sections use replace merging.
- Native settings parse JSON after stripping comments. Comments are accepted;
  input trailing commas remain invalid.
- `chunk-DPTJ5QZL.js`: original user configuration and trust storage stay under
  native user authority. Workspace `.agents/skills` is a higher-priority alias
  at the workspace tier; neither HOME nor GEMINI_CLI_HOME is rebound to the
  project by this implementation.

Offsets identify this immutable bundle and are not claims about future native
versions. No package payload, MCP server or model request was executed during
these probes.

## Implemented contract and limits

Planning, inspection, activation, verification and removal use the actual
selected project `.gemini` root while retaining detected user ConfigRoot.
Managed artifacts remain separately under project `.uap`. Exact MCP receipts,
skill directory ownership, rollback and no-follow/canonical path checks are
reused. Comment-aware Gemini writes preserve foreign entries/trivia; duplicate
keys and trailing commas remain invalid, and other strict JSON codecs retain
their previous parser.

Every configuration-only project success is `ActivationPrepared` plus
`VerificationInstalled`, with no activation attestation. Repeat automatic
reconciliation and read-only verification do not claim native discovery. The
existing preflight port rejects `--activation-complete` before the generic
shortcut, including direct adapter reuse. An ambient trust bypass flag is not
discovery evidence. Trust and policy files are never changed.

Warnings inspect configured user/workspace restrictions and potential
same-name `.agents/skills` alias paths. They do not recreate Gemini's full
settings engine. Session discovery, workspace trust, system policy and MCP
initialization remain explicitly unobserved. The existing user-scope lifecycle
stays unchanged.

## Verification

Production loader, planner, portable command renderer, stager and activator
contracts deliver ordinary skills, command skills and MCP entries into project
A; original user and project B byte/mode inventories remain unchanged. Tests
cover repeated activation/verification, exact repeatable remove, preserved
restrictions/alias, malformed settings, foreign same-name collision, and an MCP
writer failure after skill installation with complete skill rollback.

Author's exact Go1.26.10 checks: composed project contracts PASS0.063s;
native codec project contracts PASS0.008s; full Gemini/nativeconfig/domain
PASS0.028s/3.790s/0.040s. Exact golangci-lint2.13.2 changed-code and whole-file
size/complexity/architecture gates both report zero issues. Both independent
reviewers separately ran the frozen packages and composed tests successfully.

These are inert transport-independent file/composition contracts. Real Gemini
project skills/MCP listing from A plus a fresh session, and the cohesive
source/project CLI scenario, remain separate hosted qualification/integration
gates. The existing hosted-only native runtime guard remains unchanged.
