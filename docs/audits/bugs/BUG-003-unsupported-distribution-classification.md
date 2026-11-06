# BUG-003 — Unsupported signed distribution mislabeled as installer failure

## Reproduction (real artifact; historical head, not final-head qualification)

The retained `remaining-client-grok-plugin` artifact from [Actions run 41727674724](https://github.com/777genius/universal-agent-plugins/actions/runs/41727674724), source index 13 (`marketing-kit`), recorded:

```
agentplugins: no eligible directory release for "marketing-kit":
777genius/marketing-kit: release does not support complete target set; missing grok
```

Actual UAP install exit was 1, no native files were written, and the harness labeled the result `install-failed`. The refused source has no eligible Grok distribution; this is an expected source/target incompatibility, **not** proof that the installer malfunctioned.

## Minimal plan and boundaries

1. Add one precise diagnostic matcher for `release does not support complete target set` to the existing `documented-or-source-unsupported` outcome. Preserve exit code, complete diagnostic and failure semantics; never call the case `passed`.
2. Add a subprocess-free regression that feeds the exact observed diagnostic and ensures a different unexpected installer failure is *not* classified unsupported.
3. Run focused unittest and real CI on the new HEAD; inspect artifacts. Do not alter fixtures, Directory policies, LintAI, cgroup containment, client profiles or source acquisition.

## Review, before implementation

- **Critic 1 — correctness:** A broad match on `not supported` is insufficient: the actual error says `does not support complete target set`. Match the stable, exact emitted phrase instead of reinterpreting arbitrary failures.
- **Critic 2 — security/evidence:** A classification change must never convert nonzero installs to success or mask any unexpected failures. Preserve the original stderr, nonzero CI exit and a separate `documented-or-source-unsupported` status.

**Status:** narrow source-target classifier fix and focused regression pushed in PR #698; current-head live artifacts and independent review still required. This report does not qualify a merge.
