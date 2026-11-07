# Project install: partial components and single-source capability metadata

Scope: repair public site Project selection without replacing the Directory v1 or the installer. Preserve current published CLI 0.1.101 and all existing User paths.

## Root causes
- `landing/utils/projectCatalogSource.ts` hardcodes project component sets and rejects a full package when only one component is unsupported for the selected client.
- `clientCapabilities.generated.json` is generated from User capabilities only; reusing it for Project is unsafe. The CLI already uses `ScopedCapabilities(project)`, an explicit Project adapter registry, component verdicts, and receipt-based ownership.
- Project component readiness and per-client activation are distinct. CLI is authoritative for actual transport, source, mandatory dependency, trust, and write preflight.

## Small implementation
1. Extend existing Go generator and retained JSON with **Project** support, scope-qualified Skills/MCP and only interpretable extension kinds; build from `all.Project()` and `ScopedCapabilities(project)`. Keep User metadata untouched and the Go generated-file equality test.
2. Replace landing's duplicate Project component table with a typed reader of generated Project capabilities. Admit reviewed signed source only if every requested client has at least one potentially deliverable component; keep signed selection/closure and ChatGPT refusal unchanged. **Never imply all package components can execute.**
3. Reuse the existing `InstallOutcome` on both cards and detail pages to show component-level deliverable/unavailable status for each manually selected Project client. State that CLI verifies exact transports, required child packages, and activation. Keep targetless commands honest.
4. Fix any demonstrably incorrect Project-scoped capability declaration found in comparison with client project planner, without widening runtime permissions. Preserve no-implicit-User fallback.
5. Test mixed Skill+MCP, Skills-only, MCP-only, Extensions-only, multi-target refusal, stale/unsigned/revoked/ChatGPT, preserved User selection and generated capability drift; Playwright card/detail/scope/copy at desktop/mobile. Run established public Marketing Kit 0.1.101 Project A/B/cold restore/update/remove matrix plus existing CLI project/Global tests on the final HEAD.

## Reviews / acceptance
- Correctness: signed Directory v1 remains source authority; selected client and component support are separate.
- Security: no bypass of verified source, dependency lock, project root, foreign-file ownership, revocations, or native trust.
- UX: show partial support only as a *preview*, not installation success or native execution.
- Regression: generator Go test, landing unit/i18n/Playwright, required CI, published npm E2E and postmerge checks must be green before merge.
