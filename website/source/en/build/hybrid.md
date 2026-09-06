---
title: "Build a hybrid package"
description: "Combine one Skill and one MCP configuration under a single package identity."
canonicalId: "page:build:hybrid"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Build a hybrid package

> **Prepared, unreleased authoring contract.** This page is a review draft on a
> non-deploying preparation branch. Future authoring examples require the accepted
> release; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

A hybrid is useful when instructions explain how and when to use tools. Both
components live under one root manifest. A client may support them differently,
so keep per-component compatibility findings visible in the handoff.

## Choose the MCP side explicitly

The hybrid flag is `--mcp-template`. Choose `mcp-remote` or `mcp-stdio`, then supply
that template's required inputs. A hybrid does not infer a transport from the
URL, local files, or installed runtimes.

Future remote hybrid example:

```bash
agentplugins author init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Guide research using the team MCP service' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

Equivalent future authoring executable example; choose one:

```bash
plugin-kit-ai init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Guide research using the team MCP service' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

The URL is an example placeholder, not a promised service. Replace it with your
actual endpoint. Read the [remote MCP requirements](./mcp-remote).

## Or choose a local process

For a different, absent destination, the future stdio variant is:

```bash
agentplugins author init ./local-research \
  --template hybrid \
  --name local-research \
  --description 'Guide research using local Node tools' \
  --skill-name research-guide \
  --mcp-template mcp-stdio \
  --runtime node
```

The equivalent prefix is `plugin-kit-ai init`; every argument stays the same.
Do not add `--url` to the stdio variant. Review the
[stdio prerequisite and runtime limits](./mcp-stdio).

## Connect instructions to tools

A remote hybrid has this core layout:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

README and `.gitignore` are also created. A stdio hybrid adds the Node source and
package files described in the stdio journey. Review `SKILL.md` so that it names
when to use the tools, what input is needed, and how to report missing access.
Do not tell the agent to treat Skill text as an authorization grant.

## Check both components

```bash
agentplugins author skills validate ./research-helper
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper --target codex,claude
agentplugins author test ./research-helper
agentplugins author compat ./research-helper --target codex,claude
```

For the local variant, select `./local-research` instead. Preserve component-level
findings: a client supporting the Skill does not automatically support the MCP
transport or activate the service. A combined package does not erase that limit.

Continue with [extra Skills](./skills) or [handoff](./handoff).
