# Three optional plugins to try first

These examples avoid account credentials. Choose one: they are independent
alternatives, not sequential steps. The CLI detects Codex, Cursor,
GitHub Copilot/VS Code, and Kiro; if several are installed, choose one when
prompted.

Agent Code Navigator is skills-only. Cloudflare Docs uses a public remote MCP
server and also has a registered ChatGPT development binding. Docker Hub uses a
digest-pinned container and needs Docker. Context7 and Chrome DevTools stay out
of this starter list until their current distributions satisfy the Directory's
runtime-closure and materialization gates.

The registered Cloudflare Docs personal app passed Plugins UI discovery,
manual activation, and read-only runtime. The repository package separately
passed marketplace ingestion and official manager installation; package-routed
ChatGPT Work runtime remains unproved.

## 1. Agent Code Navigator

Install:

```bash
npx universal-agent-plugins add agent-code-navigator
```

Try:

```text
Map this sandbox repository's architecture and explain which search tool you use for each claim.
```

Expected: the agent loads the routing and architecture-map skills without
starting an MCP server or modifying the repository.

## 2. Cloudflare Docs

Install:

```bash
npx universal-agent-plugins add cloudflare-docs
```

Try:

```text
Use Cloudflare Docs to explain the current difference between Workers bindings and environment variables.
```

Expected: the public Streamable HTTP MCP server answers without an account.

## 3. Docker Hub

Install:

```bash
npx universal-agent-plugins add docker-hub
```

Try:

```text
Use Docker Hub to find the current official nginx image tags and summarize the available variants.
```

Expected: the digest-pinned Docker Hub MCP container reads public image data.
Docker must already be installed and running.

## OAuth follow-up

After a no-auth plugin works, test Cloudflare Radar, Figma, Linear, or Notion in
a dedicated test workspace. A one-off personal-workspace check is allowed only
with explicit owner approval, a synthetic read-only probe, no private content in
the result, immediate client cleanup, and provider-grant revocation. If a safe
granular provider revoke is unavailable, record cleanup as partial instead of
using a broader destructive action or claiming completion. Automated or
repeatable OAuth tests always require a dedicated test account or workspace.
Confirm the requested scopes before approval and begin with a read-only query.
OAuth success is client-specific and is tracked in the [test matrix](TEST_MATRIX.md),
not inferred from schema validation.
