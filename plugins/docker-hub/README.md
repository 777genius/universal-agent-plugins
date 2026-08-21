# Docker Hub

Community package for the official Docker Hub MCP plugin for repository, image, and Docker Hub workflows through Docker's containerized stdio server.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add docker-hub --target codex
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Docker Hub.

- Component: MCP server
- Transport: `stdio`
- Upstream documentation: https://github.com/docker/hub-mcp
- Authentication: The portable configuration intentionally exposes public Docker Hub data only. Authenticated writes require client-specific secret configuration outside Agent Plugins 1.0.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
