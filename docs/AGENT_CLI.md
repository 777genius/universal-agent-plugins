# AI-agent CLI workflow: discover, preview, apply, verify

**Release baseline:** This guide is verified against [CLI 0.1.110](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.110)
(source `1b7d2db803dd5eea2d632d8f4e149b4d7bb4a48e`). The agent-first commands
and machine metadata described below are included in that release. Check
`agentplugins version` before relying on them; package-manager channels may lag.
Installation evidence does not establish native runtime activation or authentication.

UAP has one installer engine for interactive people and automated agents.
The browser/TUI is optional; automated clients should use an explicit package,
target and scope, capture JSON, inspect the result, and respect native trust.

## Complete package: explicit User installation

```sh
agentplugins version --format json
agentplugins add context7 --target codex --scope user --dry-run --format json
agentplugins add context7 --target codex --scope user --format json
agentplugins info context7 --scope user --format json
agentplugins doctor --scope user --format json
```

CLI 0.1.110 accepts `--yes` on `add` and `install <source>`:

```sh
agentplugins add context7 --target codex --scope user --yes --format json
```

The flag accepts the preflighted install plan without invoking post-install
interactive lifecycle prompts. Pending activation/authentication remains
visible as a separate follow-up. The same explicit `--yes` is
accepted by `update`, `repair` and `remove`; removal requires an explicit
`--target` in this headless route and retains plugin data unless `--purge-data`
is separately specified. Legacy `add --target` and JSON calls remain accepted
for compatibility. None of these confirmations override a security finding,
foreign ownership, a client's approval UI or authentication.
`--dry-run` never applies the installation, even together with `--yes`.

## Raw Skills: discover then install into one project

`skills list` uses the existing bounded source scanner, not
a browser, code execution, external service or new installer. It reports all
discovered candidates (available, invalid or unavailable), exact source paths
and whether scanning was complete. Multiple Skills may have the same name;
select an exact `--path` when ambiguous.

```sh
agentplugins skills list owner/repository --format json
agentplugins skills list owner/repository --path skills/review --format json

agentplugins skills add owner/repository --skill review \
  --target codex --scope project --project-root "$PWD" \
  --no-ui --dry-run --format json

agentplugins skills add owner/repository --skill review \
  --target codex --scope project --project-root "$PWD" \
  --no-ui --yes --format json

agentplugins deps status --scope project --project-root "$PWD" --format json
agentplugins deps check --scope project --project-root "$PWD" --format json
```

The `skills list` source may be a GitHub repository/folder URL or a supported
HTTPS Skills-index publisher. `--path` and `--ref` are Git-only. The listing
does not invoke client-specific executable commands or mutate managed files.
For a large/incomplete catalog, narrow `--path` rather than assuming an
unseen Skill does not exist.

For exact reproducibility, commit `uap.toml` and `uap.lock`, then run:

```sh
agentplugins install --locked --scope project --project-root "$PWD" --yes --format json
```

CLI 0.1.110 accepts signed Directory short names in project-scoped `add`
when the package and its mandatory dependencies are eligible for the selected
targets. A reviewed exact commit-pinned Git source is another supported option.
Review [Project installation](PROJECT_INSTALLATION.md) for source eligibility
and client activation limits. The standalone Skills Project flow has been
available since CLI 0.1.93.

## Source-only addition: local collections and complete selection

The following commands require a build containing the local collection changes;
they are not part of the CLI 0.1.110 release baseline above.

```sh
agentplugins skills list ./skills --format json
agentplugins skills add ./skills --skill review,testing \
  --target codex,claude --scope user --yes --format json
agentplugins skills add ./skills --all \
  --target codex,claude --scope project --project-root "$PWD" --yes --format json
agentplugins skills add owner/repository --all --target codex,claude --yes
agentplugins skills add https://publisher.example --all --target codex,claude --yes
```

Local `--path` selects one exact child Skill directory. A directory with its own
`SKILL.md` retains single-Skill behavior; an invalid root document never causes
descendants to be installed instead. Without explicit names, a local collection
uses the terminal picker. Local listings have an empty JSON `revision` because
the source is mutable; installation still creates verified immutable snapshots.

`--all` selects the complete valid catalog observed for that invocation. Empty,
incomplete, invalid, unavailable or ambiguous catalogs are refused before install.
It cannot combine with `--skill`, `--path` or `--ui`; use an exact source folder
to narrow a collection. `--yes` remains confirmation, not automatic selection.
Each chosen Skill is saved independently. `update --all`, repair and locked
restore keep those choices; they do not enroll new siblings from the source.

## Machine output: interpret evidence, not just the exit status

- `--format json` suppresses human interaction and keeps command results on
  stdout, with progress/errors on stderr. An actual failure exits nonzero.
- Existing JSON shapes are **preserved**. `add`, `info`, `doctor` and
  `skills list` use `{schema_version, command, result, data}`. The existing
  `deps` and `skills add` top-level fields remain available. New machine
  metadata is additive. Do not assume every subcommand has identical legacy
  shape.
- CLI 0.1.110 includes a normalized `readiness` projection where supported:
  `ready` = the recorded installer lifecycle is verified, **not** a real tool
  invocation; `needs_action` = activation/auth/native check remains;
  `failed` = recorded failed state; `unknown` = insufficient facts/preview.
- Use machine `next_actions` when available. Each action has a stable `code`,
  an `actor` (`agent`, `user` or `native_client`) and `message`. Do not
  treat a suggested command as authority to bypass policy.
- `deps status` continues to report exact manifest/lock paths, ownership,
  receipts, `facts` and excluded clients. A stored receipt is not proof that
  a native model can use a freshly started MCP tool.
- New typed CLI usage errors originating before a command has emitted output
  use a sanitized failure envelope. Existing Skills selection errors keep
  their own one-document JSON. **Legacy native/runtime refusal paths** (including repeated removal of an absent installation)
  deliberately retain their existing nonzero exit code, empty stdout and exact
  stderr for backwards compatibility. Existing command-specific structured
  failures keep their original fields. Raw provider output is never machine
  authorization.

## Scope is a safety input

For automated work, **always set `--scope`**. For project installation, also
set `--project-root` to the exact intended directory. Omitting scope on
`add` or `skills add` still selects `user`. Only **argumentless**
`agentplugins install` may automatically select Project from a regular
`uap.toml` in its *exact current working directory*. It never searches
parent directories. Machine output explains the effective selection as
`explicit`, `default_user` or `argumentless_project_manifest`.

## Never make these assumptions

- `--yes` is **not** authorization to add `--accept-security-risk` or
  `--purge-data`. These have distinct risk and ownership boundaries.
- Do not invoke `--activation-complete` or `--auth-complete` to make a
  status green unless the corresponding **external** phase has genuinely
  occurred and the specific adapter supports attestation.
- `activation_prepared` and `auth_unverified` are not failures that justify
  repeated reinstall. Check the native client's next action.
- A partly successful multi-client operation is not atomic across every
  external system. Inspect each target, repair only owned projections and
  preserve unrelated files.
- Source-only changes cannot be assumed to exist in the last published npm,
  Homebrew or native binary. Verify the actual version and release notes.

## Useful commands

```sh
agentplugins list --scope user --format json
agentplugins info context7 --scope user --format json
agentplugins deps tree --scope project --project-root "$PWD" --format json
agentplugins deps recover --scope project --project-root "$PWD" --dry-run
agentplugins doctor --scope project --project-root "$PWD" --format json
```

Use the [user/project guide](PROJECT_INSTALLATION.md), [dependency contract](UAP_DEPENDENCIES.md),
[security and consent rules](INSTALLER_AUTOMATION_PRINCIPLE.md) and
[client evidence boundaries](PROJECT_READINESS_DOCTOR.md) for specifics.
