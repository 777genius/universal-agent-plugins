# BUG-060 — Grok E2E foreign Skill uses invalid plugin directory

## Reproduction and root cause
On real pinned Grok Build 1.0.70, the final-head 30-Skill E2E produced 30 nonzero `PLAN_REJECTED` results. The shared audit harness precreates `project/.grok/plugins/foreign-neighbor/SKILL.md` without `.grok-plugin/plugin.json`. The production Grok project collision scanner enumerates every plugin child and deliberately refuses a child with an unknown manifest layout before it can qualify a selected source. This is a **test fixture**, not a reason to loosen production collision checks. See PR #698 discussion #6660586556.

## Minimal correction and two critical passes
- **Correctness:** retain the managed Grok `.grok/plugins` target and all 30 immutable sources; place only the unrelated standalone foreign Skill in `.grok/skills/foreign-neighbor/SKILL.md`. Preserve the byte-exact foreign-sentinel oracle. Add a no-network regression proving no invalid foreign plugin directory remains.
- **Security:** keep fail-closed native manifest parsing, user-profile isolation, symlink refusal, vendor SHA-256, LintAI and process containment unchanged. Do not promote a prepared client binding to native activation.

## Acceptance
Run focused Python harness regression and current-head Required/Core; obtain original 30-case Grok Skills ZIP on the **new commit**. Verify exact source, version, lock, repeat, native identity, removal, and foreign files; any remaining refusals stay non-PASS until independently explained.
