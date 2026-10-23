# Codex project component implementation evidence

Pre-code evidence recorded 2026-10-23. This implements the reviewed single
component lane from `client-plans.md`: project skills and command skills in
`.agents/skills/`, plus exact MCP entries in `.codex/config.toml`. Existing user
marketplace/plugin behavior remains separate and unchanged.

## Official pinned inputs

- Codex CLI **0.153.4**, official asset:
  https://github.com/openai/codex/releases/download/rust-v0.153.4/codex-x86_64-unknown-linux-musl.tar.gz
- Asset SHA256 `f479424eca092484dc40d87ae28c44f4cc40234a60045d6131e493800d814a30`,
  98,582,872 bytes. Binary SHA256
  `56ef98ab4032d317ab26e9b5e5a175650717351edb16ed9cde0cb6d1734d62da`,
  258,659,424 bytes.
- Parent independently ran this binary's version, top-level help, MCP help and
  `mcp list --help` in a networkless namespace with a fresh home. These prove
  version/help contracts only, without credentials, trust or model requests.
- Exact-tag native schema:
  https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/config.schema.json
  SHA256 `692da7699367f6f4fbbd46c0021278c1311440bcebf0bcb9b836690c05e56196`, 200,401 bytes.
- Exact-tag config-layer source:
  https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/config/src/loader/README.md
  and `loader/mod.rs`. Skill source is under `codex-rs/ext/skills/src/loader/`.

## Confirmed native contract

The pinned schema defines `mcp_servers.<name>` with native `command`, `args`,
`env`, `cwd` for stdio and `url`, `http_headers` for HTTP; it supports `enabled`.
Skills have `[[skills.config]]` disable selectors by absolute `path` or `name`.
The current official docs describe repository `.agents/skills` discovery from
cwd up to repo root and allow same-name skills from separate roots to coexist.
Managed ownership of a file does not establish which skill a session selects.

The exact config loader applies project layers only after native trust admission;
user profile, user base, system, cloud-managed, session and legacy managed layers
remain separate. Project config walks from the detected root toward cwd. The
implementation does not change `CODEX_HOME`, add trust or duplicate that settings
engine. An MCP entry may inherit/merge user or higher-priority policy fields.

Current official pages were opened before implementation:

- https://developers.openai.com/codex/config-file/config-basic
- https://developers.openai.com/codex/config-file/config-advanced
- https://developers.openai.com/codex/mcp
- https://developers.openai.com/codex/build-skills

## Implementation boundary

- Use core's validated TargetContext and scoped native-config lifecycle policy;
  native root is the selected project, with `.agents/skills` and `.codex/config.toml`
  validated independently under it. Original detected user profile stays intact.
- Add the narrowly required `CodecCodex` TOML format to the existing nativeconfig
  kernel. Reuse exact receipts, writer locks, no-follow file IO, conditional
  replacement, postconditions and rollback. Preserve foreign TOML members and
  comments. TOML decoding must reject duplicate/malformed documents before edits.
- Reuse existing loader, rendered command skills, immutable staging and native
  skill transaction semantics. Exact project objects govern update, repair,
  registry collision checks and removal; project operations never run global
  marketplace registration/removal or clean a user plugin cache.
- File-only success remains prepared with installed exact objects. Bounded
  configured disabled/shadowing warnings do not claim complete effective-policy
  evaluation. Generic activation attestation cannot manufacture project native
  discovery. Unknown versions or unavailable native observation remain explicit.
- Composed fixture journeys and genuine pinned process observations are recorded
  separately. Hosted native guard remains unchanged; no local model task, trust
  mutation, credential creation or package payload execution is authorized by
  this evidence record.


## Initial implementation checkpoint

The scoped adapter now selects the exact project root, `.agents/skills` and
`.codex/config.toml` while retaining the original detected user profile. It
consumes the reviewed shared ProjectComponents transaction with a distinct
Codex codec and explicit skill/MCP ownership kinds. The existing OpenAI MCP
projector materializes stdio package/data/cwd values and nonsecret auth
references once; active reprojection verifies the bound data locator without
rewriting digest-covered package bytes. Project operations do not invoke a
user marketplace command or change `CODEX_HOME`.

Native file success remains prepared plus installed. User/project disabled
selectors and MCP entries are preserved and reported; system/managed/session
precedence and native trust remain unobserved. Existing explicit-only command
withholding policy is retained, while accepted commands use the existing skill
renderer. No global user trait is enabled to manufacture project support.

Author contract checks on the initial adapter checkpoint: full Codex **PASS
0.161s**, full domain **PASS 0.029s**, full planner **PASS 0.033s**; composed
providers `^TestProjectCodex` **PASS 0.170s**. Those provider contracts include
standard loading/planning/staging, rendered commands, remote and managed stdio
MCP, auth references, exact data-bound active reprojection, add/repeat/missing-pin
repair/update/removal, user/cache/project-B isolation, malformed TOML, foreign
disabled collision, writer failure rollback, context/root refusal and generic
attestation rejection. They execute no package payload, native Codex process,
configured MCP command or model request. Independent full-adapter reviews and
hosted native discovery qualification remain separate gates.

## Source-only native discovery gate

The source matrix now accepts an explicit `--project-components` selection for
Codex. It retains the existing disposable-host checks, immutable native artifact
pins, fresh environment, exact required-test accounting and refusal of any skip.
The option is rejected for released, draft and prepared installer inputs; those
historical evidence contracts remain unchanged. The source Codex jobs select
both existing user tests and the project component lifecycle test.

The new test runs production UAP `add --scope project`, `update`, `repair` and
`remove`. Fresh pinned Codex app-server processes call only `skills/list` and
must return the exact enabled repository skill paths under the selected
`.agents/skills`, with exact rendered command bytes. A command-rule update must
withdraw the old command skill, a deleted owned directory must disappear from
native discovery before repair and return afterwards, and removal must leave no
fixture skill. Separate project-B discovery and exact foreign user/cache/A/B
byte checks fence the scope. UAP outcomes must remain prepared plus installed;
this test does not change the product's activation admission.

The exact-tag repository-root implementation independently resolves
`.agents/skills` from cwd to the detected project boundary:
https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/ext/skills/src/host_roots.rs
The native app-server contract is also exercised upstream at:
https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/app-server/tests/suite/v2/skills_list.rs
The fixture supplies distinct existing project roots and native `.git` markers;
it neither writes trust nor changes `CODEX_HOME` to a project path.

The discovery gate has been authored but has not yet supplied a hosted runtime
result at this checkpoint. No native process was launched on the task host.
Project MCP initialization, effective trust/policy, model use and real task
completion remain unqualified. The dedicated
`codex-project-components-evidence.json` records those limits and stage outcomes
separately from existing released lifecycle evidence.

## Independent review correction: visible skill selectors

Exact-tag native source at
https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/config/src/skills_config.rs
(blob `cc944d817c1a65c8424e069b5a7cd466646f84bf`) establishes that skill
enablement selectors come from user/session layers, not project layers. Path
selectors identify the canonical skill document; names are trimmed, entries
with both selectors are ignored, and later matching rules may reenable a skill.
The adapter's bounded warnings now honor these semantics for the observed user
file only, including canonical document aliases. They do not interpret project
skill selectors as native restrictions or infer a native session's final policy.
Composed provider regressions cover both override orders, ignored project/both
selectors, document-versus-directory identity and canonical aliases. The whole
project provider selection passed after this correction (0.228s); exact changed
code and full-file size/architecture lint retain the existing gates without new
suppression rules.

## Actual hosted project discovery result

The source-only macOS arm64 run now supplies a genuine **PASS** for
`TestAgentpluginsCodexNativeProjectComponentsLifecycle` (2.25s) with pinned Codex
0.153.4. The exact source was remote commit
`0d1319f947c7f4403691b3a60b1193baa0074ee9`, tree
`4b8935ef80c3834f7145dd01289772a48379ba3f`. The runner selected and passed this
test plus both existing user suites, with no skipped required test.

The [preserved project artifact and independent interpretation](../../evidence/project-scope-native/codex-0.153.4-darwin-arm64-40011489662/README.md)
record fresh exact project-A/project-B native skill discovery after add, command
update, an absent owned skill, repair and removal. All 28 declared project
sub-artifact hashes were verified against the downloaded ZIP; the ZIP digest,
official client pin, installer digest and run/job links are recorded there.

The client explicitly warned that untrusted project configuration was disabled
while skills remained discoverable. No trust was added. The product therefore
still reports prepared plus installed, and this result does not qualify project
MCP initialization, model use, authentication, another OS or a release. Windows
native qualification is a separate running workflow. The raw removal result also
exposed a missing project context in its retained-data follow-up command; a
separate reviewed correction preserves this original artifact unchanged.
