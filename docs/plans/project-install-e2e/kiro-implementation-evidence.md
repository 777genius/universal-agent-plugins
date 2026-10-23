# Kiro project scope: pre-code research continuation

Prepared by cursor_clients, 2026-10-23. This continues the Kiro lane already
approved in both independent reviews of `client-plans.md`; it does not replace
or expand that plan. No Kiro implementation has started at this checkpoint.

## Current official evidence

Checked current official pages before coding:

- https://kiro.dev/docs/mcp/configuration/
- https://kiro.dev/docs/configuration/
- https://kiro.dev/docs/custom-agents/configuration-reference/
- https://kiro.dev/docs/skills/
- https://kiro.dev/docs/cli/
- https://kiro.dev/changelog/cli/

The MCP reference describes nested `mcpServers`, local command/args/env and
remote URL/headers, with workspace `.kiro/settings/mcp.json`. Whole workspace
entries override global entries, and custom-agent entries take precedence over
workspace entries. Disabled members and permissions remain native policy.
Custom agents use `includeMcpJson` to include those configuration files. UAP
must preserve these settings and cannot infer the active custom agent.

The skills reference confirms `.kiro/skills/<name>/SKILL.md`, global skills
under the user's Kiro directory, and custom-agent resources controlling skill
availability. Project configuration alone does not prove native selection.

The CLI changelog lists 2.33.1 on October 21. Its October 20 predecessor adds
interactive V3 workspace trust before folder configuration loads, recording
trust per workspace root under the user's `workspace-roots` directory. No
trust/default-engine/config migration is authorized or needed for file
installation. Native CLI observation must state its actual engine/version and
must not be presented as IDE/mobile/web/Powers qualification.

## Official package pin and measured version/help

Read the official installer script without executing it:
https://cli.kiro.dev/install
Local snapshot: `research/kiro-cli-install.sh`.

The script selects stable channel metadata from
https://prod.download.cli.kiro.dev/stable/latest/manifest.json
Local snapshot: `research/kiro-cli-stable-manifest.json`.

The retrieved manifest reports version **2.33.1** and gives an exact Linux
x86_64 GNU headless archive:

- URL: https://prod.download.cli.kiro.dev/stable/2.33.1/kirocli-x86_64-linux.tar.xz
- Vendor manifest SHA256:
  `e798062c22adb3be53ea1845f9b309b83df017376860f8329293f3c2954177cb`.
- Vendor manifest length: 130,643,852 bytes.

The archive was acquired from that exact versioned URL. Its measured length
and SHA256 match the vendor manifest. The complete official distribution was
inspected without running its installer: main CLI 42,879,904 bytes, companion
chat 205,977,448 bytes, and terminal 33,369,248 bytes, plus wrappers and metadata.
Main binary SHA256 is
`801e11f74e31d26f883f5f56cde96c006124c94ee8e063c58f45ad183f34f147`;
chat SHA256 is
`6c143853239d75ecbb51099ab1dc768c1a03707de6b9d5393fa3dacdf24bc036`;
terminal SHA256 is
`11b172eb92513bfda7524cb61b03042ca58b970680266388ca8c2bb2f3a45606`.
BUILD_VERSION is 2.33.1, build hash
`2d6b9103124a3b0dab1f4517a5c3400cd297ec36`.

Eight genuine read-only probes exited zero with a fresh profile/project in a
networkless namespace using empty /proc: --version, --help, --help-all,
mcp --help, mcp list --help, mcp add --help, acp --help and agent --help.
Version output is exactly `kiro-cli 2.33.1`. MCP scope help names default,
workspace and global. ACP defaults to v2 and exposes the separate v3 engine;
UAP will not silently change it. No list/session/model/MCP/auth/trust operation
was run. Initial private-proc launcher refusals are recorded as launcher
failures, separately from those successful help/version probes.

Provenance, archive and all logs remain under the task's
`tools/native-probes/kiro/2.33.1`. Reproducible extraction was reclaimed after
hash/help evidence capture; the original pinned archive is retained. These
version/help observations do not qualify project discovery, runtime, IDE or
any particular active custom-agent policy.

## Existing code reuse and project boundary

The current Kiro adapter already supplies exact native skill/MCP receipts,
JSONC-preserving MCP patching, locked ExactFile conditional replacement,
verified skill swaps and rollback. Project support should pass an explicit
validated project native root into those services, preserving the original
DetectedClient.ConfigRoot. It must not disguise the project as a user profile.

The existing user automatic lane also performs ACP session verification.
Project file installation requires a separate conservative prepared outcome:
do not reuse user ACP success as project proof, invoke a user-only session by
accident, or allow generic activation attestation to upgrade discovery. A
project inspector must validate binding scope, selected native root, exact
object kinds/paths/digests and selected artifact before calling the existing
native mutation. Unknown ownership must fail closed; only the core-managed
package receipt belongs outside the native writer.

The approved source scope remains project `.uap` for package/state/data plus
`.kiro/settings/mcp.json` and `.kiro/skills` for native files. User lifecycle,
legacy source and engine distinctions remain intact. Native helpers named
ActivateNative/DeactivateNative currently use request.Client.ConfigRoot;
project callers must select an explicit root instead of reaching those user
wrappers unchanged. Project prepared intent must remove the current user-only
assumption only on its branch. Active reprojection must read and validate the
bound package/data locators without rewriting immutable bytes.

## Implementation checkpoint

The original Kiro section of client-plans.md and its two independent GO reviews
remain the pre-code authority. This evidence checkpoint precedes production
changes. The Kiro leaf true-merges reviewed integration daa705ad before its own
changes. It adds an explicit project native root and read-only scope/receipt
validation, reuses Kiro's existing exact native transaction, and binds stdio to
the existing trusted launcher and selected package/data receipts. Project
activation remains prepared with installed-file verification; generic
activation attestation cannot upgrade it. No workflow guard or native runtime
proof claim is added by the file adapter. Separate independent implementation
reviews and composed source lifecycle checks follow the code checkpoint.

## Implemented project lane and verification

Project support is additive to the original user lane. The selected workspace
owns `.uap` package/state/data plus `.kiro/settings/mcp.json` and
`.kiro/skills/<name>`. The detected global profile is retained independently.
Explicit project root/layout, selected artifact, binding scope/native root and
exact receipt identities/portable paths are validated before mutation; copied,
unknown, duplicate, nonclean or foreign ownership refuses. The existing Kiro
JSONC ExactFile transaction and skill swap/rollback service remain the only
native mutation mechanism. Existing global CLI/ACP behavior is unchanged.

The project selection renders Kiro's documented command/url shape without the
portable transport type member. Stdio uses the existing trusted launcher to
establish cwd and receives absolute selected package/data locators. Active
reprojection validates launcher, environment and protocol package/data/cwd
arguments against the selected data receipt without changing immutable bytes.
Missing or different data authority requires projection refresh. Project
activation and verify-only both remain prepared with installed-file evidence;
activation attestation cannot turn that into native discovery. Prepare intent
uses only this explicit project branch and does not relax the user lane.

Independent implementation review is pending at this code checkpoint. Bounded
author verification includes:

- Existing Kiro user test selection: PASS 1.364s; compatibility goldens PASS
  0.009s. Only Kiro's declared project scope changes in the four capability
  goldens; the other client entries remain intact.
- Production loader/planner/stager/provider fixtures: PASS 0.243s, covering
  remote and stdio bindings, A/B/user isolation, repeats, update/removed MCP,
  missing-skill repair, native root aliases, copied scope/root/bindings,
  malformed and disabled foreign collisions, missing writer/launcher,
  forged/duplicate/unknown receipts and activation-attestation refusal.
- Public installer SDK stdio lifecycle: PASS 0.251s. Separate A/B data receipts
  and launcher roots, no-effect repeat update, repair from the installed pin
  despite a moved source, fresh installer update, inspection and exact remove
  retain data markers and leave user/B ownership unchanged.
- Project native fault cases verify visible-write error rollback restores
  original bytes and removes staged skill effects; committed lock-cleanup
  degradation retains exact receipts and the conservative prepared status.
- Production CLI TLS Git and ZIP source journeys: PASS 5.980s. Add both projects,
  locked no-op sync, read-only views, repair, update identity preservation,
  exact portable export and cold import, owned removal and scope isolation.
  The test starts TestProjectKiro so the existing cross-platform project gate
  selects it. Source HTTP/security boundaries are inert test seams; no native
  Kiro, launcher, MCP, model or trust process is executed by these fixtures.

No Kiro project native-discovery/runtime qualification is claimed. Genuine
pinned version/help is separate from configured-file and public-API evidence.
The existing hosted native guard is untouched, no account/trust migration is
performed, and CLI evidence cannot qualify the IDE. Native source qualification
must still observe the selected actual workspace/agent and interpret permissions,
includeMcpJson and resources rather than infer effective policy from these files.

## Independent review correction: retained user context

R1 identified that project root validation accepted an omitted user context
when the detected original profile was nonempty. The project lane now requires
exact equality, including empty/nonempty mismatches; both empty remains valid.
An omitted-user regression verifies refusal before native effects alongside
the existing wrong-user/scope/root/copied-project cases. This retains user
profile authority explicitly even though the file-only lane never mutates it.
