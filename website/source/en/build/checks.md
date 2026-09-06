---
title: "Check a package without running it"
description: "Interpret conformance, static readiness, compatibility, and toolchain evidence separately."
canonicalId: "page:build:checks"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Check a package without running it

> **Prepared, unreleased authoring contract.** This page is a review draft on a
> non-deploying preparation branch. Future authoring examples require the accepted
> release; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

Use these future commands against the same exact package root after editing.
The example assumes `./review-helper` exists and contains root `plugin.json`.
An explicit path keeps the check independent of your current directory.

## Run the offline authoring loop

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

Equivalent future authoring executable commands:

```bash
plugin-kit-ai validate ./review-helper
plugin-kit-ai inspect ./review-helper
plugin-kit-ai test ./review-helper
```

Choose one entrypoint. Validate evaluates exact-root standard configuration and
authoring readiness. Inspect shows captured components and unresolved runtime
requirements. Test checks configuration, hygiene, Skills, and MCP statically.
It does not launch scripts, perform a handshake, or execute a tool fixture.

## Read the evidence layers

| Layer | What it can establish | What it cannot establish |
| --- | --- | --- |
| Conformance | Captured standard data satisfies the embedded schemas/profile. | A useful or working plugin in every agent. |
| Host safety | Selected input meets bounded filesystem/host checks. | Authorization to install or execute it. |
| Readiness | Available static package evidence is sufficiently complete for the checked policy. | Publication or platform acceptance. |
| Compatibility | Static adapter support for selected clients and components. | Those clients are installed, activated, or signed in. |
| Toolchain | Bounded project/native-file and executable-metadata evidence. | Successful dependency resolution or server startup. |
| Runtime | The report identifies runtime evaluation status. | Static jobs do not supply execution evidence. |

Do not collapse these into a single “works everywhere” badge. A conforming
manifest may coexist with incomplete component or host evidence, so conformance
alone need not mean a successful overall readiness result.

## Select clients explicitly

```bash
agentplugins author compat ./review-helper --target codex,claude
agentplugins author inspect ./review-helper --target codex,claude
```

The equivalent forms begin `plugin-kit-ai compat` and `plugin-kit-ai inspect`.
Compat requires distinct comma-separated client IDs. Use explicit IDs rather
than `all` or historical target names such as `codex-runtime`. These are static
adapter checks; installed clients are not required. Inspect accepts an optional
target selection when you want client-specific component findings.

## Inspect project and engine evidence

```bash
agentplugins author doctor ./review-helper
agentplugins author capabilities
```

```bash
plugin-kit-ai doctor ./review-helper
plugin-kit-ai capabilities
```

Doctor reads captured native files and executable metadata without processes or
network. Capabilities takes no package path and reports embedded schemas,
profiles, client metadata, and implemented commands. It describes this engine;
it does not certify a package or prove the engine has been publicly released.

## Capture a reviewable report

```bash
agentplugins author validate ./review-helper --format json
plugin-kit-ai inspect ./review-helper --format json
```

Human output is `--format human`; `--no-color` is supported. Package commands can
use `--include-root` to disclose the explicitly selected root in their report.
Avoid disclosing private paths when sharing diagnostics. Capabilities has no
root to include.

Read commands validate, inspect, test, compat, and doctor accept
`--release-policy` for bounded release hygiene. This is not publication approval.
Skills subcommands have their own smaller flag surface; do not infer that every
package flag also applies to `skills validate`.

## Keep flags and responsibilities separate

Installer `--dry-run` and security/scope policy are not authoring flags. Authoring
init is a real local file creation job, even when another root's help lists
installer flags. Legacy `--strict`, runtime fixture flags, and overwrite flags
are not shortcuts to stronger v2 checks.

`agentplugins validate ./review-helper` is the separate installer validation
job. `agentplugins doctor` diagnoses managed installation state. Neither is an
alias for the authoring commands above.

If a report fails or is incomplete, fix the named input or preserve the limit in
the handoff. Do not relabel “not evaluated” as a passing runtime test.
Continue with the [local installer planner handoff](./handoff).
