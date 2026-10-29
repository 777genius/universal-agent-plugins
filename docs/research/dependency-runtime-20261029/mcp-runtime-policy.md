# MCP reuse and Skill runtime: bounded design research

Snapshot: 2026-10-29, UAP `60967b8b6ca79245a61caf28895aa09c2c9d74a2`.
Status: recommendations and scoped experiments; no production feature or runtime
schema is introduced. Companion: [industry comparison](dependency-policy.md).

## 1. What "dedupe MCP" can mean

| Layer | Useful guarantee | Not implied |
| --- | --- | --- |
| Package graph | Multiple dependants can share one compatible canonical package node. | Unrelated bundled MCP declarations become the same resource. |
| Download/storage | Identical verified immutable bytes may be reused under the existing store policy. | Identical credentials, trust, data ownership or publication authority. |
| Client binding | An already-owned selected package can be reconciled within its existing client/profile/scope. | Taking over a foreign entry or sharing it with a different environment. |
| Running process/service | A host may implement an explicit service model. | One stdio process for all agents merely because they reference one package. |

The existing UAP dependency graph and ownership model already cover parts of the
first and third layers; they are not a missing universal dependency manager.
The documented source key, graph edges, selected targets and local receipts remain
separate authorities. [U1] MCP's stdio transport explicitly uses a client-launched
subprocess; sharing a running instance would add a broker/service lifetime and
isolation contract, not just an installer optimization. [S2]

### Recommended default: explicit common package

When two plugins need the same MCP implementation, declare one common package as
a dependency of both. Let the existing resolver combine compatible requirements
and target selections. Let the existing installer retain separate client bindings
and data authority. Explain to a user that a component is "provided by shared-mcp"
and used by plugins A and B. Removing A must not remove a component still reachable
from B. When neither root reaches it, an internal dependency cycle must not keep
it installed forever. Retained user data follows the existing removal policy,
not automatic storage garbage collection. [U1]

This is an authoring recommendation using existing mechanisms, not a claim that
arbitrary inline MCP entries are already merged. It reduces accidental duplicate
provisioning without inventing a new global resource registry or rewriting plugins.

### Do not auto-merge by name, endpoint, or server-reported identity

The same HTTPS endpoint can serve different accounts and tenants. The same command
can use different working directories, package revisions, environment values and
data roots. Different clients and project scopes can have different permission
and trust decisions. A duplicate name is a namespace conflict, not proof that one
entry may be discarded. These are concrete counterexamples, not assumptions about
any particular third-party MCP server.

For a future within-scope reuse feature, the comparison would at least need the
held canonical source/pin, client/profile/scope, effective configuration, data and
credential/permission references, and proven ownership. Even exact equality of
these model fields is only a candidate: secrets can rotate and runtime facts can
change. The installer must preserve its real authority checks and never export
secrets merely to compute or display equality.

Prefer diagnostics before automation: show which package owns a conflicting
entry, whether another plugin requires it, and what action is blocked. Keep the
normal view human-readable; offer source revisions and native paths in details.
Do not silently rename a tool/server and hope every consumer references the new
name correctly. Do not strip URL parameters or merge different authentication
contexts to increase a dedupe count.

### Alternatives considered

| Option | Benefit | Main cost | Recommendation |
| --- | --- | --- | --- |
| Explicit common package | Uses existing graph, pins and removal ownership. | Authors must reference the same canonical source. | Preferred starting point. |
| Exact within-scope owned binding reuse | Could reduce redundant configuration. | Must integrate drift, reverse ownership, update and removal atomically. | Separate bounded spike only after a real duplicate-use case. |
| Automatic cross-package URL/name merge | Appears effortless in a demo. | Loses account/data/permission distinctions and ambiguous removal authority. | Reject. |
| Global stdio broker | Could reduce processes for compatible servers. | New service lifecycle, protocol compatibility, permissions, isolation, reconnect and failure domains. | Out of scope; not justified for this installer. |

## 2. Skill artifact locking is not universal runtime locking

UAP already pins saved Skills, including bundled script and resource bytes. [U2]
For selected MCP packages using its reserved locked npm launcher, it also validates
`runtime.json`, the exact private production dependency and the npm lock digest,
then prepares the declared runtime. The implementation is deliberately selected
and bounded; arbitrary Skill scripts do not automatically opt into it. [U3]

Agent Skills guidance expects authors to declare prerequisites and use existing
runtime/package tools. Its script guidance is not a universal machine-readable
executor requirement. A textual compatibility field is useful to a person, but
cannot by itself prove tool availability or complete dependency closure. [S3]

The recommended layers are:

1. **Artifact:** source revision, bytes, bundled scripts and lockfiles are pinned
   by UAP. This is already supported for managed saved Skills.
2. **Runtime dependencies:** an ecosystem manager consumes its own lock with
   integrity/strictness appropriate to that ecosystem. Do not extract an invented
   global dependency graph from Markdown or arbitrary imports.
3. **Toolchain and platform:** interpreter, package-manager version, system
   libraries and OS/architecture compatibility need their own documented policy.
4. **Execution:** network, permissions, credentials, writable data and external
   service behavior remain distinct from installed bytes.

A future UI should distinguish "Skill installed", "runtime requirements not
checked / verified", and "agent activation or sign-in required" where applicable.
It should not display one green "ready" badge based solely on the presence of a
lockfile. Avoid forcing every novice through manifests, digests or raw paths in
the ordinary success flow.

### Smallest useful evolution

First improve author templates and examples: use standard PLUGIN_ROOT/DATA tokens,
ship the relevant ecosystem lock, state supported host requirements, and document
which command consumes that lock without resolving it anew. Reuse the existing
npm preparation path rather than create a second runner. A Python example can
use uv script metadata plus its adjacent script lock; uv documents creating this
with `uv lock --script`. [S4]

Be precise about flags. In uv, `--locked` checks that the lock is current, whereas
`--frozen` skips that freshness check. In npm, `ci` rejects manifest/lock mismatch.
The words "frozen" and "locked" are not portable contracts across tools. [S5, S6]

Only add declared preparation/readiness support when a concrete client journey
requires it. Keep it opt-in, use a bounded existing process runner, and do not
execute untrusted setup merely to render a catalog card or inspect a Skill.
Lifecycle/build scripts need explicit policy; turning them on globally to make a
package work would undo a different safety boundary. `--ignore-scripts` also does
not mean every native-addon package will be functional, so a successful download
must not conceal that limitation.

Do not require Docker, ship every language interpreter, install global packages,
or invent a universal runtime lock schema for all Skills. Container execution can
be a later explicit backend for users who need it, with its own image/host policy.
Do not claim a lock controls a remote MCP implementation or mutable web API.

## 3. Executed spikes and limits

Run from this directory:

```sh
python runtime_lock_spike.py
python mcp_identity_spike.py
```

### Real npm and pip resolution

The first script generates inert local npm packages and Python wheels. npm uses
an ephemeral loopback metadata server; pip uses `--no-index` local wheels and
`--dry-run`. No third-party scripts are downloaded or executed. Package/config/cache
writes are under temporary roots; the server and roots are closed/removed before
success. Python 3.13.5, npm 10.9.2 and pip 25.1.1 were actually exercised here.
Those are the available installed versions, not claims to be the latest releases.

| Case | npm result | pip result |
| --- | --- | --- |
| Exact parent 1.0.0; only leaf 1.0.0 available | leaf 1.0.0 | leaf 1.0.0 |
| Same parent bytes; leaf 1.1.0 becomes available inside its range | fresh resolution selects 1.1.0 | fresh resolution selects 1.1.0 |
| Complete old dependency selection retained | `npm ci` installs 1.0.0; lock bytes unchanged | explicit parent+leaf requirements retain 1.0.0 in the dry-run plan |
| Conflicting declaration | manifest/lock mismatch is refused | incompatible exact transitive requirement is refused |

Results: [runtime-lock-results.json](runtime-lock-results.json).
The pip full-pins example is deliberately **not** described as hash-verified
`pylock.toml` consumption. No package code was run. This experiment establishes
that a top-level exact artifact can still have floating runtime dependencies,
not that every package manager or UAP runtime has been qualified.

### MCP identity and removal counterexamples

The second script executes seven test methods with thirteen altered-authority
subcases. Equal endpoints fail the proposed equality candidate when account,
permissions, environment, data, working directory, pin, provenance, client,
profile, scope, transport, command or ownership differs. Identical foreign entries
are not adopted. Shared graph reachability retains a component after one consumer
is removed and drops an unreachable cycle after the last root is removed.

Results: [mcp-identity-results.txt](mcp-identity-results.txt).
This is explicitly a **design model**, not production UAP execution, native MCP
startup, account isolation certification, filesystem race testing or service health
proof. Passing equality tests is not permission to deploy a generic deduper.

The real UAP resolver already has conflict/backtracking/cycle tests; their actual
implementation is a separate evidence source. This report does not substitute its
model for those tests or claim fresh execution of that whole suite locally. [U4]

## 4. Adversarial self-review

No independent GPT-6.1 Sol/xhigh workers were available. The following are review
perspectives and resulting corrections, not fabricated independent approvals.

**UX challenge:** common packages burden authors. Response: show useful ownership
and conflict information first; a wizard or author template may help later. Do not
solve author effort by silently sharing users' accounts or changing tool names.

**Security challenge:** same bytes and configuration look equivalent. Response:
include provenance and scope; never adopt foreign entries. The model intentionally
calls equality only a reuse candidate, not proof of runtime identity.

**Lifecycle challenge:** reference counts keep cyclic components alive. Response:
keep root reachability as the package-graph concept; native bindings/data remain
subject to their own existing ownership and retention policy.

**Runtime challenge:** exact package versions should be enough. Response: the
executed npm/pip fixture demonstrates changing transitive selection without any
parent-byte change. Delegate full dependency locking rather than promise it from
Skill pinning alone.

**Architecture challenge:** a universal runner makes the UX consistent. Response:
that adds toolchain acquisition, build scripts, platform support and sandboxing.
Start with precise status, author templates and existing declarative preparation.
A declared command is executable code, not automatically safe metadata.

## 5. Implementation boundary

PR #571 corrects opaque placeholders and the stale shared-Skills-parser note.
Existing project-install docs were already updated in main; they are not rewritten
here. This research adds no broker, runtime interpreter, permission prompt, source
alias map, resolver mode or new production schema. Follow-on implementation must
have its own scope and evidence; a local research PASS is not native E2E readiness.

## Primary sources

- [U1: current dependency/ownership contract](https://github.com/777genius/universal-agent-plugins/blob/60967b8b6ca79245a61caf28895aa09c2c9d74a2/docs/UAP_DEPENDENCIES.md).
- [U2: Skills acquisition and lock identities](https://github.com/777genius/universal-agent-plugins/blob/60967b8b6ca79245a61caf28895aa09c2c9d74a2/docs/AGENT_SKILLS_INSTALLER.md).
- [U3: selected locked npm runtime](https://github.com/777genius/universal-agent-plugins/blob/60967b8b6ca79245a61caf28895aa09c2c9d74a2/install/integrationctl/agentplugins/providers/locked_npm_runtime.go) and [npm invocation](https://github.com/777genius/universal-agent-plugins/blob/60967b8b6ca79245a61caf28895aa09c2c9d74a2/install/integrationctl/agentplugins/providers/locked_npm_command.go).
- [U4: real resolver conflict/backtracking/cycle tests](https://github.com/777genius/universal-agent-plugins/blob/60967b8b6ca79245a61caf28895aa09c2c9d74a2/install/integrationctl/agentplugins/environment/resolver_conflicts_test.go).
- [S2: MCP transport distinction](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports).
- [S3: official Skill script guidance](https://agentskills.io/skill-creation/using-scripts).
- [S4: uv script dependency locking](https://docs.astral.sh/uv/guides/scripts/).
- [S5: uv locked versus frozen](https://docs.astral.sh/uv/concepts/projects/sync/).
- [S6: npm ci](https://docs.npmjs.com/cli/v11/commands/npm-ci/).
