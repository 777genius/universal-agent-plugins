# Compatibility and authentication

All packages target Agent Plugins 1.0.0. Schema validation proves package
conformance; it does not prove that a vendor endpoint is available, that OAuth
will succeed in every client, or that every client exposes the same tools.

The standard-compatible launch clients and supported transports are maintained
at [agent-plugins.org](https://agent-plugins.org/compatible-clients).

Directory compatibility is release- and environment-specific. The website
shows one product card even when upstream and community distributions coexist,
labels the reviewed **Default source**, and links alternatives and immutable
provenance. Existing installs stay on their recorded distribution during
update; changing source is an explicit `switch`.

## Package matrix

| Plugin | Component | Authentication and scope |
| --- | --- | --- |
| `agent-code-navigator` | 4 skills | Local tools only; specialized tools are optional |
| `atlassian` | Streamable HTTP | Client-managed Atlassian OAuth |
| `chrome-devtools` | stdio | Local browser session; pinned npm package |
| `cloudflare` | Streamable HTTP | Client-managed Cloudflare authorization |
| `cloudflare-bindings` | Streamable HTTP | Client-managed Cloudflare authorization |
| `cloudflare-docs` | Streamable HTTP | Public documentation endpoint |
| `cloudflare-observability` | Streamable HTTP | Client-managed Cloudflare authorization |
| `cloudflare-radar` | Streamable HTTP | Public telemetry behind client-managed Cloudflare OAuth |
| `context7` | stdio | No credential stored; service limits may apply |
| `docker-hub` | stdio | Public data only in portable config; authenticated writes need client-specific secrets |
| `figma` | Streamable HTTP | Client-managed Figma OAuth |
| `firebase` | stdio | Uses the local Firebase CLI login and selected project |
| `github` | Streamable HTTP | Client-managed auth; no PAT stored |
| `gitlab` | Streamable HTTP | Client-managed GitLab OAuth |
| `greptile` | Streamable HTTP | Client-managed auth; no API key stored |
| `heroku` | Streamable HTTP | Client-managed Heroku OAuth |
| `hubspot-crm` | Streamable HTTP | Client-managed OAuth; upstream may remain beta/read-only |
| `hubspot-developer` | stdio | Uses the local HubSpot CLI login |
| `linear` | Streamable HTTP | Client-managed Linear OAuth |
| `neon` | Streamable HTTP | Client-managed Neon OAuth |
| `notion` | Streamable HTTP | Client-managed Notion OAuth |
| `sentry` | Streamable HTTP | Client-managed Sentry OAuth |
| `statsig` | Streamable HTTP | Client-managed Statsig authorization |
| `stripe` | Streamable HTTP | Client-managed Stripe OAuth; may expose write tools |
| `supabase` | Streamable HTTP | Client-managed auth and project scoping; development/test data only per upstream guidance |
| `vercel` | Streamable HTTP | Client-managed Vercel OAuth |

## OpenAI delivery boundary

| Package group | Count | Codex | ChatGPT |
| --- | ---: | --- | --- |
| stdio MCP | 5 | Generated `.mcp.json` package | Not supported; Codex-only |
| Streamable HTTP MCP | 20 | Generated `.mcp.json` package | Requires a registered `.app.json` binding |
| Skills-only | 1 | Generated skills package | Separate skills package; package UI E2E not claimed |

Cloudflare Docs is the only remote package with a registered ChatGPT
development binding. Its direct no-auth connection passed `list_resources` and
one read-only documentation search. Its registered personal app also passed
Plugins UI discovery, user-attested manual activation, and the same read-only
runtime. Separate evidence proves repository marketplace ingestion, official
manager installation, cache materialization, and exact `.app.json` linkage. It
does not yet prove ChatGPT Work UI activation or package-routed runtime. No other
remote package is claimed as ChatGPT-installable. The website keeps ChatGPT
visible but disabled for those releases and explains that a registered app
binding is required.

## Dependency pins

Verified against stable registry releases on 2026-08-07:

| Runtime dependency | Pin |
| --- | --- |
| `chrome-devtools-mcp` | `1.7.0` |
| `@upstash/context7-mcp` | `4.0.0` |
| `firebase-tools` | `15.26.0` |
| `@hubspot/cli` | `8.12.0` |
| `mcp/dockerhub` | OCI digest `sha256:76454af…d4248` |

The code-intelligence skills document current optional versions of Semble
`0.5.4`, CodeGraphContext `0.5.6`, and Serena `1.6.1`.

## Verification levels

- `Schema`: `plugin.json`, `mcp.json`, skills, path rules, and pins validate.
- `Reachability`: the endpoint or package exists without authenticating.
- `Authenticated`: a user completed the vendor flow in a specific client.
- `Behavior`: representative read and write operations were tested safely.

Runtime and authentication claims are tracked with a client, date, account
scope, and exact scenario in [the test matrix](TEST_MATRIX.md).
