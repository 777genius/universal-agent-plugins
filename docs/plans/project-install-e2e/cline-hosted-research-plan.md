# Cline pinned hosted research before project adapter implementation

2026-10-23. The original Cline project plan and both independent GO reviews in
`client-plans.md` remain in force. This supplement obtains genuine read-only
grammar/config-source observations after the task-host binary failed before
help. No Cline adapter or research-runner source has been changed in this leaf.

## Research and missing evidence

The official npm package `@cline/cli-linux-x64` **3.0.73** is pinned to
`https://registry.npmjs.org/@cline/cli-linux-x64/-/cli-linux-x64-3.0.73.tgz`.
The captured registry SHA512 integrity is
`sha512-K+T94s0kPHLoMSANOJJ7KHb/etzYXZGQUbarKDKUPoZsvy4+oD1MAqYnZlfbKFM8tpnRQWouFngD9TB3mFqqeA==`.
Archive length is 50,817,241 bytes. Its regular `package/bin/cline` member is
132,511,200 bytes, SHA256
`1ff8841d17d4d83763134c193d414e4f7fcf2ef56af759e5ebbe96727fc2c8f7`.
The genuine version/help attempts each started Bun 1.4.2 and aborted 134 in a
fresh networkless namespace with empty HOME/project and empty /proc. Neither
successful grammar nor a runtime schema is claimed. Provenance, member list and
failure logs remain under `tools/native-probes/cline-3.0.73` in the task area.

Current primary references were reread:

- https://docs.cline.bot/getting-started/config
- https://docs.cline.bot/cli/cli-reference
- https://docs.cline.bot/mcp/mcp-overview
- https://docs.cline.bot/customization/skills

Bounded official source at Git tag `cli-v3.0.73` was read without executing it:
`sdk/packages/shared/src/storage/paths.ts`,
`sdk/packages/core/src/extensions/mcp/config-loader.ts`, and
`apps/cli/src/{main.ts,commands/config.ts,commands/update.ts}`. Static source explicitly selects
`CLINE_MCP_SETTINGS_PATH` over the global data/settings path and accepts nested
transport registrations as well as legacy flat entries. The config subcommand
has explicit `mcp` and `skills` JSON inspection branches and returns before the
default agent session. These are static observations, not substituted genuine
runtime output. Ordinary project startup must not be described as project MCP
discovery. The documentation requires explicit auto-approval control; no prompt
or task execution is part of this research.

Independent R1 identified a startup side effect in this exact tagged source:
`runCli()` invokes `autoUpdateOnStartup()` before parsing even short commands,
and the updater can defer a detached shell installer until process exit.
The tagged `commands/update.ts` explicitly returns early when
`CLINE_NO_AUTO_UPDATE=1`. Every Cline child environment must set this literal
value; a fresh profile alone cannot establish the no-updater boundary. Source:
https://github.com/cline/cline/blob/cli-v3.0.73/apps/cli/src/commands/update.ts

## Planned files, ancestry and real admission

After two independent GO reviews of this supplement, merge the independently
reviewed Grok/Qoder research utility checkpoint and add only
`scripts/research-cline-native.py`, its inert tests, and
`.github/workflows/cline-native-research.yml`. Reuse the already reviewed
`native_client_research` bounded read/process helper and Grok's `Budget` and
descriptor-relative bounded `inventory` via explicit adjacent module loading;
do not modify those helpers, either existing runner or the native matrix guard.
The Cline script calls the real imported `require_hosted('linux-amd64')` before
output/temp/probe directories, acquisition or native execution. Guard rejection
prints only a bounded diagnostic, returns nonzero and creates no requested
output parent or summary. No spoofed hosted environment or monkeypatched guard
may be used in local verification.

The separate workflow uses only GitHub-hosted Ubuntu 24.04, contents read,
pinned checkout/upload actions, exact PR-head checkout and
persist-credentials:false. A fixed explicit disposable-host opt-in is allowed
only there. Native children receive no workflow tokens, secrets, proxy, model
keys or inherited account/settings authority. The ten-minute job has an
independent eight-minute collector budget and always uploads bounded research
outputs after admitted failures.

## Exact bounded acquisition

Download only the exact HTTPS npm archive, with a request timeout no larger
than 90 seconds/remaining collector budget, actual compressed reads at most
64MiB, and the exact registry SHA512 before decompression. Bound gzip-expanded
tar bytes to 384MiB. Inspect at most 256 tar members, with bounded portable
relative names under `package/`, no duplicate/case-colliding names, links,
special files, traversal or absolute paths. Directory and ordinary regular
asset entries are permitted as metadata and never extracted. Read only the
single exact regular `package/bin/cline`, size at most 256MiB, require its exact
recorded byte count and SHA256, and write it to one fixed private executable
path mode 0700. Do not run npm, a package installer, postinstall script, the
vendor updater or any archived asset. No general extraction API is used.

## Fresh fixtures and finite read-only probes

Create separate binary/home/tmp/project-A/project-B roots. Use the existing
fresh profile_environment mapping plus literal PATH=/usr/bin:/bin, TMPDIR,
LANG and **CLINE_NO_AUTO_UPDATE=1** for every Cline invocation. This explicit
vendor no-update control is required before any version/help/config probe;
tests must assert it cannot be omitted or overridden. Default native environments contain no CLINE_DIR, CLINE_DATA_DIR, custom
config/hook/provider options or ambient product variables. Only the explicit
MCP-override observations add CLINE_MCP_SETTINGS_PATH, pointing to a known
test-owned A/B file inside this same disposable fixture.

Prepare only inert text fixtures: unique user, project-A and project-B
`SKILL.md` markers; the user global settings file and separate project
`.uap/clients/cline/mcp.json` files contain unique **disabled** remote markers
with inert loopback port-9 URLs, no credentials or OAuth. Use the canonical
nested `transport` schema selected by the future existing-codec lane. No
stdio command, executable, hook, JavaScript/TypeScript plugin, agent, prompt,
trust or account fixture is created. Each project gets local system
`/usr/bin/git init --quiet --template=` only, with the fresh Git environment and
no network or remote. No native settings/registry mutation command is allowed.

Run exactly four version/help probes: `--version`, `--help`, `config --help`
and `mcp --help`. Then run `config mcp --json` in six fresh processes: default
A; default B; A with A's explicit override; B with B's override; B with A's
override; and default A again. This observes explicit process selection and
its absence on a later ordinary process. Run `config skills --json` once from
each A/B cwd. These twelve Cline probes use fixed argv and selected cwd; every
invocation includes literal `--auto-approve false` and `--cwd <selected-root>`.
No prompt, plan/act task, interactive wizard, `config tools`, native install,
enable/disable/remove, auth, trust, hub/update or MCP connection/tool command
is permitted. Fixed config inspection branches were checked in tagged source;
unknown/nonzero output fails rather than falling through to a model proof.

Each child has DEVNULL stdin, no shell, at most min(30 seconds, remaining budget),
child-enforced 1MiB stdout/stderr caps and the unchanged process-group cleanup.
Acquisition/read/inventory loops and scheduling check the total collector
budget; signal expiry fails collection, retaining post-admission summary.
The bounded no-follow inventories use the existing 4096-entry/depth-32,
1MiB-per-file/64MiB-total and bounded metadata limits, reject hard-linked regular
content, and record symlink/special metadata without opening/following it.
Compare the complete fixture text/config baseline and record native-created
state separately. Overflow, unavailable/volatile inventory or changed baseline
is failure, not partial completeness. Runtime network isolation is unclaimed.

Record every expected probe, exact argv/cwd/explicit override, exit/signal,
elapsed time, stdout/stderr counts and hashes plus separate bounded logs.
Version must have exact 3.0.73 token boundaries, help must be nonempty, and each
config output must be complete strict JSON with clean stderr. Reject duplicate
keys, non-JSON constants, truncation, nonzero/signal/deadline/overflow, missing
or skipped probes. Capture populated output before interpreting its native
fields; JSON collection alone is not schema qualification. Summary fields
`project_source_qualified:false` and `project_runtime_qualified:false` remain
false until independent human review of actual marker/path/override attribution.
The explicit override does not establish automatic project MCP startup, an MCP
handshake/tool result, enabled skill execution, IDE discovery or native restart.

## Verification and implementation gate

Commit this plan and two independent review/fix GO reports before runner code.
Inert tests cover acquisition pins/caps/member admission, exact finite argv/cwd
and explicit environments including the mandatory literal no-update control,
credential/override boundaries, baseline accounting,
strict result/failure classification and the real zero-effects guard refusal.
Reuse existing live output/deadline/descendant tests, total timer and bounded
inventory tests without native execution or guard changes. Python syntax,
workflow YAML and relevant research suites must pass. Obtain two independent
implementation reviews before root publishes the separate stacked research PR.

Review genuine uploaded evidence before writing the Cline project adapter.
If version/help/config selection cannot be observed, the missing fact remains
blocked and no success is invented. The adapter must still follow the original
prepared/manual session-selected MCP boundary and provider attestation refusal;
the research workflow cannot turn that into ordinary native project activation.
