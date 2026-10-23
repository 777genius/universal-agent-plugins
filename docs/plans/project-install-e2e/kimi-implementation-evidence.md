# Kimi Code project component implementation evidence

Implementation owner codex_claude, 2026-10-23. The original current-Kimi client
plan and its two independent corrected GO reviews are retained. This checkpoint
records pre-code official artifact/help and exact-tag source evidence supplied
by root; it is not native project/runtime qualification.

## Pinned artifact and genuine help

- Product: current MoonshotAI Kimi Code **2.3.1**, not archived kimi-cli.
- Official release: https://github.com/MoonshotAI/kimi-code/releases/tag/%40moonshot-ai%2Fkimi-code%402.3.1
- Exact official source tag commit: `684bae9ea6a437f3bae98bba1dcb797a917d500b`.
- Archive: https://github.com/MoonshotAI/kimi-code/releases/download/%40moonshot-ai/kimi-code%402.3.1/kimi-code-linux-x64.tar.gz
- Vendor GitHub asset and measured archive SHA256:
  `0dbc7d771ce4d761acc150ade72325bcd8fc51973d481f7040295f4cf062fdbe`;
  65,922,255 compressed bytes, exactly one regular `kimi` member.
- Binary: 192,285,888 bytes, SHA256
  `781ec1c24e5e79a01aaaa31ad859e8818ae11ffe792e93645cdc0c068a264ba0`.
- Genuine bounded `--version`, `--help`, `doctor --help`, `acp --help` all exited0
  in root's fresh HOME/XDG/project, networkless `bwrap --unshare-all` with
  read-only verified binary/system libraries and empty proc. Version stdout is
  exactly `2.3.1`. No account, trust, model, MCP or plugin mutation occurred.
- Logs/provenance remain under `tools/native-probes/kimi/2.3.1` in the task
  evidence area. The hash-verified extracted copy was reclaimed; do not download
  it again merely to reproduce help. No standalone read-only MCP/skill listing
  exists in the captured help, so the adapter stays prepared after file delivery.

## Exact-tag documentation and implementation

All following sources are at commit `684bae9ea6a437f3bae98bba1dcb797a917d500b`
in https://github.com/MoonshotAI/kimi-code :

| Exact source path | Inspected contract |
| --- | --- |
| `docs/en/customization/mcp.md` | Working-directory `.kimi-code/mcp.json`; user `.kimi-code` or KIMI_CODE_HOME; project overrides user; workspace trust required; open-session removal differs from fresh-session absence |
| `docs/en/customization/skills.md` | `.kimi-code/skills` plus `.agents/skills`; nearest `.git` project root; frontmatter name/description; extra directories can affect effective selection |
| `docs/en/customization/plugins.md` | Plugin registry remains user-only; project components must not register a project plugin there |
| `packages/agent-core-v2/src/app/mcpConfig/configLoader.ts` | Merge user mcp.json, nearest Git worktree root `.mcp.json`, then selected cwd `.kimi-code/mcp.json`; project can be excluded when untrusted |
| `packages/agent-core-v2/src/mcpCore/config-schema.ts` | Explicit `transport:stdio/http/sse`; inference from command/url when absent; env/cwd or headers; enabled/deferred/tools/timeout policy |
| `packages/agent-core-v2/src/features/skill/catalog/skillRoots.ts` | Upward nearest `.git` existence controls project skills; branded `.kimi-code` and generic `.agents` roots; realpath normalization |
| `packages/agent-core-v2/src/features/skill/workspace/rootFileSkillSource.ts` | Root-file skill discovery and implicit context require conservative status |
| `packages/agent-core-v2/src/workspace/workspaceMcpConfig/workspaceMcpConfigService.ts` | Workspace trust gates loading and configuration changes |
| `packages/agent-core-v2/src/workspace/workspaceTrust/workspaceTrustService.ts` | Trust is independent persisted workspace authority; installer must never grant it |

Root's provenance records each retrieved blob ID and SHA256; the reviewer can
reproduce by exact commit rather than a mutable website. These are static source
observations, not a successful native list/connection.

## Narrow implementation decisions

Deliver exact project skills and MCP entries through reviewed
`shared.ProjectComponents` and the existing transaction/native-config kernel.
Native root is `<ProjectRoot>/.kimi-code`; managed immutable package remains
`<ProjectRoot>/.uap/managed/clients/kimi/<artifact>`. Preserve detected original
user root, user plugin layout/registry/lifecycle, and KIMI_CODE_HOME. Do not
emit a project plugin manifest or invoke user registration.

Only the current exact observed2.3.1 `kimi`/`kimi.exe` executable/version pair
qualifies new project install/update/repair. Use the separately reviewed shared
optional ClientVersion propagation once its frozen implementation has two GO;
no new probe belongs in Kimi's adapter or SDK Discover. Layout/reservation and
exact receipt removal/recovery remain independent of a running current binary.

Use a narrow project-only native codec for explicit stdio/http/sse transport
keys, with no implicit user capability expansion. Stdio uses the existing
trusted managed launcher and validates immutable package/data/cwd/environment
against the selected receipts during active reuse. Preserve unsupported
components as unsupported; rendered command skills follow existing projection.

A selected skill project must be its nearest `.git` ancestor (a regular gitfile
or actual directory, never a symlink). Reject a different ancestor rather than
silently installing skills where Kimi will not scan. Without any `.git`, fail
closed for the automatic skill lane until exact fallback semantics are further
qualified. No ancestor file is mutated. User/root compatibility MCP sources
are lower priority but remain visible as warnings/collision context; disabled
or foreign selected entries cannot be adopted or enabled. Effective extra skill
roots, aliases, session flags, trust, loaded MCP and authentication are unobserved.

Configuration materialization returns prepared + installation-verified; it
never claims native loaded/runtime-active. Activation attestation is refused
before the generic composed-provider branch, including direct activation.
No KIMI_CODE_TRUST_WORKSPACE, --auto, --yolo, archived entrypoint or model prompt
is used to manufacture evidence. Exact removal only consumes recorded component
receipts and leaves user/B/foreign settings, retained data and package store.

## Verification checkpoint

Pending implementation: production provider boundary and public CLI/SDK
lifecycle tests for selected A versus B/user, exact roots/artifacts/receipts,
wrong data/rebound contexts, foreign/disabled collisions, nearest-Git mismatch,
current/missing/wrong version evidence, prepared/attestation refusal, rollback,
locked no-op/update/exact repair/recovery/removal and cold portable restore.
Two independent final implementation reviews are required before publication.
Genuine native project discovery/trust/session/restart/MCP/data evidence remains
pending; passing fixture composition does not change that boundary.

## First implementation checkpoint (not review/publication-ready)

The reviewed shared host contract64c7ee6d was incorporated exactly as3e072c99;
its two independent code GO reports are preserved alongside this evidence.
Project adapter now has scoped current-version/nearest-Git admission, dedicated
stdio/http/sse codec, exact package/artifact/native-root and original-user-root
fences, selected immutable stdio data/launcher validation, project-only SSE
qualification, component receipts and the reviewed native transaction helper.
User plugin behavior is unchanged. Project state stays prepared/installed and
attestation cannot override unobserved trust/discovery.

Bounded checks before this checkpoint: full existing Kimi client suite
PASS0.014s; full nativeconfig suitePASS3.331s; new composed project provider
suitePASS0.069s, then with current-harness/nearest-Git and version-independent
removal casesPASS0.101s. Public saved TLS Git lifecycle reached the first real
installation but could not copy the trusted launcher because task temporary
storage returned ENOSPC; this is a resource failure, not a passed public E2E.
Root is coordinating headroom before rerun. Source files were then mechanically
split by responsibility without broad recompilation. The final public SDK
stdio lifecycle, public CLI successful run, lint and two final code reviews
remain pending; this commit is a recoverable implementation checkpoint only.

## Final authored implementation verification checkpoint

After root restored only task-owned reproducible cache headroom, the real public
saved TLS Git lifecycle passed: A/B installation, locked no-op, exact installed-
pin repair after immutable/native drift while the source advanced, fresh-state
update/status, bounded pending-journal recovery, exact removal/repeat refusal,
retained data and unchanged user/B. A separate cold HTTPS ZIP export/import
restored the exact agreement and source identity into a fresh content cache.
The existing no-save local route also passed through production CLI composition
with no environment receipt, selected current host metadata, prepared/installed
project binding and exact removal preserving user/B. No fake native executable
was run; detector/security/trusted-launcher bytes are explicit inert test seams.

Public SDK tests passed A/B distinct selected stdio/data receipts, exact
launcher/args/environment/data cwd projection, retained data markers, locked
no-op, caller-supplied exact-pin repair after source advance, fresh Engine
restart/update/inspection and exact removal without current version/executable.
The launcher is copied trusted inert bytes and never executed in this fixture;
this does not establish a genuine MCP initialize/tool/data workload.

Additional provider coverage passes current/missing/older/prerelease/wrong
harness refusal, nearest-Git mismatch with unchanged inventories, version/Git-
independent owned removal, malformed/disabled/foreign/symlink/stale native
ownership, scope/artifact/delivery mismatch, immutable selected-data refusal,
project-only SSE with preserved user SSE policy, and same-name lower-priority
user/nearest-Git compatibility entries preserved byte-for-byte. Their presence
is exposed as a warning; configured trust/effective loading stays unobserved.

Final bounded results:

| Check | Result |
| --- | --- |
| Public saved Git + cold archive import | PASS7.695s |
| Public imperative local/no-save | PASS0.705s |
| Public SDK project stdio/data lifecycle | PASS0.187s |
| Project composed provider boundaries/lifecycle | PASS0.121s |
| Existing Kimi user adapter suite | PASS0.014s |
| Full nativeconfig suite including Kimi codec | PASS3.331s |
| Compatibility planner/goldens | PASS0.013s |
| Exact changed-code lint2.13.2 | Zero issues |
| Whole changed-file size/complexity/architecture lint2.13.2 | Zero issues |

An overly broad initial test expression also selected an existing Codex/Claude
user sibling-compatibility test and hit the known duplicate inherited proxy
environment rejection; that same exact existing test passed0.291s after removing
only ambient upper/lowercase proxy variables for its child environment. It is
not recorded as a Kimi failure or hidden as native qualification. A new test
fixture initially omitted the required planner path policy; it was corrected
without changing production admission.

The final implementation is frozen for two independent code reviews. Genuine
Kimi project discovery, folder trust, skill invocation, MCP initialization/data
and fresh-session removal remain pending. Neither these tests nor the genuine
help-only artifact justify active/attested status or all-client/release claims.

## Independent R2 corrections

After the byte-preserving component-base merge at `83c85702`, independent R2
identified two narrow omissions. The direct project profile validator now
requires the literal Kimi client ID before accepting an otherwise exact native
binding. A copied Codex-tagged binding refuses with unchanged inventories; the
higher-level inspector already rejected this mismatch.

The dedicated harness-version admission test also lacked the required planner
path policy. It now supplies that policy, positively proves the selected current
2.3.1 harness reaches `PlanReady`, and requires each missing/old/prerelease/wrong
executable case to report the current-harness admission reason. This prevents
an unrelated earlier planner failure from satisfying the negative assertions.
Both focused regressions passed0.016s, and the complete project Kimi provider
selection passed0.139s. Production version, scope, trust and prepared-status
contracts are unchanged.

Both independent final implementation reviews are GO at corrective source
`95b5f34cc9fb7796e79e0a76e3b853d8a8ce5386`, after the component-base merge and
both narrow R2 corrections. Their full reports preserve earlier independent
composed passes and separately identify final independent retries that failed
before assertions from actual ENOSPC. Those retries are not reported as passes;
the actual corrected author provider and lint results above remain separately
attributed. Parent requested no redundant compilation during the shared resource
shortage. No native Kimi project or runtime qualification is added by approval.
