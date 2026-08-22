# Atlassian

Community package for the official Atlassian Rovo MCP plugin for Jira, Confluence, and Compass workflows through Atlassian's hosted remote MCP service.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add atlassian
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Atlassian.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://support.atlassian.com/atlassian-rovo-mcp-server/docs/getting-started-with-the-atlassian-remote-mcp-server/
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
