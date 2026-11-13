# Cline project component preparation

Explicit project preparation ships in CLI 0.1.93. The observations below do not
qualify arbitrary native runtime behavior.
Project installation writes owned skill directories to `.cline/skills/` and
prepares selected MCP registrations in `.uap/clients/cline/mcp.json`. The managed
package, state, data and receipts belong to the selected project's `.uap`
namespace. Original user Cline configuration remains a separate authority.

Results remain **Prepared** even when the exact installed component files are
verified. File verification does not prove native session discovery, effective
trust, IDE behavior, enabled skill execution or MCP initialization. Activation
attestation cannot turn this project preparation into native activation.

## Use the prepared configuration

Start Cline in the selected project and inspect its skills. For MCP, the action
returned by add/repair specifies the exact working directory, argument array
including `--auto-approve false`, and process environment values:

- `CLINE_MCP_SETTINGS_PATH`: the selected project's absolute prepared MCP file.
- `CLINE_NO_AUTO_UPDATE`: `1` for the suggested process.

Add/repair returns these instructions for review and does not start Cline as an
installation side effect. The source checkout also provides the explicit `run`
command below. Startup consent grants no workspace trust, automatic tool
approval or authentication.
The explicit MCP file selects the whole source. User registrations are not
merged or copied into that session. A later process without the override reads
its ordinary user settings again. IDE forwarding is a separate unverified path.

The historical hosted Cline 3.0.73 research positively observed project skill
paths and explicit MCP file selection. Its raw output and exact source closure
are [preserved here](evidence/project-scope-research/cline-3.0.73-linux-amd64-40017958131/README.md).
That collector did not run UAP's project lifecycle. The separate
[current-source Cline 3.0.73 observation](evidence/project-scope-research/cline-3.0.73-current-source-linux-amd64-40182752894/README.md)
now records public UAP user/A/B installation, replay, update, repair, removal
and read-only native metadata at its exact source. It observed enabled MCP
registrations in the actual prepared files and unique project skill paths.
Same-name user/A/B skill reads selected the user skill in that table, so a
project receipt does not promise the same-name project skill is effective.
Project activation, MCP initialization and independent active-session versions
remain unqualified.

## Explicit startup from a source checkout

The additive `run` command is implemented in the source checkout for
[PR #878](https://github.com/777genius/universal-agent-plugins/pull/878).
The CLI 0.1.93 release statement above covers project preparation; it does not
announce published release availability of this startup command. Run these
examples from a checkout containing that implementation, using the toolchain
declared in `go.work`, and replace the absolute project path with your project.
An unambiguous detected Cline CLI and an existing UAP-owned Project binding with
supported stdio MCP servers are required. A Skills-only project uses ordinary
Cline startup; this command does not create an MCP source for it.

Review the selected project's owned MCP servers without starting a session:

```sh
go run ./cli/cmd/agentplugins run \
  --target cline --scope project --project-root /absolute/path/to/project \
  --dry-run
```

Explicitly start the interactive Cline session with that Project MCP source:

```sh
go run ./cli/cmd/agentplugins run \
  --target cline --scope project --project-root /absolute/path/to/project \
  --confirm-client-start --format human
```

To run native configuration inventory instead of the interactive session:

```sh
go run ./cli/cmd/agentplugins run \
  --target cline --scope project --project-root /absolute/path/to/project \
  --inspect --confirm-client-start --format human
```

Installation `--yes` does not authorize startup. `run` requires one explicit
target and Project scope; actual startup requires human output and
`--confirm-client-start`, including for `--inspect`. For this Cline route, that
consent authorizes a native `config mcp --json` verification process followed by
the selected client command. The second process is an interactive session by
default, or another configuration inventory with `--inspect`. A failed
verification prevents the second process. Extra child arguments are not accepted.
`--dry-run --format json` is available for a structured preview; a dry-run starts
neither process.

Before startup, UAP verifies package/component ownership and the whole selected
MCP file, checks native inventory against that exact source, and rechecks the
plan. Missing or edited components, foreign MCP members, disabled servers,
automatic tool approvals and pending installation recovery refuse startup.
The child uses `--auto-approve false`, `CLINE_MCP_SETTINGS_PATH` and
`CLINE_NO_AUTO_UPDATE=1`. UAP does not rewrite the original User configuration,
the parent shell environment or IDE settings. A later ordinary `cline` process
without these overrides continues to select its ordinary User source.

Receipts remain **Prepared** after startup or inspection. `--inspect` observes
configuration without a model or MCP call. A direct call through the installed
MCP entry proves a separate transport path; it does not prove that Cline itself
connected or called the tool. IDE behavior and native Cline tool calls require
their own evidence. The command contract lives in
[client_session.go](../cli/internal/agentpluginscli/client_session.go), with
[Cline planning](../install/integrationctl/agentplugins/clients/cline/session.go)
and [native inventory verification](../install/integrationctl/agentplugins/clients/cline/session_verify.go).

## Ownership and maintenance

UAP merges only its recorded MCP members and swaps only its recorded skill
directories. It preserves unrelated settings/members, the user profile, other
projects and persistent plugin data. Project paths do not follow ambient
`CLINE_MCP_SETTINGS_PATH` or `CLINE_DATA_DIR`. Project/profile overlap, aliases,
foreign targets, changed owned bytes and inconsistent receipts are refused.

Repair can restore a missing exact owned component or rematerialize the pinned
managed package. An externally edited native skill or MCP member is preserved
and requires review rather than being silently overwritten. Removal verifies
recorded file ownership and does not invoke native/global uninstall. Existing
sessions can retain configuration until restarted.

## SDK host admission boundary

Both the production CLI and the default SDK compose mandatory local ownership
checks for project component delivery. Setting `EnableNativeObserver=false`
disables optional live native discovery; it does not disable exact skill/MCP
file checks. Those local checks require no client runner and preserve Prepared
semantics. Foreign names or changed owned files are refused before managed
publication, including after a fresh SDK instance reopens the project.

The installed-file observer and a native client's effective configuration are
different evidence. Neither the default SDK nor enabling a live observer can
turn Cline's project preparation into an active session through attestation.
