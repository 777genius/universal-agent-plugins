# Codex project adapter independent implementation review R2

Reviewer: cursor_clients, 2026-10-23. Reviewed the final adapter checkpoint
`6387ac8509a76b84cd42ef19bcf53ebcefa91dcb` and source-only native discovery gate
`048c34555543534bfc38eecf0a58cb64ff0d5ac5` in `work/uap-project-codex`.
The earlier codec-only review is separate and is not counted as full-adapter
approval. Scope includes production planner/provider composition, selected
project roots, exact native ownership, activation/deactivation, policy-warning
truthfulness, and the new hosted native-test selection and evidence boundary.

## Initial decision: NO GO pending configured-warning correction

The adapter's bounded skill restriction warnings do not match pinned Codex
`rust-v0.153.4` semantics. `project_status.go` currently inspects skill rules
from both project and user files and treats any matching disabled selector as
effective. Native `codex-rs/config/src/skills_config.rs` accepts skill rules only
from User and SessionFlags layers and processes ordered selectors, allowing
later matching enables to override earlier disables. Rules with both a name
and a path are ignored. Consequently the adapter can incorrectly report that a
preserved project selector disables a skill, or report a disable that a later
user rule reverses. The author independently confirmed the exact-tag source
and is correcting the narrow configured-warning claim. No project trust,
profile, ancestor, managed or session-policy evaluator should be added.

## Reviewed boundaries without another blocker

- Project context binds the selected root to its original detected user
  ConfigRoot; native authority is the project root, with independent contained
  `.agents/skills` and `.codex/config.toml` paths. Scope/root/user mismatches
  refuse before native effects. Prepared packages and persistent data remain
  under the project's `.uap` namespace. `CODEX_HOME` remains the original user
  profile; project operations do not invoke marketplace/cache mutation.
- Project skills and rendered command skills use the shared exact directory
  transaction; MCP uses CodecCodex and the existing nativeconfig kernel.
  Object identity, selected path and staged digest validation cover updates,
  missing-same-pin repair, drift, unowned collisions and exact removal. Foreign
  MCP fields, TOML comments/settings, skills and user cache remain outside
  ownership. Unsupported SSE is withheld rather than rewritten as HTTP.
- Active reprojection reads already staged values and verifies the bound
  package/data locators without changing digest-covered package bytes. Auth
  references retain the existing nonsecret OpenAI projection contract.
- Preflight executes before generic activation-attestation shortcuts and also
  on direct adapter activation. File configuration remains `prepared` plus
  installed verification with ActivationAttested false. Native trust,
  selection/aliases, effective managed/session policy and MCP initialization
  remain unobserved. Exact native-file verification is not a native session
  discovery claim.
- The source matrix adds the exact anchored project-test name to existing
  required Codex tests. Released/draft/prepared evidence inputs refuse the new
  selection. Existing immutable client pins, fresh environment and disposable
  hosted guard remain intact; skipped or omitted required tests cannot pass.
  The workflow is still limited to its existing configured platforms and
  triggers; writing this test is not evidence of a successful hosted run.
- The new skills-only native suite uses fresh app-server `skills/list`, exact
  repo scope/path/enabled checks, expected rendered command bytes and project-B
  absence through add/update/missing-skill repair/removal. It has no thread,
  model, trust mutation or configured MCP initialization request. Evidence
  explicitly separates those unobserved stages from project discovery.

## Independent verification

With Go 1.26.10, GOMAXPROCS=3 and bounded existing caches:

- Complete `clients/codex`: PASS, 0.140s.
- Complete `adapters/nativeconfig`: PASS, 3.624s.
- Complete `providers`: PASS, 1.888s.
- Complete `planner`: PASS, 0.023s; complete `domain`: PASS, 0.025s.
- `repotests -run 'TestCodexProjectDiscoveredSkills|TestNativeDisposableBoundary|TestNativePlatformEnvironmentIsolation' -count=1`:
  PASS, 0.014s. Only inert validators/environment contracts were selected.
- `python scripts/test_native_client_matrix.py`: PASS, 17 tests.

The task host carries duplicate uppercase/lowercase proxy environment names.
An initial pattern that also selected existing user Codex provider tests
reproduced their existing strict environment refusal. Excluding only lowercase
`http_proxy`, `https_proxy`, `all_proxy`, `no_proxy` for the full inert package
run gave the passing results above; no production/test/golden change was made
for that host condition. No native Codex process, package payload, model call,
MCP command, login or trust action ran during this review.

## Fix verification and final decision: GO

Independently rechecked correction
`9d128aa206fd9560a2c3d4d32602e4cf3b17ab59` against the exact-tag
`codex-rs/config/src/skills_config.rs` source snapshot. Only the visible user
file now contributes skill selectors. Ordered matching name/document selectors
honor later enables, names are trimmed, both/no selector combinations are
ignored and existing document aliases are canonicalized. Messages identify
the user settings and leave session/effective enablement unobserved. Project
selectors no longer manufacture a disabled-policy claim. This is bounded
observation, without a second native settings/precedence engine.

Independently ran all composed `providers -run '^TestProjectCodex' -count=1`
contracts at the fixed checkpoint: PASS, 0.244s. The new cases exercise
disabled canonical document/name, later name/path enable, ignored ambiguous
selectors, directory versus document path, ignored project selector and a
canonical document alias. Existing scope/rollback/attestation contracts remain
selected. No native process was launched.

The sole implementation blocker is closed. GO for the configured project
component implementation and authored source-only discovery gate. Hosted
runtime qualification remains separate: this review does not establish
native skills discovery, MCP initialization, effective trust/policy, model
use, real task completion or release readiness.

Final frozen-checkpoint follow-up: reviewed
`4f4a320bc58c5c45e23a75d56c43bc0c2b3da1a6`, including the native all-or-nothing
SkillsConfig section decoding guard. A missing required enabled value or any
nonabsolute typed selector path suppresses warnings from the whole section,
matching the native layer decoder. Independent complete `^TestProjectCodex`
provider selection PASS, 0.298s, including nine selector scenarios plus a
canonical document alias. Final decision remains GO at this checkpoint.
