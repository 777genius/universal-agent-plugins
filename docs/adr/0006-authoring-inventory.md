# Authoring command and policy inventory

Companion to [ADR 0006](./0006-standard-first-authoring.md), inspected at
`2a5346461c8078df083d18db8c4e88932020b4fd` on 2026-09-06. These are migration
decisions, not commands newly available in v1. The
[full plan](../STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md) controls
availability. Reuse means a behavior-neutral seam; adapt changes the project
model; migrate changes the workflow; retire removes it only at its gate.

| Current command or family | Decision | Standard destination / gate |
| --- | --- | --- |
| `init` | adapt | Missing-destination standard templates, Phase 3 |
| `validate` | adapt | Standard facts plus author policy, Phase 2 |
| `inspect` | adapt | Components, digests and separate evidence, Phase 4 |
| `compat` | adapt | Explicit target capabilities; no client writes, Phase 4 |
| `capabilities` | adapt | Shared generated schema/client metadata, Phase 4 |
| `doctor` | adapt | Read-only project readiness, Phase 4 |
| `test` | adapt | Offline static test Phase 5; explicit runtime Phase 7 |
| `skills init`, `skills validate` | adapt | Standard Skills services, Phase 5 |
| `skills generate` | migrate | Client preview `generate`, Phase 9 |
| `skills install` (`add`), `list` (`ls`), `update` (`upgrade`), `remove` (`rm`) | retire | External npm pass-through is not authoring; no silent lifecycle alias |
| `dev` | adapt | Bounded single-package watch, Phase 7 |
| `bootstrap` | adapt | Recognized native locked dependencies, Phase 7 |
| `generate` | adapt | Explicit disposable projection output, Phase 9 |
| `normalize` | adapt | Lossless JSON/CAS; no SKILL.md formatting, Phase 8 |
| `import` | migrate | Explicit native path and separate destination, Phase 8 |
| `export` | adapt | Deterministic standard archive, Phase 9 |
| `bundle` / `bundle fetch` | adapt | Narrow checksum-bound fetch and inspection, Phase 9 |
| `bundle install` | retire | Installer lifecycle only when source type is supported; otherwise precise error |
| `bundle publish` | migrate | `publish github`, Phase 10 |
| `publish` | adapt | Reviewed immutable destination plan, Phase 10 |
| `publication`, `publication doctor` | migrate | `publish inspect` diagnostics, Phase 10 |
| `publication materialize`, `publication remove` | retire | Old marketplace materialization is not a portable package workflow |
| `install [owner/repo]` | retire | Binary downloader is not `agentplugins add`; no deceptive alias |
| `integrations add/update/remove/repair` and root `add/update/remove/repair` | retire | Main `agentplugins` transactional lifecycle |
| `integrations list/doctor/sync/enable/disable` | retire | Main manager where equivalent; no automatic mapping for absent behavior |
| `version` | reuse | Product version stays distinct from shared engine revision |
| `__docs export-cli`, `__docs export-support` | reuse | Internal generation only, never public author help |
| Cobra-generated `help`, completion | reuse | Generated from each fresh root; no hidden command leakage |

New `migrate project`, `bundle inspect`, and authoring Skills subcommands are
registered only when their services meet the plan. The existing installer root
(`search`, `add`, `update`, `repair`, `remove`, `switch`, `rebind`,
`migrate-format`, `migrate-state`, `list`, `info`, `validate`, `outdated`,
`doctor`, `version`) is retained unchanged. Its `validate` and `doctor` keep
installer meanings. Current v1 help and JSON contracts remain authoritative
until the explicit v2 release. Until Phase 8, historical guidance points to
v1 `1.2.4`; future migration commands must not be advertised as executable.

## Policy classification contract

The independent conformance audit and Phase 2 extraction must turn each fatal
site (including propagated failures) in the sources below into a reviewed
fixture. This inventory freezes category decisions, not an assertion that all
fatal branches have already been behaviorally exercised. Do not merge a decoder
extraction on the strength of this prose alone.

| Current source / check | Classification | Required fixture/evidence |
| --- | --- | --- |
| `loader/plugin.go`: missing manifest, malformed JSON/field types, missing/unsupported schema, invalid core schema, field decoding | normative | Exact root, malformed UTF-8/JSON, duplicate keys, unsupported schema, invalid field type |
| `loader/plugin.go`: `plugin_name_unsafe` / `ValidateLeafID` | installer-policy | Schema-valid name rejected as a physical identifier; normative success preserved |
| `loader/plugin.go`: read failure / bounded file checks | host-safety | Distinguish permission/containment/size from schema facts; do not infer malformed JSON from unread bytes |
| `loader/loader.go`: `snapshot_invalid`, `snapshot_digest_invalid`, `schema_registry_missing`, context failure | host-safety | Bad snapshot/identity, unavailable embedded registry, cancellation; conformance unknown |
| `loader/mcp*`, `loader/skills*` | normative with narrow component boundaries | Invalid document, one server, one Skill; retain other valid components |
| `loader/plugin.go`: unknown fields, non-object extension container | normative with report/ignore loading boundary | Loadability does not imply authored schema conformance |
| `loader/native.go`, `openai_*`: compatibility manifest, hooks and path checks | separate format; host-safety or installer-policy | Explicit compatibility fixtures; never Agent Plugins conformance evidence |
| `packagedigest/snapshot.go`: traversal, unsafe link target, cycle, device/FIFO, resource limits | host-safety | Containment and bounded resource failures; valid contained links distinguished |
| `packagedigest/snapshot.go`: case/Unicode collisions and portable physical path checks | installer-policy | Cross-platform physical restrictions do not overwrite normative facts |
| `packagedigest/snapshot.go`: unavailable files, changing source, copy/seal/cleanup failure, context | host-safety | Interrupted/inconsistent snapshot never used as trusted parsed evidence |
| `sourceacquisition/acquirer.go`: Git identity, full SHA, source selector, expected digest, sparse tree/submodule/LFS restrictions | installer-policy | Immutable identity and supported acquisition contract preserved |
| `sourceacquisition/acquirer.go`: I/O, process bounds, unsafe tree paths, malformed tree, temporary ownership, cancellation | host-safety | No unbounded acquisition or unsafe cleanup; no false schema failure |
| Later deterministic export: reject symlink archive entries, dirty input, credentials and legacy manifest inclusion | release-policy | Contained source link may conform yet fail archive policy |
| Later destination publish: version/tag policy | release-policy | Missing/non-SemVer version not rejected by general standard conformance |

Each audit row must retain source location, error/code, format ID, category,
loading failure boundary, normative status (including unknown), installer
outcome, author/release outcome, and an executable fixture. Split compound
errors by underlying cause; never mechanically classify all wrapper errors as
normative. Include unwrapped context and filesystem errors. New or changed
fatal branches must fail the audit's coverage check until reviewed. The
foundation adds no production policy table and changes none of these checks.

Review controls: conformance facts cannot import authoring; standard authoring
cannot reach legacy packages transitively through a helper. The foundation's
AST import test checks all production Go files, including non-host build tags,
and follows local dependencies. Migration is a separately gated exception, not
an import permission for ordinary authoring. Publication services must stay
outside portable core facts.
