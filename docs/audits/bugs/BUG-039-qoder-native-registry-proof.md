# BUG-039 — Qoder native registry proof for installed packages

## Source-backed reproduction

[Real Qoder 30-plugin CI](https://github.com/777genius/universal-agent-plugins/actions/runs/41830588372), artifact 13327709513 on historical source HEAD `bb574a1eb04bf58feaa54eaeff51d078fc02c967`, has **23 package-installed-native-unqualified** cases. Those had UAP add/list/repeat/remove exit zero, exact immutable receipts, retained neighbors and no live native leftovers. The generic filesystem oracle did not recognize Qoder's actual `name@local` native registry identity. No historical result is retrospectively promoted.

The existing production Qoder adapter already executes real `<pinned qodercli> --config-dir <selected isolated .qoder> plugins list --json` and accepts only the unambiguous versioned `name@local` ID. A downloaded cache/file path alone cannot prove registration.

## Scope-limited plan

1. In the live E2E harness only, use the **prebound absolute Qoder native executable**, with both isolated profile selectors bound to the case. Query after verified UAP repeated add and after successful UAP remove. Do not run models, modify vendor settings or use paid credentials.
2. Before remove, require one exact `<declared name>@local` and exact version from the verified UAP pinned binding. After remove, require **no same-name Qoder registry ID**. Unknown/malformed/duplicate JSON, foreign IDs, missing version, wrong version, nonzero native CLI exit, symlinked/outside profile or missing executable remain non-PASS. All other UAP source/lock, idempotence, sentinel and native residual oracles remain strict.
3. Retain **fixed status-only** native evidence. Do not serialize native listing, user settings, token values or CLI stdout. Add no-network counterexample tests and rerun final-head real 30-source Qoder lane. Do not call PASS from earlier historical logs.

## Two critical review rounds (self-review, not independent approval)

- **Ownership critic:** native list alone lacks immutable source SHA and can contain a foreign marketplace plugin. Require the *existing independently checked UAP installed pinned receipt*, exact local ID, exact version, one target, stable repeated-add ID, and subsequent absence; never a mere substring.
- **Security/cleanup critic:** pin the native executable before any plugin acquisition, refuse symlink/escaping profiles and forward only the isolated environment. Require Qoder and UAP native state both absent after remove while retaining foreign sentinels. A failed or unparseable native query cannot be converted to success, and cache/log exceptions must never suppress live registry entries.

## Status

Harness and no-network regression committed in PR #698; independent final-head Qoder 30-source ZIP, CI, security review and merge qualification remain pending. No product adapter, trust policy or feature flag change.
