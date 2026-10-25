# Qoder capture correction: recovery checkpoint

The workspace runtime reset removed all unpublished worktrees, tooling and
local Git objects. This checkpoint reconstructs the retained decisions and
observations from the completed prior conversation. It does not claim the
lost implementation or its earlier test executions still exist or have been
verified in the recovered workspace.

Published recovery base: `1dfe89a5eb79386333d50b82e1705c26659e79fc` on
`feat/project-qoder-e2e`. The lost final local checkpoint was
`919d54b9800436e4326aa693a4faa71cf24ea041`; its final source was
`24ace90d572ebb582eeb3a5de85b7c19c99a0234`.

## Genuine phase-one failure retained

Run40011845496/job120613942005 at the published recovery base failed overall.
Artifact12532010944, qoder-1.1.78-precode-research-linux-amd64, had actual ZIP
SHA256 `918333b4690c1076d1714a19e036c7e66d62625d58881ae216449590c980ac54`.
The prior author downloaded the ZIP and verified that hash against GitHub's
artifact metadata. Its thirteen original members must be downloaded and
verified again before being restored as local raw evidence.

All six native commands exited zero. Version stdout was1.1.78, help exposed
project/user/local scopes, local source, JSON and scoped uninstall keep-data;
empty-list stdout was exactly `[]` plus newline. All six stderr streams also
contained the same83-byte warning:

`[qodercli] Warning: failed to extract native modules: EFBIG: file too large, write`

Stderr SHA256 was
`eeeb556610ef3a555aa2f6566f36fa8654d8ea6ef8221a60b9d5a40d7c227e78`.
The strict empty-list validator rejected it and success remained false. This
failed run must never be relabeled as a successful native collection.

## Prior reviewed correction and current requirements

The inherited child-wide RLIMIT_FSIZE1MiB limits unrelated cache files as well
as output logs. The approved correction replaces it with parent-owned live
nonblocking separate pipe capture, at most1MiB per stream. Retain core-dump
refusal, DEVNULL, fixed argv/environment/cwd, monotonic child deadline including
inherited-pipe draining, original-group cleanup, bounded reaping and descriptor
closure on all paths including enclosing interruption. Exact-cap EOF succeeds;
one excess byte fails and never expands the stored stream beyond its cap.

The prior final plan at d53a9de8 received independent R1 GO from
finish_sources_vscode_cline and R2 GO from finish_grok_integration, committed
at bbf178ef before source. The final clarification rejects every nonempty
stderr stream on all six probes, including whitespace, before unchanged
version/help/JSON shape checks. No warning whitelist is admitted.

Both reviewers also approved the lost final implementation24ace90d after
meaningful inert capture and preserved Qoder tests. Those historical results
are not current proof. Reconstructed code requires two fresh independent code
reviews and new meaningful tests before publication/replay. Any material new
implementation decision requires two fresh independent plan reviews first.

Do not change the actual hosted guard, pins, acquisition, native environment,
six read-only argv, native qualification rules or ownership behavior. No
vendor/native client executes on this task host. Preserve the exact official
archive SHA2565e3213d2d4256c86c5ad2c8bf73fd392f92ba80d0d38f822806542805a48a8ea
and binary SHA2560da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953.

After fresh reviews/tests, publish the recovered correction immediately to its
own feature branch with an exact expected-head lease and no force. A genuine
warning-free phase-one replay must then be downloaded, verified and interpreted
before any populated phase-two runner or production Qoder adapter code.
