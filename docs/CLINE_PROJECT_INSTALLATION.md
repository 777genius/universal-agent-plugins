# Cline project component preparation

Current source; not a released or native-runtime qualification statement.
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

These are structured instructions for review; UAP does not execute that Cline
session, run a prompt, connect a server, grant trust or alter authentication.
The explicit MCP file selects the whole source. User registrations are not
merged or copied into that session. A later process without the override reads
its ordinary user settings again. IDE forwarding is a separate unverified path.

The historical hosted Cline 3.0.73 research positively observed project skill
paths and explicit MCP file selection. Its raw output and exact source closure
are [preserved here](evidence/project-scope-research/cline-3.0.73-linux-amd64-40017958131/README.md).
That collector did not run UAP's project lifecycle. New current-source hosted
product evidence remains required, including honest same-name precedence limits.

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

The production CLI composes the namespace-aware observer used by the Cline
project tests. The existing SDK `EnableNativeObserver=false` default/opt-out
currently falls back to observing the managed package only. Component mutation
still preserves foreign files, but a native component collision can then be
reported after managed-package delivery with an incomplete state. This generic
composition gap is under separate integration review; the opt-out path is not
qualified for the stronger pre-delivery collision guarantee. No native discovery
or command execution should be needed to perform local ownership checks.
