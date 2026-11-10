# BUG-065 — git init can write through a pre-existing .git symlink

## Reproduction (actual local Git, disposable test paths)
The live project Skill fixture invokes `git -c core.hooksPath=/dev/null init --quiet` in a case-scoped project. Its previous check validated `project` and only checked the resulting `.git` marker **after** invoking Git. Independently reproduced under `tempfile.TemporaryDirectory`: create `project/.git` as a directory symlink to an unrelated temporary `outside` folder; `git init` returns 0 and writes `HEAD`, `config`, hooks, `objects/`, and `refs/` into the outside destination. The post-execution symlink refusal comes too late. Earlier hostile source cases could prepopulate a predictable future case directory within the same run root.

## Scoped correction
Refuse any pre-existing `project/.git` (directory, file, symlink, including dangling link) **before** invoking the pinned native Git executable. Project-root containment and post-run checks remain. Our test always uses a freshly created disposable project, so no legitimate existing worktree is rejected. Add one no-network regression with a preplanted symlink and a real pre-existing `.git` directory; assert the Git command is never invoked and the external target remains untouched.

Correctness review: pre-write refusal closes the demonstrated test-fixture vulnerability without altering source acquisition, product/adapter behavior or accepted native installation outcomes. Security review: a fail-closed check happens before external Git rather than after its side effects; existing sanitized `HOME/XDG/GIT_CONFIG`, hooks-disabled init, source SHA, LintAI, process containment, and foreign-profile guards stay unchanged. There remains an ordinary filesystem TOCTOU possibility against concurrent privileged writers; the audit's disposable case is not a security sandbox for hostile parallel OS processes. No broad abstraction or feature flag.

## Acceptance
Focused Python no-network test plus exact final-source GitHub Actions 30x30 source ZIP matrix; never count a genuine initializer refusal as PASS. Include this with BUG-063/064 detached candidate only after previous current-head native ZIPs finish.
