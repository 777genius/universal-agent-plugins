---
title: "Build a remote MCP package"
description: "Describe an existing MCP endpoint without confusing configuration with connectivity."
canonicalId: "page:build:mcp-remote"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Build a remote MCP package

> **Prepared, unreleased authoring contract.** This public page documents a reviewed
> future authoring workflow. Future authoring commands described here are not available in current
> releases; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

Choose remote MCP when the server already exists and the package should describe
how an agent connects to it. Authoring records the URL without contacting it.
You remain responsible for knowing the service's transport and authentication
requirements before handing the package to users.

## Supply the endpoint explicitly

The examples use `https://mcp.example.com/mcp` as a documentation placeholder.
Replace it with your service's real MCP endpoint when preparing your own package.
The authoring command accepts the URL as configuration; it does not verify that
the address serves MCP or that your account can use it.

Future installer-hosted authoring spelling:

```bash
agentplugins author init ./docs-service \
  --template mcp-remote \
  --name docs-service \
  --description 'Connect to the team documentation MCP service' \
  --url https://mcp.example.com/mcp
```

Equivalent future authoring executable spelling; run only one:

```bash
plugin-kit-ai init ./docs-service \
  --template mcp-remote \
  --name docs-service \
  --description 'Connect to the team documentation MCP service' \
  --url https://mcp.example.com/mcp
```

Use an absent destination under an existing parent. The template requires an
absolute HTTPS URL, or HTTP on loopback. Credentials, fragments, and unresolved
URL tokens are not accepted template inputs. Do not put a password in the URL.

## Understand the files

```text
docs-service/
  plugin.json
  mcp.json
  README.md
  .gitignore
```

The root manifest owns package identity. `mcp.json` describes the server under
`mcpServers`; this template uses `type: streamable-http` and the supplied URL.
It does not generate a server or add a Skill. Use [hybrid](./hybrid) if the
package also needs instructions that explain when to call its tools.

Review the README and document the endpoint's purpose, account prerequisites,
and where users obtain authentication guidance. Keep secrets outside the package.
A URL in a conforming file is not evidence of a live service.

## Check configuration and target support

```bash
agentplugins author validate ./docs-service
agentplugins author inspect ./docs-service --target codex,claude
agentplugins author test ./docs-service
agentplugins author compat ./docs-service --target codex,claude
agentplugins author doctor ./docs-service
```

The same arguments work with the future `plugin-kit-ai` prefix. `inspect` can
show components and unresolved requirements. `compat` evaluates the selected
clients' static adapter support; neither command needs those clients installed.
Doctor does not contact the endpoint.

## Keep the remaining work visible

The author's static evidence should identify the transport and intended clients.
The receiving installer still needs to evaluate installation policy and show
activation instructions. The user may need to sign in, approve access, or start
a new client session. Runtime tool discovery and calls need separate evidence.

If static checks fail, correct the captured configuration first. If the endpoint
is unreachable later, investigate the service or client connection instead of
interpreting another successful static test as a connectivity check.

Continue with [report interpretation](./checks) and the [planner handoff](./handoff).
