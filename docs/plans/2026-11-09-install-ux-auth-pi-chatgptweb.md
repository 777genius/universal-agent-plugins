# Install UI: authentication, Pi, ChatGPT Web clarity

Date: 2026-11-09 · Scope: landing/UI copy and tests only.

## User-visible defects

1. **Authentication** in auto-detect mode pools *all eligible release/distribution policies*. A client may have different authentication across variants and appears twice as both required and not required. Group each client once, mark release-dependent policy as **varies**, and explain why; do not invent an authenticated session or change signed policy.
2. **Pi** is a supported client, but the plugin picker checks whether this *specific package's active signed release* has a Pi-compatible target. Preserve disabled state for incompatible packages; explain that the package (not Pi) lacks a Pi target. The Pi agent page and all nine locales still describe extensions only, although the current adapter and capabilities also implement Skills and stdio/Streamable HTTP MCP in user/project scopes. Synchronize explanatory copy with the existing capability table and code.
3. **ChatGPT Web** is not installed into a remote account by UAP. A signed app binding is distinct from the user's app connection, and the bounded Context7 personal-preparation flow is separately qualified. Replace the ambiguous “no verified connection” label with a package-specific binding explanation; keep Web disabled without signed authority and keep explicit consent/setup in ChatGPT.

## Non-goals / safety

No changes to the CLI, signed Directory, adapter, authentication policy values, account connection logic, installation eligibility or release. No inference that a supported agent can install every plugin, no synthesized Pi target, and no remote ChatGPT account mutation. Keep desktop-vs-web canonical identities and per-scope refusal intact.

## Acceptance

- Unit: auto policy with duplicate client + different releases maps to one **varies** group; mixed different clients stays differentiated; explicit resolved release uses its policy.
- Browser: Pi disabled on a plugin lacking signed Pi target, with accurate explanation; ChatGPT Web disabled without signed binding, with separate setup guidance; no unwanted command or target enablement.
- i18n: update text in all nine published locale dictionaries; verify client page/table/readme parity.
- Exact-head: TypeScript, locale tests, unit/lint, both Playwright shards, and both Pages paths, then independent review. Remain Draft until results pass.

Estimated change: 1–3/10 complexity for copy, 3/10 for grouping; ~130–250 production/test LOC plus translations. Maintain Clean Architecture, SOLID, DRY; no feature flags.
