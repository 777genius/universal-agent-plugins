---
title: "Historical plugin-kit-ai v1, baseline 1.2.4"
description: "Version-pinned context for preserved YAML, runtime, SDK, and publication references."
canonicalId: "page:legacy:v1:index"
section: "legacy"
locale: "en"
generated: false
translationRequired: true
---

# Historical plugin-kit-ai v1, baseline 1.2.4

> **Prepared, unreleased authoring contract.** This public page documents a reviewed
> future authoring workflow. The commands shown here are not available in current
> releases; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

This index preserves a way to understand existing projects. Its command baseline
is tag **v1.2.4**, resolving to commit
`9beca10448ac50fbe526a52101d1433a12471980`, rather than the broader command tree
on the current documentation branch.

**Project migration is not available in v2 yet. Maintain legacy projects using
the v1 1.2.4 command set.** A version pin records historical behavior; it does not
promise a second maintained YAML engine or a new support commitment.

## Choose the right context

| What you have | Where to go |
| --- | --- |
| A new root `plugin.json` package | [Prepared Build journey](/en/build/). |
| A package to install into an agent | [Use plugins](/en/use/). |
| `plugin/plugin.yaml`, generated targets, or launcher source | Use the historical baseline and retained references below. |
| A plan to convert a legacy project automatically | Migration is unavailable in v2; preserve the project. |

Do not run new Build commands against a legacy directory expecting conversion.
Standard authoring has no YAML fallback, temporary conversion, or native hidden
manifest fallback.

## Exact historical command context

The tagged [root command source](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai/cmd/plugin-kit-ai/root.go)
registers init, bootstrap, doctor, dev, test, export, bundle, generate, import,
inspect, compat, publish, publication, normalize, validate, capabilities,
install, integrations, and version. Skills registration is visible separately in
the tagged [Skills source](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai/cmd/plugin-kit-ai/skills.go).
These are historical names, not the v2 command navigation.

The tagged [init source](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai/cmd/plugin-kit-ai/init.go)
describes `online-service`, `local-tool`, and `custom-logic`, with authored source
under `plugin/`. Runtime and platform switches there have historical meanings.
For example, v1 test exercises launcher fixtures, while prepared v2 test is a
static package check. The same command name does not imply the same behavior.

Use version 1.2.4 deliberately when maintaining those projects; avoid unversioned
or latest-channel historical installation snippets. This index does not install
or replace any executable. Package-wrapper development version placeholders in
the tagged source are not the historical release version.

## Retained explanations

These existing site pages remain intact. They are linked as legacy/support
explanations; their current-branch wording is not a byte-for-byte v1.2.4 snapshot.
Use the exact tagged sources above when a command detail depends on the version.

| Topic | Retained reading |
| --- | --- |
| YAML project and generated outputs | [Managed project model](/en/concepts/managed-project-model), [historical authoring workflow](/en/reference/authoring-workflow). |
| Runtime and target selection | [Choosing runtime](/en/concepts/choosing-runtime), [target model](/en/concepts/target-model). |
| Node and Python launcher projects | [Node/TypeScript runtime](/en/guide/node-typescript-runtime), [Python runtime](/en/guide/python-runtime). |
| SDK and runtime API context | [Existing API index](/en/api/); these generated references are not the new authoring CLI contract. |
| Bundle and publication explanations | [Bundle handoff](/en/guide/bundle-handoff), [publishing guide](/en/guide/how-to-publish-plugins). |
| Historical changes | [Existing release records](/en/releases/). |

## Pinned design and support sources

- [1.2.4 YAML specification](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/docs/PLUGIN_YAML_V1_SPEC.md).
- [1.2.4 authoring documentation](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai/README.md).
- [1.2.4 SDK README](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/sdk/README.md).
- [1.2.4 Node runtime README](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/npm/plugin-kit-ai-runtime/README.md).
- [1.2.4 Python runtime README](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/python/plugin-kit-ai-runtime/README.md).
- [1.2.4 publication design](https://github.com/777genius/universal-agent-plugins/blob/9beca10448ac50fbe526a52101d1433a12471980/docs/PUBLISH_LAYER_SPEC.md).
- [1.2.4 repository tree](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980).

The repository's 1.2.4 release note is dated 2026-05-24. Historical release
records may describe planned downstream publication; they alone do not prove a
channel is currently available. SDK and runtime helper packages also have their
own versions, independent of any later authoring CLI major version.

## Preservation is deliberate

Useful YAML implementation, dependencies, tests, examples, and design docs stay
in their existing locations. Removing a command from the standard authoring
surface does not authorize deleting its service or documentation. Unresolved
capabilities remain preserved outside the standard dependency graph.

This index does not copy all historical pages, bulk-migrate examples, or advertise
a maintained parallel engine. Future source removal requires an inventory and
explicit owner acceptance. Public navigation and redirects need separate review
before this preparation can be activated.
