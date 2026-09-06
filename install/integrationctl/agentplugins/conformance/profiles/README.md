# Captured configuration profile v1

This profile is an implementation interpretation, not a competing specification.
It evaluates captured portable configuration and explicit containment observations.
It does not attest to client behavior, subprocess behavior, runtime dependencies,
credential provenance, authentication, publisher ownership or extension semantics.
No source is fetched during decoding. Unknown schema identities are unsupported;
invalid known configuration fails; unavailable evidence is not evaluated.

Normative sources, reviewed in full before implementation:

* Agent Plugins Specification 1.0.0, Agent Plugins contributors,
  https://github.com/agentplugins/agent-plugins-spec/tree/ff8ab5e392cc87bd88d87c060815a87490e51003
  (specification SHA-256 `97a658b7dca3ce1b4c2266b95da300fa51d9dc4ade59d73168e5f9104272da18`).
* Agent Skills Specification, Agent Skills contributors,
  https://github.com/agentskills/agentskills/tree/69ef37e9424c0a7ea9dd2293b559e43ec8176379
  (specification MDX SHA-256 `b9079c0c10b7930e8c6a20ff2bc10cda2a3343c55185120e3f1116a1a529b220`).

The source prose is licensed CC BY 4.0, https://creativecommons.org/licenses/by/4.0/,
provided without warranties. This concise rule mapping adapts those works;
it does not reproduce their examples or claim endorsement. Original licensed
source inputs remain audit evidence, outside the committed implementation.
Schemas retain the existing registry's exact identifiers and pinned digests.

| Rule reference | Source requirement and implementation |
| --- | --- |
| plugins/5.1 | Exact root plugin.json, sole authority. Captured path checked; no alternate format selection. |
| plugins/5.2 | Closed object, required exact schema, exact keys and metadata types. Original schema validation is independent of installer filtering; case variants remain opaque (AUD-018 correction). |
| plugins/5.3-5.5 | Required name and type-only optional metadata via pinned schema. No SemVer, URL, email, SPDX or physical-name policy added. |
| plugins/8.1 | Author schema requires an object container and object members. Installer reports and ignores a non-object container; non-object members remain rejected under the documented disputed interpretation below. Object contents stay opaque. |
| plugins/6.2 | Absent components are optional; wrong kind invalidates that component. Unreadable is unavailable, never absence. |
| plugins/7.1 | Only supplied immediate Skills; complete discovery must be supplied by the rooted reader. |
| plugins/7.2.1 | Closed explicit transport schemas; inert single-token command, allowed cwd anchors, absolute HTTP(S), no userinfo/fragment, loopback-only cleartext and valid case-unique literal headers. |
| plugins/7.2.2 | MCP document failure and server failure isolated; valid siblings retained. Unsupported schema differs from invalid known configuration. |
| plugins/10.1 | Cross-document schema version mismatch disables MCP. |
| plugins/4.1 | Configuration path prefixes plus supplied actual containment observations. Dot segments, contained links and Windows device names do not alone prove normative failure. args/env remain opaque. |
| plugins/9.1-9.2 | Reserved env keys rejected by schema; root/data placeholder usage recorded only in args/env/cwd. No expansion or PATH lookup is performed. Runtime/client obligations are outside captured configuration evidence. |
| skills/frontmatter | YAML mapping, string required and optional fields, name 1-64 and matches directory, description 1-1024, compatibility 1-500 if supplied, string-to-string metadata, inert allowed-tools. |
| host/input | Typed absent/present/unreadable/wrong_kind/blocked observations, exact logical paths, aggregate/component count bounds. Partial discovery preserves captured facts; failed discovery is unavailable and contradictory absence cannot pass. |
| host/bounds | 1 MiB plugin/Skill, 4 MiB MCP, 64 KiB frontmatter, 16 MiB aggregate, depth 64, 100000 tokens/nodes, 10000 members/inputs, 256 findings. Caller can only lower ceilings. |
| host/utf8 | Strict UTF-8 before parsing author input. Installer compatibility retains its historical byte interpretation. |
| host/duplicates | One structural scanner records decoded-key duplicates. Installer rejects root/known values and server-local duplicates while opaque unknown nested duplicates survive. Author duplicate ambiguity is host unavailability, independent of original schema failure. |
| host/yaml | Conservative lexical potential-depth guard precedes node allocation; actual nodes/depth/members are checked before map conversion. Aliases and merge keys follow YAML semantics with expansion work counted before conversion; cycles and budget exhaustion are host unavailability. Duplicate mapping keys are host ambiguity. |
| engine/schema | Only pinned local registry digests establish engine availability. A schema ValidationError proves invalidity; other engine errors do not. |

Interpretive decisions for the pinned Skills prose: its detailed name paragraph
explicitly says Unicode lowercase alphanumeric characters, despite parenthetical
ASCII examples. This profile accepts Unicode lowercase letters and numbers,
counts characters as Unicode code points, and does not normalize names. Unknown
frontmatter fields are not normatively prohibited by this prose and remain opaque.
Installer retains its ASCII names and unknown-field rejection, empty optional
compatibility acceptance, and unrestricted metadata values. YAML LF/CRLF framing
and EOF closing delimiter reuse the installer grammar; author input also accepts a leading UTF-8 BOM while retaining its original bytes. Body content has no format
restrictions; empty body is valid per the source minimal example. Markdown
references and scripts are not parsed or executed; their captured path containment
belongs to the reader. No arbitrary text is asserted secret-free by this decoder.

Finding identities hash code/rule/layer/boundary/logical location and opaque item
identity and deterministic duplicate-span ordinal, never messages or values. Public findings contain fixed message codes;
raw bytes, env, headers, extensions and underlying errors are not serialized.
Facts.Package is an internal canonical PackageEnvelope capability, excluded from
Facts JSON. Consumers must build allowlisted report DTOs and must not serialize it.

## Extension loading policy (published 1.0)

| Original input | Canonical author schema | Installer disposition | Extension interpretation |
| --- | --- | --- | --- |
| Absent or empty object | Valid | Load | None |
| Container null, string or array | Invalid | Report `plugin_extensions_ignored`, ignore, load healthy skills/MCP | None |
| Unknown namespace with object value | Valid | Load, retain raw bytes | Opaque |
| Unknown namespace with scalar, null or array value | Invalid | Reject manifest (disputed policy retained) | None |
| Unknown top-level field | Invalid | Report `plugin_unknown_field`, preserve, load healthy skills/MCP | None |
| Implemented namespace valid/invalid contents | Not applicable | No portable namespace interpreter currently implemented | No invented semantics |

The original author document is validated independently of the installer view.
Tolerant loading does not make invalid author input valid. Namespace names are
not subject to an invented reverse-domain regex. Regression tests exercise both
views and healthy component discovery in disposable roots without execution.
Canonical schemas, profile source digests and runtime member policy are unchanged.

At corpus `Booyaka101/agent-plugins-conformance-kit@4d163c6281a9b4929f482f387efe340b4dc175a1`,
`disputed/AP-8.1-EXTENSIONS-MEMBER-OBJECTS/fixture.json` has
`expect.rejected: null`, expects skill `alpha`, and records optional rejection
and an `extensions` report. Its rationale explicitly records the two readings;
it is not an authoritative scalar-member rejection requirement or certification.
The corpus pin and strict-reporting gate remain unchanged.

[Issue #77 discussion](https://github.com/agentplugins/agent-plugins-spec/issues/77#issuecomment-5488098709)
remained OPEN at the 2026-09-06 review and tracks the distinction between the
object-member requirement and ignoring unimplemented namespaces. [PR #82](https://github.com/agentplugins/agent-plugins-spec/pull/82)
was OPEN, unmerged, at head `6b0210e4bee84d8bea3f200091badbe796a5c88f`
on 2026-09-06. Its draft 1.1 container change does not settle published 1.0
member interpretation. Unknown schema versions, including draft 1.1, remain
unsupported. Revisit this bounded interpretation after authoritative clarification.
