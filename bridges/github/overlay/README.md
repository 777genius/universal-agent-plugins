# GitHub

Community package for GitHub MCP Server. Work with repositories, issues, pull requests, reviews, and code search through GitHub's hosted MCP endpoint.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add github --target codex
```
<!-- agentplugins-install:end -->

This package is independently assembled by 777genius from configuration anchored to github/github-mcp-server at commit `fcdd664099f957c4a7dc183d9381cef191e8c8a9`. It is not authored, published, or endorsed by GitHub.

- Component: MCP server
- Transport: `streamable-http`
- Endpoint: `https://api.githubcopilot.com/mcp/`
- Upstream source: https://github.com/github/github-mcp-server
- Authentication: GitHub manages authentication for the hosted endpoint; no credential is embedded in this package.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
