# Cloudflare Bindings

Community package for the official Cloudflare Workers Bindings MCP integration for storage, AI, and compute primitives.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add cloudflare-bindings --target codex
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Cloudflare Bindings.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://developers.cloudflare.com/agents/model-context-protocol/mcp-servers-for-cloudflare/
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
