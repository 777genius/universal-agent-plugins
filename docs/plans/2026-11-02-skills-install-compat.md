# Skills install availability: production recovery (2026-11-02)

## Observed problem
The published Pages bundle contains PR #643, but some Skills still show no Install action. The UI reads an independently deployed `https://api.universal-agent-plugins.com/v1/skills` and requires `availability != unsupported-source` plus an installable source. A green Pages deployment does **not** establish that the API binary is deployed. The previous browser E2E route-mocked the Skills catalog; it did not prove the live API.

## Scope and guardrails
- Preserve the real CLI acquisition rules, canonical source identity, explicit source review, and refusal of invalid or missing publisher contents.
- Do not fabricate a Git path or exact Skill name, pretend an unverified source is verified, or turn unsafe catalog records into executable commands.
- Recover safe canonical GitHub repository / HTTPS publisher identities from historical provider listings **only when the listing ID, provider URL and source metadata agree**; keep unsupported malformed/unrecoverable entries visibly unavailable with a useful explanation.
- Add browser and TypeScript regressions for legacy website listings, conventional GitHub repositories, invalid sources, nonconforming Skill names and malicious mismatches. Preserve source-selector/reload behavior.
- Add a real HTTP acceptance probe for the public API and deployed site with bounded timeouts and explicit evidence. The probe must fail on invalid live contracts instead of using mock responses or reporting an untested deployment as success.
- Separate Pages deployment proof from independently operated systemd/Caddy Skills API deployment. Do not modify production host, DNS or credentials without verified trusted inventory.
- Commit to this draft PR immediately, keep PR updates minimal, review and repair failures on exact SHA, then merge only after required CI and live acceptance are confirmed.

## Verification matrix
1. Unit: repository source, publisher origin, old API shape, ambiguous/mismatched ID, injection, unsupported invalid origins.
2. Playwright: exact Install button and command on supported reconstructed records; truthful disabled state for genuinely unknown sources; locale/direct-link path.
3. Real public API: HTTPS health, browse/search DTOs, CORS, source classification and representative command eligibility; no intercepted requests.
4. Post-merge: Pages build/deploy SHA and real public browser smoke, plus separate API release verification.

All preceding deployment/status claims are provisional until GitHub and public HTTP evidence support them.
