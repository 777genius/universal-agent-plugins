# Chrome DevTools

Community package for Chrome DevTools MCP. Inspect pages, automate flows, analyze performance, and debug browser state.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add chrome-devtools --target codex
```
<!-- agentplugins-install:end -->

This package is independently assembled by 777genius from configuration anchored to ChromeDevTools/chrome-devtools-mcp at commit `774d78f5eef5e610407a0c92fa6ec5ed74b027e8`. It is not authored, published, or endorsed by ChromeDevTools or Google.

- Component: MCP server
- Transport: `stdio`
- Runtime dependency: `npx` and exact package `chrome-devtools-mcp@1.7.0`
- Upstream source: https://github.com/ChromeDevTools/chrome-devtools-mcp
- Authentication: No service credential is declared; the launched browser controls its own session.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.
