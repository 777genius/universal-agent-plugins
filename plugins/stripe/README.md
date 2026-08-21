# Stripe

Community package for the official Stripe hosted MCP plugin for payments, billing, customer, and documentation workflows through Stripe's remote MCP service.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add stripe --target codex
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Stripe.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://docs.stripe.com/mcp
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
