---
title: "SDKs and integrations"
description: "Choose the Go hooks, Go detection, Go installer or TypeScript management SDK."
canonicalId: "page:sdk:index"
section: "sdk"
locale: "en"
generated: false
translationRequired: true
---

# SDKs and integrations

Use the SDKs when you are **embedding Universal Agent Plugins in another application** or writing typed hook handlers. Installing a plugin through the CLI is a different workflow; see [Use plugins](../use/).

## Choose the right SDK

| You want to… | Use | Language and entry point |
| --- | --- | --- |
| Handle an agent's native hook/event | [Go Hooks SDK](./go-hooks) | Go: `github.com/777genius/plugin-kit-ai/sdk` |
| Distinguish Claude Code from Codex in a shared hook executable | [Host detection](./host-detection) | Go: `sdk/hostdetect` |
| Embed package install/update/repair/remove directly in a Go host | [Go Installer SDK](./go-installer) | Go: `install/integrationctl/agentplugins/installer` |
| Discover agent profiles and manage installed plugins from a desktop/Node application | [TypeScript Management SDK](./typescript-management) | TypeScript: `universal-agent-plugins-management-sdk` and `/node` |

**Two different kinds of detection:** `hostdetect.Detect` identifies the host *invoking one hook* from its payload. `discoverTargets` (TypeScript SDK) inspects available *native client profiles* via the management engine. Neither grants automatic installation or host authentication.

## Availability and version boundaries

| Surface | Documented baseline | What it actually proves |
| --- | --- | --- |
| Go Hooks SDK | Published Go module `github.com/777genius/plugin-kit-ai/sdk@v1.3.0` | Typed runtime contract for named supported hooks; event-level maturity differs |
| Go host detection | Current `sdk/hostdetect` source; verify the exact module tag before using it | Bounded Claude/Codex hook-payload identification, not agent installation discovery |
| Go Installer facade | Current `install/integrationctl/agentplugins` source and its external sample | Process-local install/repair APIs; pin a reviewed source revision, do not assume a separate published module version |
| TypeScript Management SDK | Source package `0.1.1`; verify npm availability | Typed client for Management API v1, plus Node-owned engine; not a replacement for the Go engine |
| Native management engine | Released `agentplugins` CLI `0.1.95` or newer with `manage serve` and `manage discover` | These commands are available; individual operations need their own engine capability/version |

The TypeScript SDK's `checkUpdates` method exists in SDK 0.1.1 but **is not served by CLI 0.1.95 or older**. Select a newer qualified engine or a pinned source build before using it. A documented method is not evidence that every released engine has it.

## Features and where to find them

| Task | Read this | Important boundary |
| --- | --- | --- |
| Signed catalog, direct Git/archives, standalone Skills and project scope | [Install guide](../use/install) | Source trust and client support differ |
| `uap.toml`, `uap.lock`, export/import, update, repair and removal | [Maintenance](../use/manage) | Installed does not mean authenticated |
| Claude/Codex/Gemini/Qoder typed hook events | [Go Hooks](./go-hooks) | Event maturity varies by release |
| Identify the host invoking a hook | [Host detection](./host-detection) | Default JSON signals cover Claude and Codex |
| Prepare, confirm, apply, inspect, recover and migrate state | [Go Installer](./go-installer) | Explicit plan and profile authority |
| Native profile discovery, Skill preview/edit, operation tracking and update checks | [TypeScript Management](./typescript-management) | SDK methods need matching engine capabilities |
| Author a portable `plugin.json` package with Skills and MCP | [Build guide](../build/) | `author` tools are source-build-only |

**Choose by task.** The CLI installs for users; the TypeScript SDK manages
a private Go engine from Node/desktop; the Go installer facade embeds install
authority; the Hooks SDK handles callbacks inside native clients. None of these
operations alone proves that a remote MCP call succeeded.

## The three integration layers

1. **Hooks runtime:** Agent invokes your Go executable; it reads its native event and writes a typed response. This does not install packages.
2. **Installer engine:** Go owns package resolution, state, rollback/recovery, local profile authority and native activation. Embedding code must choose the exact targets and obtain explicit review/confirmation.
3. **Management client:** TypeScript uses the engine's authenticated local Management API; its `/node` helper owns a pinned engine process. The client itself does not manipulate native agent files.

Do not equate schema validation, package materialization, native agent discovery, service authentication and actual tool execution. They require separate evidence.

## Next steps

- [Write a Go hook](./go-hooks) with exact typed event handlers and maturity guidance.
- [Detect the invoking agent](./host-detection) safely without guessing from shared environment markers.
- [Embed the Go installer](./go-installer) with prepare → human review → apply → inspect/recover.
- [Embed plugin management in Node/TypeScript](./typescript-management) with capability checks and durable operation IDs.

### Source and reference

- [Go hook SDK](https://github.com/777genius/universal-agent-plugins/tree/main/sdk) and its [stability policy](https://github.com/777genius/universal-agent-plugins/blob/main/sdk/STABILITY.md)
- [Go installer facade](https://github.com/777genius/universal-agent-plugins/tree/main/install/integrationctl/agentplugins/installer)
- [TypeScript SDK README](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md) and [Management API contract](https://github.com/777genius/universal-agent-plugins/blob/main/contracts/management/README.md)

These are integration guides, not a claim that an archived generated API reference or source-only command ships in the current installer.
