# BUG-049 — Cursor project-Skill native audit lacked a verified Agent binary

## Observed

The original source-HEAD `ff760b32880f255a520cbe21d7df1927296375b6` [Cursor Skill job #126949099983](https://github.com/777genius/universal-agent-plugins/actions/runs/41937361674/job/126949099983) failed before executing any source. Its [raw ZIP #13375466154](https://github.com/777genius/universal-agent-plugins/actions/runs/41937361674/artifacts/13375466154) records `cases_attempted=0`, 30 explicit `client-unavailable` slots, and a vendor bootstrap refusal: the downloaded Cursor Editor `.deb` has no independently reviewed digest. This is a legitimate fail-closed result, not 30 Skills PASS.

The [immutable Homebrew Cursor Agent Cask at 5815fd3716b83066412887902c6dd4bf92e4d7ad](https://github.com/Homebrew/homebrew-cask/blob/5815fd3716b83066412887902c6dd4bf92e4d7ad/Casks/c/cursor-cli.rb) independently records Linux x64 Cursor Agent `2026.10.22-27dcf5f`, source `https://downloads.cursor.com/lab/2026.10.22-27dcf5f/linux/x64/agent-cli-package.tar.gz`, SHA-256 `85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294`, and native executable `dist-package/cursor-agent`.

The production Cursor adapter deliberately differentiates `cursor_agent` (eligible for *project* components) from `cursor_editor` (required for *user* plugin-package preparation). Replacing the Editor with Agent in the Plugin lane would be an invalid native-qualification shortcut.

## Bounded correction

1. Select the independently pinned official `cursor-agent` archive **only** in the Cursor **Skill/project** matrix lane. Retain the Editor `.deb` and missing-digest refusal for Cursor **Plugins/user** until that exact artifact has independently reviewed provenance.
2. Verify the official archive SHA-256 **before** extraction. Use Python 3.12 `tarfile`'s `filter="data"` to reject traversal and escaping archive links, then require the exact regular executable at the Homebrew-declared path. No `curl | sh`, `sudo`, upstream installer execution, or substitution of one artifact's SHA for another.
3. Run vendor `--version` with an isolated disposable `HOME`, `USERPROFILE` and `XDG_CONFIG_HOME`, rather than risk an implicit write into the runner's ambient profile.
4. Retain the harness's exact source, lock, repeated install, native-file attribution, removal, and foreign-sentinel oracles and nonzero exit for any unqualified cases. Complete the **new-head** original ZIP review before counting any source as PASS.

## Two critical checks

**Supply chain/safety critic:** The immutable independent Homebrew Cask binds both source URL and digest; a mismatched hash, unsafe archive member, missing executable or failed version probe aborts native qualification. The Editor `.deb` remains blocked, and no third-party command is run as a shell installer.

**Correctness/contract critic:** Agent-only presence qualifies only Cursor project Skills, not editor package discovery, manual plugin activation, model execution, or authenticated user sessions. A successful `--version` alone does not count any of the 30 source cases as PASS.

## Verification gate

CI must execute the real 30-Skill source suite for Cursor on the final PR commit, retain original JSON and vendor-proof ZIP, and independently verify its actual per-source results. Until then, no PASS or merge is claimed. External security review and other unresolved vendor-native lanes remain separate gates.
