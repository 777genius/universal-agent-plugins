# BUG-042 — native residual scan mistakes URL/hostname substrings for installed plugins

## Reproduction

Independent [CodeRabbit inline review](https://github.com/777genius/universal-agent-plugins/pull/698#discussion_r4470160661) (2026-11-04) identified `scripts/live-install-audit.py:native_identity_residuals`: `identity.casefold() in relative.casefold()` and substring search over every native file mark `https://github.com/other/repo` as a retained `github` plugin. The same affects short aliases such as `neon`, `linear`, `notion`, `vercel`. This is a **false-negative audit conclusion** (`cleanup-failed`) rather than proof of a broken production uninstall.

## Minimal plan

1. Use existing `exact_native_identity` token/key parser for small native file bodies. Detect managed `name@agentplugins-<12 lower hex>` identities with a boundary-aware, anchored regex; a URL hostname or arbitrary substring does not qualify.
2. For paths, match only an exact filename/directory name, its exact stem, or a fully qualified managed marketplace component; never arbitrary substrings. Keep symlink, unreadable, non-regular and walk-error paths fail-closed.
3. A **nonpassive** native file larger than the existing 1 MiB scan cap cannot prove absence without reading; record a bounded `unscanned-too-large` negative result instead of treating it as cleaned. Keep narrowly established Codex/Qoder passive cache/log exceptions.
4. Regress with `https://github.com/other/repo` as the only text (must be ignored), an exact `github@agentplugins-aaaaaaaaaaaa` entry (must remain residue), an exact `github` key, foreign-sentinel content and symlink/unreadable objects. Re-run full final-head E2E and inspect native evidence.

## Critical review round 1 — false positives

A native file containing `https://github.com` or a similarly named foreign package is not evidence that a managed plugin remains. Do not take CodeRabbit's unqualified `name@` prefix suggestion literally: accept only UAP's fixed `@agentplugins-<12 hex>` namespace (or an independently attributable exact name), not any foreign `@marketplace` string.

## Critical review round 2 — false PASS / complexity

Do not fix a false-negative by ignoring all native settings. Retain exact managed keys, name fields, unsafe links, parse failures and files that cannot be read under the bounded proof budget. Reuse the existing parser rather than creating another client registry or general-purpose document scanner. No product adapter, auth, LintAI, cgroup, symlink policy, or source immutability change.

## Status

Finding independently reported and verified in current source; narrowly scoped regression and implementation pending. No final-head acceptance or merge approval.
