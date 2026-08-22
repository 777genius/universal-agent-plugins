# Cloudflare API

Community package for the official Cloudflare API MCP plugin for token-efficient access to the Cloudflare API through Cloudflare's hosted Code Mode server.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add cloudflare
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Cloudflare API.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://developers.cloudflare.com/agents/model-context-protocol/mcp-servers-for-cloudflare/
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
