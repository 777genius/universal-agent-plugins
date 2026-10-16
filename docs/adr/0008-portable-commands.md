# ADR 0008: Portable Commands

## Status

Accepted (2026-10-16)

## Context

Authors want one user-started prompt (a "command") that works in every client
the installer supports. Clients disagree on almost everything about commands:
Claude Code and Codex start skills by hand (`/plugin:skill`, `$skill`), Gemini
CLI and Cursor have their own command files, OpenCode turns skills into
commands, and only some clients document a way to keep a skill out of the
model's own choice. Placeholders, shell injection and root variables are
expanded differently or not at all. The Agent Plugins 1.0 specification has no
command component, and the package must stay a valid 1.0 package.

The installer already owns one directory in packages,
`io.github.777genius.agentplugins/` (authored `runtime/`, installer-written
`managed-stdio-v1/`), and it already delivers skills to all 13 clients.

## Decision

**Source.** A command is `io.github.777genius.agentplugins/commands/<name>.md`
with a closed frontmatter of exactly `description` and
`disable-model-invocation` (a required YAML boolean). The body may not use
plugin or client variables, argument placeholders or shell injection syntax.
Anything outside that format is a `command_*` diagnostic with
`boundary: command`, and the command is not delivered. A file the reader
cannot read gets its own code, `command_read_failed`, so an I/O failure is
not reported as an authoring mistake. The format is checked by
the pure `commandext` package, shared by the loader, the package view used by
authoring, the stager and the security scan; it imports only `domain`,
`conformance` and `gopkg.in/yaml.v3` (depguard `commandext-pure`, archtest).

**Delivery as skills.** The installer renders each deliverable command as the
skill `<slug>-<name>` (plugin name with dots as dashes), written into the
sanitized staging tree before the client projector runs, so projectors treat
it like any authored skill. The name is the same in every client. A planned
command skill is a `ComponentDecision` of kind `skill` with
`source: command` and `source_name`. The package never contains the rendered
bytes, and the state schema does not change.

**One rule for explicit-only commands.** The planner alone decides, from the
declared trait `ClientTraits.EnforcesExplicitSkillInvocation`. An ordinary
command reaches every client that takes skills and does not withhold commands
(see below). An explicit-only command
reaches a client only when that trait is true; otherwise it is withheld: no
component, a plan warning `explicit_invocation_not_enforceable`, and an
unsupported compatibility row. A plan left with nothing else is rejected with
`no_supported_components`. A client that declares the trait must implement
`clients.ExplicitSkillDecorator`, which may only add header fields and files;
the stager refuses an explicit-only command for a client without the trait or
without the decorator, refuses a decorator that changes name, description or
body, and requires `disable-model-invocation: true` in every explicit skill it
writes. That header field is the installer's own marker, checked on the
written bytes the same way for every client, not only Claude Code's
mechanism. The Codex decorator therefore keeps it next to
`agents/openai.yaml`, the file Codex acts on: Codex's skill loader ignores
header fields it does not know, so the marker changes nothing there, while
one client-independent check stays possible. Backend siblings must agree on
the trait. The trait is false for every
client until a probe with the real client confirms its mechanism.

**New rules for an unchanged package.** A package with commands depends on the
installer's rendering rules, so a newer installer can change its tree without a
new package revision. `Update` therefore runs `RefreshProjection` before the
ordinary apply when the installation has no environment authority and its
binding is materialized at the same revision. That refresh acts only when the
staged digest differs from the recorded one and otherwise leaves files and
state alone. A preview cannot know the answer without staging, so the preview
of such an update, single or grouped, is never unchanged: hosts and
`update --all` that skip unchanged previews still run it. Refresh authorizes
only `repair`, which a saved-dependency sync never grants, so environment
bindings pick up new rules at the next pin change.

**Repair keeps recorded bytes.** For an intact package with commands, a
confirmed single-target `Service.Repair`, which the embedded installer uses,
first stages under the current rules. If the digest differs from the recorded
one, it stops with `usecase.ErrProjectionRefreshRequired` and leaves the change
to update. A group repair, which `agentplugins repair` uses, compares every
staged digest the same way before it commits anything.

**Other operations keep the installed tree.** A repeated add, a resumed
activation and an unchanged group target rebuild a native projection from the
installed tree on clients that track native objects. The stager's read-only
projection first compares the installed command skills with what the current
rules render, byte for byte, including skills of commands that are no longer
delivered, and fails with `domain.ErrProjectionRefreshRequired` (the same
value as the use case error) on any difference. A dry-run add and a group run
this check before any effect. The embedded installer reports the error as
`ErrUpdateRequired`, with a conflict outcome and the reason
`update_required`.

**Frozen client family.** Visual Studio Code is frozen by an owner decision
of 2026-10-16, so its adapters do not change for commands. It installs through
the GitHub Copilot CLI backend and shares one physical package with Copilot
CLI; group operations coalesce the two plans and either client's update
restages that package. Withholding commands from VS Code alone would need two
different trees for one package. The client registry therefore carries a
declarative `CommandSupport` entry, empty by default and unsupported for both
members of the github-copilot family, and backend siblings must agree on it.
The planner withholds every command from such a client with the warning
`command_not_supported_by_client`, exactly like a withheld explicit-only
command; the rest of the package is unaffected.

**Security review of the delivered view.** When a package has commands, the
security evaluator scans the snapshot plus the rendered command skills, keys
its cache by that view, and does not accept a signed-index verdict, which does
not cover commands.

## Consequences

- Authors write a command once and get the same skill name everywhere; the
  package remains a valid Agent Plugins 1.0 package.
- Until a client's trait is enabled, explicit-only commands reach no client.
  This is deliberate: a withheld command is safe, an ordinary skill the model
  can pick on its own is not.
- An update of a package with commands always considers a refresh, so an
  embedded installer's preview of such an update requires confirmation.
- The namespace and the `<slug>-<name>` rule are now part of the authoring
  contract. Changing either later breaks installed names and package layouts
  and needs a minor release with a migration guide.
- The installer has two kinds of input for one installed tree: the package and
  its own rules. Rendering changes must be called out in the changelog with the
  instruction to run `agentplugins update`.
- A package that is damaged or missing while the rules also changed cannot be
  repaired; remove and add it again.
- On clients that track native objects, a saved-dependency binding whose
  command skills predate the current rules cannot be updated at the same pin:
  update and an unfinished sync stop with `ErrProjectionRefreshRequired` until
  the pin changes.
- OpenCode expands `@path` file references in skill text. The body rules do
  not reject them, so such a reference reads a file in OpenCode only. This is
  a known limitation, not a format rule.
- GitHub Copilot CLI receives no commands while Visual Studio Code is frozen,
  although nothing about Copilot CLI itself requires it. The change is
  reversible: removing `withoutCommands` from both registry entries delivers
  commands to the family again, and `agentplugins update` then adds the
  command skills to installed packages.
- The embedded `installer` facade does not scan packages itself, so a host that
  embeds it gets a command-aware review only if it supplies an assessment.

## Non-Goals

- Argument placeholders, shell injection, root variables or per-client fields.
- Aliases, nested command directories and per-client skill names.
- Client-native command files (Gemini CLI TOML, Cursor `commands/`).
- Converting authoring trees under `plugin/targets/<client>/commands/`.
- Enabling any client's explicit-invocation trait; each client needs its own
  probe and its own change.
- A command-aware signed Directory index.

## Rejected Alternatives

- **Authors generate client files.** Every author would carry the per-client
  rules and would have to republish when a client changes. Rendering in the
  installer keeps one owner of that knowledge.
- **A pre-rendered skill in the package.** It duplicates the source and cannot
  follow client changes without a new package revision.
- **Commands as JSON under `extensions`.** Markdown with frontmatter is what
  authors and clients already use for skills.
- **A separate component kind `command`.** Every client receives a skill, and a
  new kind would ripple through every iterator over components for no
  difference in delivery.
- **Per-client skill names.** One name keeps documentation, support and
  collisions predictable.
- **An open frontmatter that drops unknown fields with a warning.** A field one
  client honors and another ignores silently changes what the command does.
- **Delivering explicit-only commands as ordinary skills with a warning.** The
  model could run them uninvited, which is exactly what the author ruled out.
- **Intersecting the trait with adapter capabilities in the planner.** The
  decision would have two inputs that can disagree; the trait plus parity tests
  and fail-closed staging keep one.
- **Refreshing after a no-change apply.** Clients with read-only native
  projection verify the installed tree before that point and fail on a tree
  that misses the new skills.
- **Refreshing every package on update.** Packages without commands do not
  depend on rendering rules; refreshing them would only add cost and
  confirmation prompts.
- **Letting repair apply new rules.** Repair would lose its meaning of
  restoring recorded bytes and could hide a rule change behind a repair.
- **Granting environment bindings a cloned `repair` authority for refresh.**
  It would weaken the exact-action guard of saved-dependency sync.
