# P1 — Fix real /landing glob and minimatch dependency blockers

## Evidence
[Issue #692](https://github.com/777genius/universal-agent-plugins/issues/692): Dependabot reported `glob` / `minimatch` `security_update_not_possible` because the lockfile resolves old majors. This may be **transitive** build tooling; advisory existence does not automatically establish runtime exploitability. The published website and CLI E2E gates do not certify this separately.

## Minimal plan
1. Inspect actual `landing/package.json`, `landing/pnpm-lock.yaml`, `pnpm why glob minimatch`, Node/pnpm engines, and parent package graph. Enumerate resolved paths / advisories / production vs build-only exposure; obtain current metadata instead of guessing.
2. Prefer a supported upstream update that resolves fixed compatible package versions. Do not force incompatible global overrides merely to quiet alerts. Keep unrelated dependency churn out.
3. Validate clean `pnpm install --frozen-lockfile`, dependency tree, `pnpm audit`/advisory status, lint, typecheck, site build, registry tests and real browser E2E at desktop/mobile, light/dark.
4. Check generated artifacts and GitHub Dependabot alerts on the exact PR SHA. No silent suppression or security-ignore. Preserve unrelated site PRs (#773 and SkillSpector).
5. Review, full applicable CI, safe merge, post-merge site smoke and confirmed resolution; if upstream cannot resolve safely, retain the issue with explicit mitigation, not invented PASS.

## Non-goals
No Nuxt major migration solely for a transitive advisory, no security alert disabling, no blind pinning, no unrelated app changes.
