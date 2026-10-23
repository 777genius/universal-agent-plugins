# Cursor project component evidence

## Pre-implementation evidence, 2026-10-23

The reviewed implementation selects project components. Official documentation
confirms `.cursor/mcp.json` for project MCP and `.cursor/skills/` for project
skills. User MCP remains `~/.cursor/mcp.json`; native discovery also considers
other skill roots, including `.agents/skills` and compatibility directories.
The MCP CLI documentation exposes interactive listing, connection status and
project/global configuration source. Enablement and tool approval remain native
client decisions. No enable, approve, trust, login or model command is invoked
by this adapter.

Sources checked before coding:

- <https://cursor.com/docs/mcp>
- <https://cursor.com/docs/skills>
- <https://cursor.com/docs/cli/mcp>
- <https://cursor.com/docs/plugins>

The plugin UI documents project and user install scopes, but this implementation
does not infer a stable unattended plugin-install interface from that UI. It
retains the existing user editor package preparation path and uses exact owned
project skill directories and MCP members for project requests. It does not
write user configuration or hooks, redirect HOME, or detect an ambiguous `agent`
binary as the editor.

## Evidence boundary

Pinned official CLI artifact/help research is recorded below.
Production provider/loader/planner/stager contract tests will be recorded below;
they do not prove that a real editor or Cursor Agent discovers, enables or
executes these components. Project configuration success remains prepared with
installed verification, and activation attestation cannot upgrade that claim.
Native session discovery, policy, approvals, aliases and MCP initialization
remain unobserved until the existing hosted native-client workflow qualifies
that exact client/version/platform.

## Pinned official CLI research

Parent provisioned Cursor Agent `2026.10.22-27dcf5f` directly from the official
installer's download metadata, without executing the installer:

- URL: <https://downloads.cursor.com/lab/2026.10.22-27dcf5f/linux/x64/agent-cli-package.tar.gz>
- Measured archive SHA256: `85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294`.
- The installer did not provide an independently authenticated digest; this is
  a measured acquisition pin, not a vendor signature claim.
- Actual version, top-level help and MCP help exited zero in a networkless
  disposable namespace with fresh HOME/project and no user mounts. These are
  read-only research probes, not native component discovery qualification.
- MCP help explicitly names `.cursor/mcp.json` and `~/.cursor/mcp.json` and
  exposes list/list-tools/enable/disable/login. No enable/login was invoked.
- Current top-level help also exposes `plugin`, `--plugin-dir`, and `--workspace`.
  No assertion that Cursor Agent lacks plugin CLI support is made. This stack
  retains the reviewed component lane; plugin installation is a separate native
  delivery contract requiring its own scoped ownership qualification.

The project component lane does not add ambiguous `agent` executable detection
or claim that agent-only detection selects the editor. Existing editor and
Cursor Agent detection surfaces remain distinct.


Follow-up actual plugin help exposed only the `marketplace` child. Requests for
install/list/uninstall `--help` printed the same ancestor help and did not
establish those lifecycle verbs or a project scope grammar. This component
implementation therefore does not claim that CLI plugin install/remove was
qualified by those probes.

## Implementation and contract verification

The adapter qualifies the catalog's Cursor project scope using native file
configuration lifecycle traits while preserving user manual preparation.
ProjectTargetRoot selects managed artifacts under project `.uap`; NativeRegistry
selects exact `.cursor`. It validates original detected user root and matching
scope/target/native root throughout plan, inspect, activate, verify and remove.
Project binding validation accepts only the selected managed artifact and exact
native ownership paths. The shared artifact receipt is handled by core; the
component helper receives only native ownership, retaining unknown kinds for
closed refusal.

The new shared component helper has two independent plan reviews and two
independent implementation reviews in this directory. It uses the existing
nativeconfig Kernel, exact receipts, verified directory swaps and rollback;
there is no second settings/discovery engine. The project stdio projection
uses the existing trusted launcher and binds absolute package/data/cwd inputs.
User package projection and legacy attestation behavior retain their existing
contract. Project attestation is refused before generic provider shortcuts.

Author verification includes complete Cursor/shared/domain package tests
(PASS 0.189s/0.078s/0.038s), project provider/planner contracts and unchanged
user activation goldens (PASS 0.359s/0.014s). The task host carried duplicated
uppercase/lowercase proxy environment names; the pre-existing Codex user
validator rejects those even when discarded. Removing only duplicated lowercase
proxy names for this inert golden check restored its expected existing behavior;
no production or golden change was made for that unrelated host condition.
Composed Cursor tests load standard plugin.json, render a nonrestricted command
skill, stage production projections and drive production providers. They cover
remote and stdio MCP, read-only active reprojection, repeat/noop/update,
missing-same-pin repair, dropped MCP, repeat removal and persistent data,
foreign disabled entries/hooks/skills, malformed config, lock failure rollback,
user/project-A/project-B byte and mode inventories, scope/root/copied-context
mismatch, contained native aliases and direct/provider attestation refusal.
The injected launcher bytes are inert and never executed.

Both exact changed-code and full-file size/architecture lint gates pass without
new suppressions. The compatibility golden delta changes only the newly
qualified Cursor Scopes field from `[user]` to `[user, project]` in four reports.
Actual native session discovery, MCP handshake/tool result and fresh restart
qualification remain pending the hosted native-client matrix; none of these
fixture or help probes is presented as that evidence.

## Composed source lifecycle checkpoint

The Cursor stack was aligned with reviewed integration `d9f3f757` in local
merge `12177d41`. Dedicated Cursor CLI tests reuse that integration's real
loopback TLS Git and ZIP acquisition transports and its CLI/state inspection
helpers. The scoped CLI composer, dependency solver/manifest/lock, immutable
store, installer, planner, stager and native configuration writer are production
implementations. Only host detection and package security assessment are inert
fixture boundaries. Remote MCP entries avoid executing configured commands or
copying the test executable as a managed launcher.

`TestCursorProjectComposedSourceLifecycle` covers both Git and ZIP sources:
saved project add in A and B, exact native skill/MCP paths, locked no-op sync,
list/info/doctor/outdated/dependency status/tree reads without native/state
changes, missing native skill repair, pin-advancing update that retains binding
authority, portable export and cold-cache import/install into a new project,
and exact removal. User Cursor configuration/hooks/skills and project B retain
their byte/mode inventories and recorded state while only A changes. The cold
restore retains the exact exported agreement and portable source/tree identity.
Project bindings stay prepared plus installed verification throughout. This
remote-only composed fixture does not allocate persistent stdio data; the
separate stdio provider contract remains the data/launcher evidence.

Final author verification for the composed checkpoint:

- Dedicated CLI Git/ZIP lifecycle: PASS, 5.865s.
- Existing project Cursor provider contracts after integration: PASS, 0.183s.
- Exact changed-code and full-file size/complexity/architecture lint gates:
  0 issues, without new suppressions.
- Git whitespace check: clean.

Independent final adapter R1 is GO in `cursor-code-review-r1.md`; final R2 and
review of the added composed journey remain separate gates. These tests do not
launch Cursor, execute a package payload or configured MCP command, write
trust/approval/login state, or issue a model request. Genuine Cursor discovery,
effective enablement and policy, MCP initialization/tool use, editor behavior,
fresh native restarts and release qualification remain unobserved.

## Independent R2 correction: selected data authority

The final adapter R2 found that read-only active reprojection ignored its
selected DataReceipt locator and could reuse old embedded stdio bindings.
Project active reprojection now refuses a different or absent selected data
locator with the existing projection-refresh-required error. It verifies the
managed launcher path, reserved package/data environment, and protocol
package/data/cwd/anchor arguments against the selected active package and data
receipt before producing native ownership. It uses the existing read-only cwd
resolver and never changes digest-covered bytes to hide a mismatch. User
projection and remote-only project entries retain their previous behavior.

The composed stdio provider regression supplies the correct bound locator for
the positive read-only case, then requires wrong/empty data locators to fail
with unchanged immutable package bytes. Full project Cursor provider selection
passes at this correction: **PASS 0.180s**. The independent R2 reviewer must
recheck this correction before closing the adapter review.
