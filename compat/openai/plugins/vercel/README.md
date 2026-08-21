# Vercel

Community package for the official Vercel hosted MCP plugin for project, deployment, log, and documentation workflows through Vercel's remote MCP service.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add vercel --target codex
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Vercel.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://vercel.com/docs/agent-resources/vercel-mcp
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
