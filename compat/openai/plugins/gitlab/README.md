# GitLab

Portable Agent Plugins package for GitLab MCP. Inspect projects, issues, merge requests, and related DevOps workflows through GitLab's HTTP server.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add gitlab
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from GitLab.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://gitlab.com
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
