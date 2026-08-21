# Notion

Community package for the official Notion hosted MCP plugin for user-authorized workspace access. Search pages, read docs, edit content, and work with Notion knowledge directly from AI agents.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add notion --target codex
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Notion.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://developers.notion.com/docs/get-started-with-mcp
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
