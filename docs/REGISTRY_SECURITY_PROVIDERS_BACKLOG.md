# Registry security providers backlog

Status: **DEFERRED by owner, 2026-10-23**. This is a future integration backlog,
not an executable feature, release claim, or instruction to scan packages.
The owner deferred adding Gen Agent Trust Hub, Socket, and Snyk registry
providers while current source/project-installation work proceeds.

Do not add new provider dependencies, provider CI jobs, credentials, hosted
scanners, package uploads, or automatic third-party calls as part of that work.
Keep the existing signed LintAI assessment and exact-package verification
contract. See [current security and launch evidence](E2E_AND_COMPETITIVE_LAUNCH_PLAN.md)
for its versioned policy, signed subject identity, and local fallback behavior.
A vendor logo or an advisory registry verdict is not evidence that the exact
installed package bytes were assessed.

## Provider-specific prerequisites

The following are researched constraints, checked against the linked primary
documentation on 2026-10-23. Recheck them before any future implementation.

| Provider | Confirmed integration boundary | Deferred work before admission |
| --- | --- | --- |
| Gen Agent Trust Hub | The documented Skill Scanner lookup API accepts a ClawHub skill URL. That documentation does not establish an arbitrary UAP/Git package upload or exact-revision assessment contract. | Confirm a supported UAP subject/API with the provider, access terms, immutable byte identity, coverage, response provenance, and permitted retention. Do not infer general registry support from the ClawHub example. |
| Socket | Skills.sh scanning is experimental and requires contacting Socket for access; custom skills are marked planned in its support table. | Obtain supported custom-registry/skill access and a documented subject/result API. Pin the accepted coverage and maturity; do not treat ordinary npm scanning as a portable agent-plugin assessment. |
| Snyk | Snyk documents registry-partner integrations using its skill-analysis API, including Vercel and Tessl. That establishes partner use, not automatically granted UAP access or a general public submission contract. | Obtain designated registry API access and terms directly from Snyk; confirm payload, asynchronous job/result identity, complete subject coverage, and privacy/retention. Do not substitute a generic dependency scan or SAST upload for this provider contract. |

Primary sources:

- [Gen Skill Scanner and lookup example](https://ai.gendigital.com/skill-scanner).
- [Socket ecosystem and extension-scanning support](https://docs.socket.dev/docs/language-support).
- [Snyk and Vercel registry integration](https://snyk.io/blog/snyk-vercel-securing-agent-skill-ecosystem/).
- [Snyk and Tessl Batch Skill Analysis API](https://snyk.io/blog/snyk-tessl-partnership/).

## Skills.sh advisory data is a separate integration

The [Skills.sh API reference](https://www.skills.sh/docs/api) documents
Vercel OIDC authentication and a security-audit endpoint returning partner
verdicts. Its documented audit response identifies a skill by source/slug and
includes provider, status, summary, and assessment time. It does **not** include
an assessed content hash. The detail endpoint documents a content hash, but
joining that response to a separate audit does not prove that the audit covered
the same bytes. The API reference and its examples also differ in how they show
authentication for the audit request; verify the supported access contract
instead of assuming public anonymous access from a successful request.

Any future Skills.sh consumer must therefore label this data advisory unless
the provider supplies a verifiable immutable subject binding. It must remain
separate from UAP's signed exact-package assessment, from provider-specific
direct access, and from an authorization to submit package content. Missing
audits, a 404, inaccessible authentication, and a stale assessment are
unavailable evidence; none means a package passed.

## Future acceptance criteria

Before reopening any provider, propose a separate reviewed change with the
following concrete contract:

| Requirement | Required evidence and behavior |
| --- | --- |
| Subject identity | Bind canonical source, immutable revision, package/subdirectory, and the exact digest of assessed bytes. Explain any normalized or excluded files. |
| Coverage | State whether the assessment includes skill instructions, scripts/binaries, MCP manifests, native extensions, dependencies, and fetched content. Uncovered components remain explicitly unassessed. |
| Provider identity | Record authenticated provider identity, API/schema version, scanner/policy version where available, signed or otherwise authenticated result, and a traceable result/job ID. |
| Freshness and TTL | Record assessment time and validity policy. Reuse only when immutable subject, required coverage, and TTL match; ref changes and changed bytes require a new matching assessment. |
| Status | Preserve pass, warning, failure, pending, unsupported, unavailable, and error distinctions. Do not turn absent, stale, rate-limited, or failed evidence into pass. Define blocking policy separately from vendor wording. |
| Access and data handling | Obtain supported access/credentials and owner approval for the actual data sent. Document source secrecy, retention/deletion, quotas, asynchronous polling, timeouts and retry bounds. Never embed a shared secret in a distributed client. |
| Existing gates | Keep signed LintAI/exact-byte verification authoritative for its current contract. A provider outage or advisory result must not silently remove existing checks. |
| Tests and claims | Use contract fixtures for API/status/freshness behavior and a separately recorded authorized provider trial for live proof. Document known gaps and release status accurately. |

This backlog does not authorize implementing the contract, running a provider
trial, obtaining credentials, uploading source, or publishing a new security
claim. Reopening requires a future owner instruction after prerequisites are
concrete and reviewable.
