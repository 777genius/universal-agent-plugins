# Heroku

Community package for the official Heroku hosted MCP plugin for apps, add-ons, logs, Postgres, and operational workflows through Heroku's remote MCP service.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add heroku
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Heroku.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://devcenter.heroku.com/articles/heroku-remote-mcp-server
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
