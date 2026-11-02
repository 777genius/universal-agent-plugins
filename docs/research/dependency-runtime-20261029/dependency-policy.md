# Dependency policy: industry comparison and UAP decisions

Research snapshot: 2026-10-29. UAP source: `60967b8b6ca79245a61caf28895aa09c2c9d74a2`.
Status: research recommendations, not an accepted resolver migration or a release.

## Question and boundary

Make UAP predictable before broad adoption without copying another ecosystem's
historical constraints. This comparison covers fifteen general-purpose package
managers and three agent-oriented tools. It is an architectural sample, not a
measured popularity ranking or an executable benchmark of all eighteen tools.
Facts come from the linked primary documentation; recommendations below are our
interpretation. Executed npm/pip experiments are identified separately.

A package format, a dependency resolver, a content store, a client configuration
binding and a running service are different layers. Agent Plugins does not make
npm's version grammar or any one solver mandatory. The published specification
page identifies version 1.0.0 and Working Draft status; its normative requirements
still govern an implementation's claim to follow that version. The MCP placeholder
correction is a separate implementation task in PR #571, not evidence that every
client/runtime path is conformant. [S1]

## Are Skills already locked?

Yes. In UAP's saved managed workflow, the selected Skill is a dependency in
`uap.toml`. `uap.lock` records the immutable source commit, resolved Skill path,
raw-source digest, normalizer version and normalized package digests. The original
Skill document, bundled scripts and resources are retained. A descriptive
`metadata.version` is not the authority for the installed bytes. These are existing
features, not a proposed new lockfile. [U1]

That lock fixes a script's bytes, not every external thing it may download, import
or contact when executed. A bundled language lockfile is itself pinned as content,
but its dependencies become controlled only when the intended package manager
actually consumes it with the appropriate strict behavior. Host runtimes, native
libraries, credentials and remote services are further boundaries. See the
[runtime/MCP report](mcp-runtime-policy.md) and executed fixtures.

## Comparison matrix

The columns deliberately separate fresh selection, preservation and ecosystem
constraints. There is no universal meaning of `install`, `frozen`, a bare version
number, or "dedupe" across all these products.

| Tool | Relevant documented behavior | Transferable lesson / limitation |
| --- | --- | --- |
| npm | Existing compatible lock governs installation; an unqualified package request normally follows the configured tag, usually `latest`. `npm ci` requires a matching lock and does not rewrite it. [M1] | Separate intent, lock replay and update. A dist-tag is not an immutable identity; nested library layouts are not shared MCP ownership. |
| pnpm | `resolutionMode` currently defaults to `highest`; `lowest-direct` was the default only in 8.0.0–8.6.12. Time-based and lowest-direct modes remain alternatives. CI supports frozen installation. [M2] | Do not copy a remembered historical default. Content/layout reuse and chosen graph policy are separate decisions. |
| Yarn | Immutable installs preserve the lock. Explicit dedupe can reuse higher locked locators for overlapping ranges, never downgrading with that strategy. It warns that SemVer compliance is not assured. [M3] | Dedupe can be an explicit graph change, not a hidden optimization during replay. Show a diff. |
| Bun | `bun.lock` and frozen installation separate resolution from replay. Dependency lifecycle scripts are restricted by trust policy; root scripts are a separate case. [M4] | A successful package install and permission to execute downloaded code are different promises. Do not transplant its entire layout/runtime. |
| pip | The resolver can backtrack from newer candidates to satisfy the graph. Current documentation includes experimental `pip lock`, scoped to the current Python/platform. [M5] | Avoid outdated claims that pip has no lock command. A single exact top-level requirement does not pin all transitive requirements. |
| uv | Highest-compatible preference by default; existing compatible locks are preferred; lowest and lowest-direct strategies exist. Universal marker-aware locking is broader than one environment's installed set. [M6] | Prefer stable locked behavior while treating minimum-version testing as a separate author concern. Do not copy Python marker machinery without a UAP need. |
| Poetry | Initial installation resolves compatible versions; an existing lock drives exact installation; updating is a separate operation. [M7] | A familiar install/update distinction is useful without adopting a second UAP manifest. |
| Cargo | Generally prefers highest versions, with constraints such as Rust compatibility; reuses locked choices. SemVer-compatible unification and incompatible-version coexistence have distinct rules. [M8] | A good source of backtracking and conflict explanation ideas, not permission to install two identically named native resources. |
| Go modules | MVS selects the highest minimum already required for each module path in the graph. Major-version import paths distinguish identities. `go.sum` is checksum evidence, not a conventional solver lock. [M9] | UAP's lowest available caret-satisfying candidate is NOT Go MVS. Lower-bound-only Go constraints cannot be equated to UAP ranges with upper bounds. |
| Composer | Install consumes the lock; update resolves current matching versions. `--prefer-lowest` is an explicit alternative useful for lower-bound tests. [M10] | Reuse conservative replay and author-side compatibility testing, not a default belief that oldest means safest. |
| Bundler | Conservative install retains unaffected locked gems; selected updates may change shared transitive dependencies unless constrained conservatively. [M11] | Define what "update this dependency" may change and show the shared impact before applying. |
| Maven | Default mediation chooses the nearest definition; equal-depth ties use declaration order. Dependency management can control the choice. [M12] | This is a different classpath policy, not a universal best solver. UAP should not silently ignore an incompatible consumer because its root is nearer. |
| Gradle | Default conflict handling prefers the highest requested version; rich constraints, capabilities and explicit locking alter the outcome. [M13] | Distinguish version conflict from resource/capability conflict. No need to copy the entire rich constraint system. |
| NuGet | PackageReference combines lowest-applicable selection, floating-version rules, direct-dependency-wins and cousin resolution. Floating patterns select their highest match. [M14] | Lowest selection is a real industry policy, but its override/diagnostic rules are not automatically appropriate for agent plugins. |
| Nix flakes | Input locks preserve existing up-to-date entries; updates explicitly advance them. The cited command interface remains experimental. [M15] | Useful source/environment separation; not evidence that UAP needs Nix's build/runtime architecture or that a source lock is a complete hermetic environment. |
| Microsoft APM | Manifest plus lock, immutable resolved sources, frozen mode, and highest matching tagged versions for supported ranges. [A1] | Closest conceptual comparator; package acquisition does not itself interpret UAP-specific declarations. Avoid two competing graph/configuration owners. |
| Vercel Skills | Project restore uses `skills-lock.json`; global tracking is separate. Its command documentation distinguishes npm dependency installation from restoring skills provided by npm. [A2] | Artifact restore and runtime/package preparation are separate layers even in an agent-oriented installer. |
| Skillshare | Project configuration records intent; its lock records source, commit and tree hash for supported remote sources; local-source limitations are explicit. [A3] | Source pinning is useful without pretending all resources or runtime requirements have portable pins. |

## Decisions to retain

**Preserve locks before optimizing freshness.** Ordinary locked restore must never
silently select a newer version, reinterpret a mutable Git tag, replace bytes at
an archive URL, or discard a dependency edge. A pin that becomes prohibited should
produce a clear refusal, not an unannounced upgrade. Existing UAP source/digest,
revocation, ownership, target and repair rules remain authoritative. [U2]

**Keep one selected version per canonical source in one environment for now.**
UAP projects native Skills/MCP configuration into agent namespaces; npm-style
nested libraries do not establish isolated native resources. Side-by-side versions
would require a concrete namespace/data/activation/removal contract. Rejecting an
incompatible shared dependency is currently easier to explain than silently
installing a second server with ambiguous ownership. Cross-project environments
remain separate; this is not a global one-version rule. [U2]

**Keep root authority and source provenance.** A dependency's own `uap.lock` must
not override its consumer's chosen graph. Equal content hashes across Registry and
Git do not prove equal publication authority or revocation policy. Prefer one
explicit canonical shared dependency in authoring; do not alias unrelated sources
just because their bytes match. [U2]

**Do not turn dependency edges into service orchestration.** Co-installation is
not launch ordering, readiness, account authorization or health. This boundary is
already documented by UAP and should remain explicit. [U2]

## Fresh resolution: recommendation, not a silent change

For a broad audience, the leading candidate is **highest eligible compatible
stable preference when a node has no usable existing pin**, with deterministic
backtracking and stable ordering. This resembles several contemporary developer
workflows and avoids routinely selecting an obsolete lower bound. It is a ranking
preference, not a promise to globally maximize every version in an NP-style search.
Every consumer constraint and current security/client/platform policy still wins.

The alternative, keeping UAP's current lowest-compatible policy, offers smaller
initial choices and predictable lower-bound behavior. However, minimum does not
prove safety: old releases may contain fixed bugs. Highest does not prove safety
either: a newer publication may be compromised or have an incorrect compatibility
claim. Eligibility and provenance must be evaluated before ranking, independently
of any future release-age policy. No new security bypass or cooldown knob is
introduced by this research.

The middle option, lowest direct plus higher transitive dependencies, has real
precedents but is harder to explain here and is not justified by measured UAP
usage. Do not add three public solver modes merely because other tools expose them.
A minimum-version test mode for package authors can be a later, independent need.

Before changing the fresh default, require a bounded real-Directory fixture:
multiple eligible versions, old compatible lock, revoked/new incompatible version,
shared constraints, 0.x caret boundaries, a backtracking candidate, moving Git
source, same-version publication sequence, and all selected client targets. Show
exact old/new graphs and prove existing lock replay is byte-stable. Document the
behavior once in the CLI, SDK and author guidance. No lock-schema migration is
necessary merely to reverse fresh candidate preference, but the user-visible
selection contract changes and needs explicit acceptance.

PR #563 separately implements latest-compatible **explicit updates**, including
no silent downgrade. At this snapshot it is a draft, not a released guarantee.
It must not be duplicated or confused with a change to ordinary fresh resolution.
[U3]

## Version grammar: familiar subset before more syntax

Keep exact stable versions and the documented caret subset until a concrete
publisher requirement justifies expansion. A partial grammar is not inherently
non-standard; ecosystems disagree about bare versions, prereleases and 0.x.
Publish valid examples and rejection messages, and test the boundary cases. Never
interpret an unsupported operator as a different supported selector. [U2]

Tilde/comparator/disjunction support, prerelease opt-in and SemVer build metadata
need separate contracts before implementation. Prefer a maintained semantic
version implementation over a growing custom parser when that decision is made.
An unrestricted wildcard should not be introduced as a workaround for missing
constraints. Keep human version, immutable revision and Directory release sequence
distinct: a later sequence at the same advertised version may represent different
bytes, which a locked restore must never adopt silently.

## Adversarial self-review and resulting corrections

These are competing review perspectives by the same assistant, NOT independent
GPT-6.1 Sol/xhigh reviews. Those workers were unavailable in this continuation.
External PR reviews and executable evidence must be reported separately.

| Challenge | Assessment and refinement |
| --- | --- |
| UX: "latest compatible is the industry standard" | Too strong. Go, NuGet and Maven differ; recommend a UAP default based on its workload, not a majority vote. |
| Compatibility: "follow Go, it is also minimal" | Incorrect equivalence. Distinguish graph minimum requirements from registry candidate ranking and upper bounds. |
| Security: "newest is safest" | Reject. Preserve eligibility, provenance, revocation and trust checks before ranking. |
| Architecture: "npm supports duplicates, so we should too" | Reject without native namespace/data/process isolation. Start with explicit shared packages and good conflicts. |
| Simplicity: "support all popular range syntaxes" | Reject. One documented subset beats inconsistent aliases and early migration debt. |
| Evidence: "docs prove all managers work this way on our host" | Reject. Only the recorded npm/pip versions were executed locally; the other rows are documentation/source research. |

## Scope and next decision

This report changes no resolver, lock schema, package identity or public API.
Retain existing lock and ownership contracts. Decide fresh candidate preference
separately after actual-fixture review; improve conflict explanations rather than
adding broad solver modes. The [MCP/runtime report](mcp-runtime-policy.md) records
what can be reused safely and where running code needs its own dependency locks.

## Primary sources (retrieved 2026-10-29)

- [S1: Agent Plugins specification](https://agent-plugins.org/specification).
- [U1: UAP Skills installation/source pins](https://github.com/777genius/universal-agent-plugins/blob/60967b8b6ca79245a61caf28895aa09c2c9d74a2/docs/AGENT_SKILLS_INSTALLER.md).
- [U2: UAP dependency environment contract](https://github.com/777genius/universal-agent-plugins/blob/60967b8b6ca79245a61caf28895aa09c2c9d74a2/docs/UAP_DEPENDENCIES.md).
- [U3: existing update-lifecycle PR #563](https://github.com/777genius/universal-agent-plugins/pull/563).
- [M1: npm install](https://docs.npmjs.com/cli/v11/commands/npm-install/) and [npm ci](https://docs.npmjs.com/cli/v11/commands/npm-ci/).
- [M2: pnpm resolutionMode](https://github.com/pnpm/pnpm.io/blob/main/docs/settings/other.md) and [install](https://pnpm.io/cli/install).
- [M3: Yarn dedupe](https://yarnpkg.com/cli/dedupe) and [immutable install](https://yarnpkg.com/cli/install).
- [M4: Bun install, lock and lifecycle policy](https://bun.com/docs/pm/cli/install).
- [M5: pip resolution](https://pip.pypa.io/en/stable/topics/dependency-resolution/) and [experimental lock](https://pip.pypa.io/en/stable/cli/pip_lock/).
- [M6: uv resolution](https://docs.astral.sh/uv/concepts/resolution/) and [locking/syncing](https://docs.astral.sh/uv/concepts/projects/sync/).
- [M7: Poetry basic usage](https://python-poetry.org/docs/basic-usage/).
- [M8: Cargo resolver](https://doc.rust-lang.org/cargo/reference/resolver.html).
- [M9: Go modules reference](https://go.dev/ref/mod).
- [M10: Composer basic usage](https://getcomposer.org/doc/01-basic-usage.md) and [CLI prefer-lowest](https://getcomposer.org/doc/03-cli.md).
- [M11: Bundler update guidance](https://bundler.io/guides/updating_gems.html) and [bundle update](https://guides.rubygems.org/command-reference/bundle-update/).
- [M12: Maven dependency mechanism](https://maven.apache.org/guides/introduction/introduction-to-dependency-mechanism.html).
- [M13: Gradle conflicts](https://docs.gradle.org/current/userguide/dependency_constraints_conflicts.html) and [locking](https://docs.gradle.org/current/userguide/dependency_locking.html).
- [M14: NuGet resolution](https://learn.microsoft.com/en-us/nuget/concepts/dependency-resolution).
- [M15: Nix flake lock](https://nix.dev/manual/nix/2.34/command-ref/new-cli/nix3-flake-lock.html).
- [A1: Microsoft APM dependencies](https://microsoft.github.io/apm/consumer/manage-dependencies/).
- [A2: Vercel Skills command contracts](https://github.com/vercel-labs/skills/blob/main/docs/commands.md).
- [A3: Skillshare project skills](https://github.com/runkids/skillshare/blob/main/website/docs/understand/project-skills.md).
