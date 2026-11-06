# BUG-001: LintAI scanner failures hide process exit reason

**Observed**: [Remaining-client run 41724433025](https://github.com/777genius/universal-agent-plugins/actions/runs/41724433025), Cursor plugin case `rayfin` at immutable source `github:microsoft/rayfin@0979439a7a0806abd0ee48c2619b37f297ae1324`, returned:

```text
agentplugins: security assessment failed before installation: lintai security scan failed:
```

**Classification**: reproducible **error-reporting defect**. The security scan correctly blocked installation, but the message is missing the underlying exit status. The precise LintAI internal cause remains unknown until captured; do not mistake this for a successful security assessment.

## Root cause

`install/integrationctl/agentplugins/adapters/securityscan/scanner.go` checks failed `exec.CommandContext`. For any nonzero exit other than 1, the error is formatted using **only trimmed stderr**. A process that returns status 2 with empty stderr therefore produces an opaque message and discards `*exec.ExitError` from the error chain.

## Minimal fix plan (strict scope)

1. Preserve existing deadline, stdout/stderr size limits, scanner environment, and exit-code-1 contract (findings flow).
2. When exit status is not 1, wrap and report the original command error (so `exit status 2` survives); include trimmed scanner stderr when nonempty. Do not expose additional command-line secrets, output bodies or install source contents.
3. Add **one** focused regression test using a short disposable executable that exits nonzero with no stderr, asserting the returned error includes the actual exit status. Windows can skip the POSIX-script fixture; production code remains OS-neutral.
4. Re-run package securityscan tests and a genuine (not mocked) `rayfin` source acquisition/scan from the live installer matrix. If the same LintAI failure remains, mark the installation as **blocked** with new diagnostic evidence and triage the underlying scanner separately.

**No scope creep**: do not change LintAI policy, validation thresholds, security signatures, risk overrides, scanner version, source restrictions, native agent adapters, or Registry contents. Never turn a scanner failure into an installation pass.

## Critic review before change

- **Security**: retaining a short OS exit status and pre-existing stderr is safe; do not echo stdout/credentials or bypass enforcement.
- **Behavior**: exit status 1 remains treated as report/finding flow; timeout and output-limit branches remain first and unchanged.
- **Testing**: test the exact observed missing-reason branch, not the entire scanner implementation; use the existing package layout.
- **Release**: only merge if focused tests and final live source run remain green; no need for an independent release.

**Review state:** self-critique complete, independent CodeRabbit re-review requested earlier but rate-limited; do not claim that critical external review passed.
