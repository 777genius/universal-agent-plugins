# Supabase

Community package for the official Supabase MCP integration for development and database operations from agent workflows.

<!-- agentplugins-install:start -->
## Install

```bash
npx universal-agent-plugins add supabase
```
<!-- agentplugins-install:end -->

This is an independent community package for [Agent Plugins 1.0](https://agent-plugins.org/specification). It is not an endorsement or an official package from Supabase.

- Component: MCP server
- Transport: `streamable-http`
- Upstream documentation: https://supabase.com/docs/guides/ai-tools/mcp
- Authentication: Authentication and project scoping are client-managed. No access token or project reference is stored in this package.

Review the server's tools, scopes, and write capabilities before enabling it. Agent Plugins 1.0 standardizes packaging, not permissions or sandboxing.

Supabase's documentation says its hosted MCP server is for development and
testing, not production data. Use project scoping, read-only mode, and feature
groups from the upstream guide where your client supports them.
