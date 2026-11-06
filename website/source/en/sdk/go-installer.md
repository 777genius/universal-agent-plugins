---
title: "Go Installer SDK"
description: "Embed the Go installer facade with explicit plans, confirmation, discovery and recovery."
canonicalId: "page:sdk:go-installer"
section: "sdk"
locale: "en"
generated: false
translationRequired: true
---

# Embed the Go Installer SDK

Use the process-local Go installer facade when your **Go host** must own the complete package lifecycle: prepare, install, update, repair, refresh a projection, remove, inspect and recover. This is a different package from the [Go Hooks SDK](./go-hooks) and from the [TypeScript Management SDK](./typescript-management).

## Source and availability

The public facade lives at:

```text
github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer
```

Its module is `github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins`. The [current source README](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md) and [external consumer example](https://github.com/777genius/universal-agent-plugins/tree/main/install/integrationctl/agentplugins/installer/example) are the authoritative implementation references. **Pin a reviewed module revision and its dependencies**; do not assume the Go installer facade uses the hook module's `v1.3.0` version or has an independently verified release.

## Construction and discovery

You must choose an **absolute state root** and explicitly register allowed client adapters. A missing registry is rejected. `New` validates configuration but does **not** create directories or run a client. `Discover` reports metadata for registered adapters; an executable being present does not prove activation.

```go
registry, err := clients.NewRegistry(codex.New(), claude.New())
if err != nil { return err }

engine, err := uapinstaller.New(uapinstaller.Config{
    StateRoot: "/absolute/private/uap-state",
    Registry: registry,
})
if err != nil { return err }

available := engine.Discover()
_ = available // inspect scopes, capabilities and detected executables
```

The snippet assumes imports from the [complete sample](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go). It is an **illustrative fragment**, not a complete executable. Add the actual runner/helper, profile and trusted-source policy for the component types your selected adapter needs.

## Lifecycle: prepare → review → apply → inspect

```go
req := uapinstaller.Request{
    Operation:        uapinstaller.OpInstall,
    PackageRoot:      "/absolute/reviewed/package",
    ClientID:         "codex",
    ClientConfigRoot: "/absolute/codex-profile",
    ClientExecutable: "/absolute/path/to/codex",
    InstallationID:   "00000000-0000-4000-8000-000000000099",
    OperationID:      "host-generated-unique-operation-id",
}
prepared, err := engine.Prepare(ctx, req)
if err != nil { return err }
defer prepared.Close()

plan := prepared.Plan()
// Show plan, targets, components, warnings and digests to the user.
// Only after explicit approval, with immutable prepared inputs:
result, err := engine.Apply(ctx, prepared, uapinstaller.Decision{Confirmed: true})
if err != nil { return err }
_ = result

view, err := engine.Inspect(ctx)
if err != nil { return err }
if view.Recovery.Required {
    _, err = engine.Recover(ctx, view)
    if err != nil { return err }
}
```

This also assumes `ctx`, `engine`, and the [sample imports](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go). Do not copy it into production without showing and approving **the actual immutable plan**. `Prepare` makes an immutable review handle. `Apply` rechecks authority and can fail with a changed plan. Close abandoned handles. `Inspect` is read-only; `Recover` needs its **exact typed observation** and refuses changed state or scope.

## Ownership and safety

- `Config.StateRoot` and `ClientConfigRoot` are explicit authority. `HOME`, `CODEX_HOME` and `CLAUDE_CONFIG_DIR` are **not** implicit substitutes for request profile paths. Select the project root explicitly for project-scoped operation.
- For local package bytes, use an explicit `Assess` callback or a digest-bound `Request.Assessment`. `TrustedLocalPackages: true` means your host deliberately pre-authorized bundled bytes; it is **not** a generic skip-security switch.
- `Config.HelperExecutable`, `Runner` and `EnableNativeObserver` are needed according to selected delivery path. A client adapter registered in `Registry` does not imply every component, scope or native host path is supported.
- `OpInstall`, `OpUpdate`, `OpRepair`, `OpRefreshProjection` and `OpRemove` have separate plans. Multi-target requests use `Request.Targets`, not a fictional `install-group` verb. Group OpenCode install/update/repair and broad removal have documented limits.
- A missing existing binding cannot be repaired or updated by silently treating the request as install. Native activation, authentication and real tool invocation remain separate checks.

## Recovery and schema migrations

Inspect before starting new work and surface pending operations, journals and native intents. Review `PrepareStateMigration(ctx)` and `ApplyStateMigration(ctx, plan)` when opening older state; migration is an explicit, digest-bound, backed-up step, **not** automatic state rewriting. Recover old pending work with the corresponding prior tool before upgrading. A failed or interrupted apply can have partial effects, so never blindly retry with different identities.

See the [installer README](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md) for per-binding revisions, group support, typed errors and native-removal boundaries.

[TypeScript Management SDK](./typescript-management) · [Install plugins with the CLI](../use/install)
