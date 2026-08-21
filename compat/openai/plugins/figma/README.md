# Figma

Community package for the official Figma MCP plugin for design context, code-to-design workflows, and authenticated access to Figma Design, Make, and FigJam through Figma's remote MCP service.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add figma --target codex
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Figma.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://developers.figma.com/docs/figma-mcp-server/remote-server-installation/
- Authentication: Authentication is discovered and managed by the client. No credentials are bundled.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
