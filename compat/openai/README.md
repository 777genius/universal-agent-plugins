# OpenAI compatibility layer

Agent Plugins 1.0 uses root `plugin.json` and `mcp.json`. The
[official OpenAI plugin packaging contract](https://developers.openai.com/plugins/build/plugins)
uses a host-specific `.codex-plugin/plugin.json`. A remote MCP registered in
ChatGPT Developer Mode is referenced by a plugin-root `.app.json`, and the
manifest's compatibility `apps` field must point to `./.app.json`.

This directory is generated from the portable packages under `plugins/`:

```bash
python3 scripts/build_openai_compat.py
python3 scripts/build_openai_compat.py --check
python3 scripts/validate_openai_compat.py
```

Do not edit generated package files here. Update the portable source package and
the generator instead. OpenAI-only auth metadata is maintained explicitly in
the generator. Registered ChatGPT development connections are separately
allowlisted in `app-bindings.json`; this host-only sidecar is not part of the
portable package and must match one exact Streamable HTTP endpoint. Each entry
also pins repository-relative direct and personal-app runtime records by exact
Git revision and SHA-256 digest. The
loader requires the exact public app ID, plugin, endpoint, UI observations, call
counts, and read-only runtime checks to agree before any package is generated.
The portable catalog mirrors only the public binding fields plus an evidence
path and immutable Git revision under `compatibility.chatgpt.app_binding`.
Clients must still match its server and URL to the portable `mcp.json` before
generating `.app.json`.

Structural validation cannot prove who controls a registered ChatGPT app ID.
That ownership is a human review boundary: `@777genius` must confirm the ID was
created in the intended ChatGPT workspace. `CODEOWNERS` requests that review;
whether GitHub enforces it depends on repository branch-protection settings.

Only `cloudflare-docs` has a registered no-auth development binding backed by
the cited historical observations. Current stable-launch runtime/OAuth evidence
is unavailable until the protected launch workflow succeeds for an exact signed
production publication and attested CLI release; these records do not satisfy
that prerequisite.
The generated package passes static validation, and the direct connection has a
sanitized read-only runtime record. The registered personal app also passed
Plugins UI discovery, user-attested manual activation, and read-only runtime.
This proves the exact `.app.json` ID linkage, not ingestion of the local
`.codex-plugin` package or an `agentplugins` lifecycle in ChatGPT.

The repo marketplace is at `.agents/plugins/marketplace.json`. These adapters
are intended for local compatibility testing. They are not public listings in
OpenAI's Plugins Directory and do not include the logos, legal pages, verified
publisher identity, domain verification, or review materials required for a
public submission.
