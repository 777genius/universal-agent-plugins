# BUG-002: Live E2E inherited client profile overrides

**Source:** independent [Strix finding on PR #698](https://github.com/777genius/universal-agent-plugins/pull/698) (CWE-668, low severity as reported; potentially serious for the test runner's local profiles).

## Reproduction and risk

`scripts/live-install-audit.py` created a disposable `HOME`, `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, etc. but copied the host's environment first via `dict(os.environ)`. The actual UAP client detectors and native clients honor explicit overrides such as `CODEX_HOME`, `CLAUDE_CONFIG_DIR`, `GEMINI_CLI_HOME`, `QODER_CONFIG_DIR`, `PI_CODING_AGENT_DIR`, `KIMI_CODE_HOME` / `KIMI_CODE_CACHE_DIR`, `GROK_HOME`, `CLINE_DATA_DIR` and `CLINE_MCP_SETTINGS_PATH`. If any inherited override pointed outside the disposable case, a user-scope `add`/`remove` could affect an unrelated profile. The previous sentinels/snapshots watched only the sandbox and could falsely pass.

This is a **security problem in the new test harness**, not evidence of a defect in the production installer. No observed external-profile mutation has been established.

## Minimal fix plan

1. Build each subprocess environment from a single helper. Do not inherit known client-profile overrides or `GIT_CONFIG_*` injection settings from the invoking shell. Explicitly bind every supported client override to a directory inside that case's own temporary `home`; preserve only ordinary process variables required to run real vendors/CLI/network.
2. Use the existing normalized parent path for `GEMINI_CLI_HOME` (Gemini appends `.gemini`); do not point it at the final config root. Bind `CODEX_HOME` to `home/.codex`, `CLAUDE_CONFIG_DIR` to `home/.claude`, `PI_CODING_AGENT_DIR` to `home/.pi/agent`, etc. Retain the existing `AGENTPLUGINS_HOME` temporary `data` and isolated Git config.
3. Add a focused regression that injects hostile absolute override values into the parent's environment and requires all client profile roots in the child to reside inside that disposable home. Do not invoke real vendor clients or network for this guard test.
4. Run the guard test in the existing live CI pilot, then re-run real cross-agent source installation at the final commit. Keep the manual/vendor-auth/unsupported states truthful.

## Critical plan review

- **Strix security critic:** override inheritance made the “disposable only” claim false; fix at the test process composition boundary, not in each native adapter.
- **Data integrity:** assert the child environment, not just a sentinel that could miss an external path; ensure `GEMINI_CLI_HOME` means parent/home.
- **Architecture:** one helper, no daemon, feature flag, new public API, installer code, vendor profile mutation or scan-policy exception.
- **Scope:** this PR's E2E harness and one focused regression only. A green test must not imply a user profile was touched, nor that activation happened.

**Status:** plan recorded before fix; rerun and review required.