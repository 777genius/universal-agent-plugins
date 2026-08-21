# HubSpot CRM

Community package for the official HubSpot remote MCP integration for beta, read-only CRM object access through HubSpot's hosted MCP server.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add hubspot-crm --target codex
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from HubSpot CRM.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://developers.hubspot.com/mcp
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
