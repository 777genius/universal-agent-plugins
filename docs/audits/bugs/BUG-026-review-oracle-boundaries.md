# BUG-026 — foreign sentinel decode and forensic depth coverage

## Reproduction / evidence

Independent [CodeRabbit review on PR #698](https://github.com/777genius/universal-agent-plugins/pull/698) reports two narrow audit-evidence defects on HEAD `8c6696a5e901ff0ecaf95d00abf66a402cd1f804` (2026-11-04).

1. `preserved_regular_text` calls `Path.read_text()` and catches only `OSError`. A foreign sentinel replaced with invalid UTF-8 raises `UnicodeDecodeError`, aborts the case, and loses the lifecycle's distinct install/list/remove evidence instead of reporting a tampered sentinel.
2. `copilot_settings_sanitized_shape` skips nodes at depth 32 but reports `parsed` whenever its remaining work stack is empty. A single deep terminal node is therefore silently omitted from BUG-019 forensic coverage.

These are *harness evidence defects*, not evidence of a UAP installation product failure.

## Narrow plan

- Read foreign sentinel with explicit UTF-8 and catch `UnicodeError` with existing `OSError`; tampered bytes must return `False`.
- Track any depth-limit skip and mark `truncated` even when the work stack empties.
- Add focused regressions for invalid-byte sentinel and a deep-only Copilot settings tree, plus shallow parsed control. Never emit settings values, never read another user profile, and preserve the 10,000-node limit.
- Run Python unit checks and CI on exact updated GitHub HEAD; inspect fresh Actions results and review again. **Do not merge while broader E2E/security gates remain non-PASS.**

## Critic round 1 — failure modes

A decode failure must be observable as lost preservation, *not* a success or a fatal harness-error; explicitly catching Unicode failures is narrower than swallowing all exceptions. The Copilot forensic probe must report incomplete coverage if any branch is truncated, even when its sibling branches are scanned.

## Critic round 2 — safety and evidence integrity

The change must not relax `path_inside_case`, change native deletion classification, export sensitive JSON, or conflate `truncated` with absent matches. New regression assertions should fail on the original implementation, then pass after the minimal patch. Independent final-head review and actual 30+30×15 artifacts remain separate release gates.

## Status

Implementation and two focused regressions pushed (commits `113ca2d`, `78266b1`, `23cbe88`). Critic rounds above assess scope and failure modes but are **not** independent approval. Original invalid UTF-8 canary uses actual `b"\\xff\\xfe"` bytes. Required CI, current-head live ZIP evidence and independent CodeRabbit/Strix clearance are still pending; no merge authorization.
