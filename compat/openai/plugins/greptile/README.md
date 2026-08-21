# Greptile

Community package for the official Greptile MCP integration for repository search and code intelligence tooling.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add greptile --target codex
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Greptile.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://greptile.com
- Authentication: Authentication is client-managed. No API key or Authorization header is stored in this package.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
