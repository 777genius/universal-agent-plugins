# Comparison sources and limits

This supports the short [README comparison](../README.md#compare-with-other-tools).
It compares implemented, documented behavior at fixed versions, rather than
roadmaps, repository popularity or a count of green checks. Inspection covered
official documentation and the relevant source paths; it is not a runtime
certification of every client, platform or package.

## Versions checked

Checked on **2026-10-24**.

| Project | Compared release | Source revision |
| --- | --- | --- |
| UAP | [Agentplugins 0.1.90](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.90) | [`008370d`](https://github.com/777genius/universal-agent-plugins/tree/008370dd6767fecd19ebf25902f099b102226ba5) |
| UAP hook SDK | [`sdk/v1.1.0`](https://github.com/777genius/universal-agent-plugins/releases/tag/v1.1.0) | [`c211449`](https://github.com/777genius/universal-agent-plugins/tree/c2114493c8c0e55bf57d42f8bb9624a8640e87a4) |
| Skillshare | [v0.26.4](https://github.com/runkids/skillshare/releases/tag/v0.26.4) | [`a5477eb`](https://github.com/runkids/skillshare/tree/a5477ebe6b0975c3ecf479b417b4db9ae30e7609) |
| Vercel Skills | [v1.7.4](https://github.com/vercel-labs/skills/releases/tag/v1.7.4) | [`a80e4c9`](https://github.com/vercel-labs/skills/tree/a80e4c9d12d607631249d4d7d059f835d4b2f13a) |
| Microsoft APM | [v0.35.0](https://github.com/microsoft/apm/releases/tag/v0.35.0) | [`17a2fc4`](https://github.com/microsoft/apm/tree/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e) |

UAP's current main was also inspected at
[`62fb629`](https://github.com/777genius/universal-agent-plugins/tree/62fb629f3306a46296fda492f00bc9ced62b007e).
Its README, dependency guide, Skill installer guide, SDK README and installer
command root match the compared CLI release. The Go SDK is a separately consumed
module, not a claim that the installer exposes every authoring command.

## What the short rows mean

### One Agent Plugins 1.0 package across clients

This asks whether the tool adapts the supported components of the **same
canonical Agent Plugins 1.0 package** for clients with different formats. It
does not ask whether a tool supports any plugin format, or whether it can install
an already compatible package in several clients.

- **UAP:** reads the standard package and projects supported skills/MCP into
  client formats. For example, the [Codex projection](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/install/integrationctl/agentplugins/clients/codex/project.go)
  and [Claude projection](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/install/integrationctl/agentplugins/clients/claude/project.go)
  produce different native layouts. This is component-specific support, not
  automatic portability of all hooks, commands, transports or authentication.
  Some clients still require activation; see the [client delivery table](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/README.md#supported-clients).
- **Skillshare:** has a real native plugin lifecycle. It recognizes the standard
  root manifest for compatible targets, but requires target-native formats where
  needed; Claude requires `.claude-plugin/plugin.json`. It does not generally
  translate one target's package into another's. See the [target matrix and boundaries](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/website/docs/reference/commands/plugin.md#compatibility-and-boundaries)
  and [manifest classification](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/internal/plugin/source_manifest.go).
- **Vercel Skills:** reads Claude plugin manifests to discover and group their
  **skills**. It does not install the whole native plugin or configure its MCP
  servers. See [plugin discovery](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/src/plugin-manifest.ts)
  and the [CLI command surface](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/src/cli.ts).
- **APM:** recognizes canonical Agent Plugins 1.0 and registers the complete
  package natively for **Copilot**. It does not project that recognized package
  into loose components for other clients. APM-native and Claude-format packages
  have broader cross-client deployment, and there is a separate Codex-native
  bundle route. See [package types](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/docs/src/content/docs/reference/package-types.md)
  and [canonical plugin admission](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/src/apm_cli/install/native_plugin_admission.py).

### Typed hook SDK

UAP provides a separately released [Go SDK](https://github.com/777genius/universal-agent-plugins/blob/c2114493c8c0e55bf57d42f8bb9624a8640e87a4/sdk/README.md)
with typed events, responses, runtime dispatch and separate client registrars.
The checked `sdk/v1.1.0` module covers Claude, Codex and Gemini with documented
stable/beta event boundaries. Newer APIs in the current source tree are not
attributed to that SDK release. It is not a promise to write one handler and
obtain identical semantics in every agent. The [installer SDK](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/install/integrationctl/agentplugins/installer/README.md)
is a separate embeddable lifecycle API documented at the inspected source snapshot,
with explicit adapter and operation limits; it is not part of `sdk/v1.1.0`.
Neither SDK claim implies that the current npm/native installer exposes the
[`agentplugins author` commands](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/README.md#build-plugins).

The other tools' documented authoring contracts do not provide a comparable
typed hook SDK. This does **not** mean they lack hooks:

- [Skillshare hooks](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/website/docs/reference/commands/hooks.md)
  retain per-agent native event maps and supplied extension code.
- [Vercel's skill compatibility table](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/docs/agents.md#L92-L102)
  includes agent-supported hooks in skill frontmatter; its installer is still
  scoped to skills.
- [APM hooks and commands](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/docs/src/content/docs/producer/author-primitives/hooks-and-commands.md)
  use descriptors, scripts and target-specific mappings.

### Skills and MCP setup

All four install skills for multiple agents. UAP, Skillshare and APM also write
MCP configuration. Support varies by client and transport; an available adapter
does not prove every component is usable in that client.

Sources: [UAP clients](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/README.md#supported-clients),
[Skillshare MCP](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/website/docs/reference/commands/mcp.md),
[Vercel commands](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/docs/commands.md),
[APM target matrix](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/docs/src/content/docs/reference/targets-matrix.md).

### Local browser UI

UAP 0.1.90's [release notes](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.90)
explicitly ship the bundled Skill picker: browse, preview, select, review targets
and install. Skillshare has a broader [local management dashboard](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/website/docs/reference/commands/ui.md).
The compared Vercel and APM CLIs have no equivalent local browser management
interface. Vercel's [skills.sh](https://skills.sh/) is a useful public discovery
site; the table's cross does not deny its existence or its installation guidance.

### Lockfiles are not all the same

| Tool | Recorded state | Restore boundary |
| --- | --- | --- |
| UAP | `uap.lock`: immutable commits, package/Skill digests and targets | 0.1.90 explicitly ships saved Skill entries. The complete dependency environment, locked sync and transfer workflow remain labelled unreleased in the repository's support guidance. |
| Skillshare | `.skillshare/skills.lock.json`: Git commits for project skills | Project install uses matching source pins. Missing pins or changed sources do not trigger a frozen-mode failure; local paths have no Git pin. |
| Vercel Skills | `skills-lock.json`: sources, optional refs, paths and hashes; a separate global lock tracks installs | `experimental_install` reconstructs sources and writes fresh hashes. A moving ref can move; an explicit full commit SHA can pin Git content. There is no frozen hash-enforcement contract. |
| APM | `apm.lock.yaml`: resolved commits, content/deployed-file hashes and ownership | `apm install --frozen` enforces its lock contract without rewriting it. This does not imply offline installation or rollback of every target side effect. |

Sources: [UAP dependency contract](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/docs/UAP_DEPENDENCIES.md),
[Skillshare project lock](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/website/docs/understand/project-skills.md#L213-L245),
[Vercel lock schema](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/src/local-lock.ts),
[Vercel restore test](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/tests/install-from-lock.test.ts),
[APM install contract](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/docs/src/content/docs/reference/cli/install.md).

### Declared skill/plugin dependencies

- **UAP:** the [dependency environment](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/docs/UAP_DEPENDENCIES.md)
  resolves package-declared dependencies, checks compatible versions, assigns
  inherited targets and records the graph. Its complete workflow is still
  labelled **unreleased**. Shipped Skill recording is not evidence that every
  dependency/transfer scenario is release-qualified.
- **Skillshare:** project installation reads an explicit [resource list](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/internal/config/project.go)
  through [manifest installation](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/internal/install/install_config.go).
  No tool-managed transitive resource graph was found. Skillshare also
  [delegates Pi/npm package installation to Pi](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/internal/plugin/pi_npm.go).
  That can install npm dependencies, but is distinct from resolving declared
  dependencies between skills/plugins across clients.
- **Vercel Skills:** [experimental npm skill packs](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/docs/commands.md#skills-experimental_sync)
  can declare other packs and remote skills. [Sync traverses those declarations](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/src/sync.ts)
  with conflict rules; npm/pnpm or another package manager resolves the installed
  package versions. Arbitrary Git skills do not gain a recursive plugin graph.
- **APM:** [transitive dependency resolution](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/docs/src/content/docs/consumer/manage-dependencies.md)
  and Git tag ranges are released. Its [immutable-ref checks](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/src/apm_cli/deps/immutable_requirements.py)
  are not a general proof that all intersecting semver ranges are solved
  together; [single-version hoisting](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/src/apm_cli/deps/apm_resolver.py)
  has its own policy.

### Project-local installation

UAP's [compared CLI rejects project scope](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/cli/internal/agentpluginscli/root.go).
[Project installation PR #472](https://github.com/777genius/universal-agent-plugins/pull/472)
was still open at the check date. Likewise, [source transport PR #473](https://github.com/777genius/universal-agent-plugins/pull/473)
is not counted as released support for arbitrary Git, private SSH or HTTP archives.

[Skillshare project mode](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/website/docs/understand/project-skills.md),
[Vercel project installs](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/docs/commands.md)
and [APM project installs](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/docs/src/content/docs/reference/cli/install.md)
are available. This row means project-local skills/setup is supported, not that
every native plugin/MCP adapter supports both scopes. In particular, Skillshare's
[native plugin scope matrix](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/website/docs/reference/commands/plugin.md#target-support)
has several user-only targets.

## Other useful strengths

- **UAP:** standard-package adaptation, typed hook authoring and an embeddable
  installer. Its [owned-file lifecycle](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/docs/REMOVAL_SAFETY.md)
  is useful, but ownership-aware safety is not exclusive to UAP.
- **Skillshare:** a management UI, configuration sync/adoption, backups and a
  [skill audit engine](https://github.com/runkids/skillshare/blob/a5477ebe6b0975c3ecf479b417b4db9ae30e7609/website/docs/understand/audit-engine.md).
- **Vercel Skills:** [skills.sh](https://skills.sh/) discovery, a broad
  [agent target list](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/docs/agents.md),
  and [Git/private/local/HTTP archive sources](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/docs/commands.md).
- **APM:** released project dependencies and frozen installs, [instruction compilation](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/docs/src/content/docs/producer/compile.md)
  and [organization policies](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/docs/src/content/docs/enterprise/apm-policy.md).

No security superiority or identical safety guarantees are inferred from this
table. UAP has [automated package assessment](https://github.com/777genius/universal-agent-plugins/blob/008370dd6767fecd19ebf25902f099b102226ba5/cli/cmd/agentplugins/compose.go),
Skillshare has audit gates, Vercel presents [advisory partner assessments](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/src/telemetry.ts),
and APM has a documented [security and governance model](https://github.com/microsoft/apm/blob/17a2fc4dfff238c7d93e55cce6ef42ffa1db928e/docs/src/content/docs/enterprise/security.md).
Their coverage, failure behavior and trust assumptions differ.

## Updating the comparison

Recheck the pinned documentation and implementation before changing a cell.
Update the date and release snapshots together. An open PR, a discovered package
format, a source-only command or a supported-agent logo is not by itself evidence
of a released installation capability. Keep material qualifications beside the
table and link detailed semantics here.
