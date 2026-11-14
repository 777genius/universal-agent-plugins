# Codex project implementation independent review R1

Reviewer: core_clients, 2026-10-23. Reviewed frozen `048c3455` including the
`fda36d23` TOML codec, `6387ac85` adapter and source-native discovery addition.
This review is independent of the codec-only R1 and the author implementation.

## Decision

**R1 GO at `4f4a320bc58c5c45e23a75d56c43bc0c2b3da1a6`.** No remaining
implementation blocker was found in scope authority, ownership, mutation or
native evidence boundaries. The R2 selector correction and R1 follow-up below
are independently inspected closed. Hosted native discovery is still pending;
this review supplies no runtime GO.

## Reviewed behavior

- Project admission uses the declarative native-config scoped policy instead
  of Codex user CLI-registry traits. The adapter requires both plan and target
  context to name project scope, retains the original detected user profile,
  and checks the explicit project registry root and no-follow native children.
  Project layout does not derive authority from a prepared package path.
- The production planner/stager/projector/activator path emits exact owned
  `.agents/skills` directories and `.codex/config.toml` MCP members. Existing
  rendered commands, OpenAI MCP projection, managed stdio package/data/cwd
  materialization and authentication references are reused. Active reprojection
  validates the bound stdio locators without editing immutable package bytes.
- Skill and MCP mutation consumes the reviewed shared component transaction
  and native ownership kernel. Per-object IDs, kinds, logical names, paths and
  digests fence update/remove, foreign disabled entries remain collisions, and
  missing exact owned skills can be repaired. Project operations do not run
  Codex user marketplace registration/removal or mutate its user cache.
- The TOML parser refuses malformed/duplicate documents. Retained syntax edits
  are re-parsed and compared against complete requested MCP entries and all
  non-MCP semantic settings before publication. Existing JSON/JSONC lifecycle
  remains on the same kernel. SSE is refused rather than silently converted.
- Project file success is always prepared activation plus installed
  verification. The adapter's preflight unconditionally rejects project
  activation attestation, including through the composed provider shortcut.
  Visible user/project restrictions are diagnostic observations rather than a
  claim to reconstruct native trust, profile, managed or session precedence.
- The new native gate is restricted to current-source Codex. It uses exact
  required-test accounting, rejects released/draft/prepared evidence inputs,
  retains hosted disposal and artifact integrity gates, and fails skipped or
  missing required tests. No guard is spoofed or removed.
- The native fixture uses fresh genuine pinned app-server processes and the
  documented `initialize`/`skills/list` route only. Its parser requires exact
  selected-project repository paths, enabled entries, no plugin identity,
  correct cwd, no discovery errors and no duplicates. Update, deleted-skill
  repair and removal observe fresh discovery; project B must not see A's
  fixture skills and recorded foreign user/cache/project files retain bytes.
  Exact rendered command bytes are checked. No thread/model request, trust,
  credentials, MCP initialization or effective-policy proof is manufactured.

## Independent execution

Go 1.26.10, GOMAXPROCS=3, `-p=2`, RAM toolchain/cache:

- Focused nativeconfig Codex contracts: PASS 0.011s.
- Codex client contracts (including existing user Codex cases): PASS 0.033s.
- Production provider Codex contracts: PASS 0.242s.
- Domain compatibility and planner goldens: PASS 0.005s / 0.020s.
- Exact new response-parser and disposable-boundary contracts: PASS 0.008s.
- Native runner Python safety/evidence suite: 56 tests, PASS 0.495s.

The first broader user tests correctly rejected this harness's case-duplicate
ambient proxy entries. A clean process environment resolved those failures;
production validation was unchanged. An overbroad repotest name selection also
reached existing legacy CLI build cases whose two uncached dependencies could
not be downloaded without inherited network configuration. Those unrelated
legacy cases are not claimed as passing; the exact new parser/safety contracts
were rerun successfully. No native client was executed on the task host.

## Selector correction and R1 closure

Independently compared the pinned native `codex-rs/config/src/skills_config.rs`
with `9d128aa2`. Ordered matching, trimmed names, canonical SKILL.md document
identity, ignored both-selector rules, and user/session-only selector authority
are reflected by the bounded user-file warning helper. Project rules no longer
manufacture a native disable observation; session effective policy is explicit
unknown.

One R1 follow-up was confirmed: native SkillsConfig decoding is all-or-nothing
per layer. A missing required enabled value or nonabsolute typed path
invalidates the section before selector interpretation, including a relative
path on an otherwise ignored both-selector entry. The initial correction
skipped only the invalid rule and could falsely retain a disable warning from
another rule. Closed in `b39a4e9d` and final `4f4a320b`: these typed-invalid
sections now suppress all bounded selector warnings. The focused composed
regressions retain the exact invalid-section cases.

Independently reran all `^TestProjectCodex` production provider contracts at the
final frozen SHA: **PASS 0.283s**. The code remains conservatively prepared,
never accepts project activation attestation, and never interprets a partial
settings observation as native effective policy. No production file was edited
by this reviewer and no native client was executed.

## Remaining gates

Retain both full independent implementation GO results on the final SHA. Run
hosted current-source native
discovery and cross-platform CI. Complete the saved production CLI composed
project journey after integration. Project MCP initialization, native effective
trust/policy, task completion and release qualification remain unqualified.
