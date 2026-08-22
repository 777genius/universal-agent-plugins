# Sentry

Community package for the official Sentry hosted MCP plugin for human-in-the-loop debugging, issue triage, and incident workflows through Sentry's remote MCP service.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add sentry
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Sentry.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://mcp.sentry.dev
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
