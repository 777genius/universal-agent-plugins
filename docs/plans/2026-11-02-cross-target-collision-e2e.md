# C7 cross-target native collision: truthful failure and retry

**2026-11-02** — bounded follow-up to [the 12-gate plan](./2026-11-01-competitor-adversarial-reliability-gates.md) and its completed C1/C2/C4/C5 pilot. This is an executable-regression task, not evidence of a new production defect or a claim about current Microsoft APM.

## Supported route and independent oracle

- Use the existing **portable HTTPS archive** fixture, real Cobra CLI, resolver, lock, installer, file/receipt state and native adapters, with **Gemini and Codex** targets selected in one saved manifest. Native agents and security assessments are fixture-driven; do not claim live model execution.
- Prepare a disposable foreign `gemini/skills/a/SKILL.md` *before* installation. Save its bytes, file metadata, unrelated Gemini settings and user-data sentinel. Keep Codex free of the conflicting name.
- Generate a real immutable lock, snapshot manifest+lock, then invoke `install --locked` once. An exit-zero/full-success result is prohibited. The exact foreign Skill must survive. If Codex was installed before Gemini refused, the durable binding and `deps status` must report the partial outcome; a pre-write refusal is also allowed. Do not demand universal multi-target rollback.
- Remove **only** the deliberately created foreign fixture and replay the same locked intent. Both targets must reach verified installation with correct source bytes and intact unrelated settings; no untracked owner adoption.
- Inspect real returned errors, NativeObject ownership/receipts, read-only status and actual materialized/native files. A passing test cannot be inferred from only a CLI exit code or fabricated status.
- Register one additional `adversarial` case in the existing `package_manager_e2e_cases.json` (no new runner). Qualify with exact-head four-OS workflow and required checks; inspect raw `run`/`pass` events.

## Review and limits

**Security critic:** reject success without source-bound native ownership; challenge a false partial result, failed retry, foreign file mutation, no-receipt adoption, Windows path semantics.

**Architecture critic:** reuse existing fixtures; no new resolver, daemon, state, flags, VS Code changes or ownership policy. Existing `component-name-collision` checks two *sources on one Gemini target*; this checks one source on *two targets* where only one is obstructed.

**Evidence limits:** does not certify Claude, Copilot, all provider orderings, cross-process power loss or a running model. Do not merge until the final SHA passes the applicable E2E, security checks and review.
