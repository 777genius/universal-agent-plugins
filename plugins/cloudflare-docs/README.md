# Cloudflare Docs

Community package for Cloudflare Docs MCP. Search current Cloudflare documentation through Cloudflare's hosted documentation server.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add cloudflare-docs --target codex
```
<!-- agentplugins-install:end -->

This package is independently assembled by 777genius from configuration anchored to cloudflare/mcp-server-cloudflare at commit `0c51a6fbcf9a2fae80120287e8238fb947cdc2df`. It is not authored, published, or endorsed by Cloudflare.

- Component: MCP server
- Transport: `streamable-http`
- Endpoint: `https://docs.mcp.cloudflare.com/mcp`
- Upstream source: https://github.com/cloudflare/mcp-server-cloudflare/tree/main/apps/docs-ai-search
- Authentication: No credential is declared by this package.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
