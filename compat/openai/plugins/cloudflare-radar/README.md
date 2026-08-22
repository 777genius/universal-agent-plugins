# Cloudflare Radar

Community package for the official Cloudflare Radar MCP plugin for internet telemetry, traffic trends, and network intelligence through Cloudflare's hosted Radar server.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add cloudflare-radar
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Cloudflare Radar.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://developers.cloudflare.com/agents/model-context-protocol/mcp-servers-for-cloudflare/
- Authentication: The service exposes public telemetry, but the hosted MCP endpoint requires client-managed Cloudflare OAuth.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
