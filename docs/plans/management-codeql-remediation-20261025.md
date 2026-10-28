# Management CodeQL remediation, 2026-10-25

## Scope and evidence

The owner supplied the exact mapping of all 17 open alerts in UAP PR #500 and
Agent Teams PR #989. Reported line numbers refer to UAP merge
`893460f3742e70cd4ff76218483819dbc0835c0c` and App merge
`d363df3c770378e58c58889a9296cea33b537264`. Baseline executable/SDK source is
UAP `8645eb5e71321f9321b8e742b8eda74408b55f13`; baseline App source is
`7667b158c368b07655fb1d0c288b8e88efd21587`.

The previous review's blanket false-positive conclusion is withdrawn. The
current review reconstructs callers, guards and sinks from source. Full SARIF
code-flow edges are unavailable through the connected tools, so this record
does not claim that those unavailable edges were inspected. Alert IDs and
original locations below come from the owner's current GitHub report.

## Reviewed correction plan

1. Correct package-link resolution in the existing snapshot validator. Resolve
   a symbolic link before processing the following parent component; retain
   bounded expansion and reject missing or non-directory intermediate paths.
   Keep valid internal links, exact link text and the existing digest format.
2. Remove the unnecessary local filesystem probe from Management's rejection of
   local plugin archives. Use the existing offline source grammar; preserve the
   ordinary CLI's local-source capability.
3. Constrain directory-swap journal reads with Go's standard rooted-file API,
   verify a regular file and decode from the same descriptor. Preserve receipt
   format, recovery ownership checks and supported payloads.
4. In App's UAP desktop workflow, remove the privileged manual-dispatch entry
   point, keep normal pull-request qualification, and deny cache authority with
   `cache-mode: none`. Verify the effective mode before executing checked-out
   code. Existing workflow reruns remain available.
5. Replace dynamic renderer expression construction in the App E2E harness with
   a fixed CDP function and protocol arguments. Labels, selectors, IDs and
   timestamps remain data. Preserve real pointer/keyboard input, operation
   identity checks, bounded navigation and native MCP assertions.
6. Review every remaining finding independently. Preserve intentional private
   HTTPS publishers and native-client execution selected by trusted startup;
   do not invent a network-isolation guarantee or rewrite the command framework
   merely to alter a scanner result.
7. Push each commit immediately, build the matched SDK/native source pin, run
   new CodeQL and applicable current-source CI, and inspect the aggregates.
   Historical successful runs do not qualify the changed engine or harness.

## UAP: input, guard, sink and disposition

All paths below are relative to the UAP repository. The process owner supplies
startup roots and executables. The authenticated HTTP contract supplies source
intent, captured target IDs and retained operation identities; it exposes no
arbitrary package root, executable, argv or environment field. Authentication
does not make downloaded package trees trusted.

| Alert | Reported sink | Reconstructed path and current disposition |
| --- | --- | --- |
| [45](https://github.com/777genius/universal-agent-plugins/security/code-scanning/45), Critical | `sourceacquisition/skill_web.go:178`, HTTP GET | `SourceRequest.Source` -> website catalog -> credential-free normalized HTTPS publisher -> fixed well-known index and same-publisher derived file URLs -> `client.Do`. Redirects, arbitrary index file URLs and supplied credentials are refused; TLS, time, size, count and pinned-content checks apply. Private destinations and trusted proxy configuration remain intentionally allowed. Discovery performs GET before Apply. No IP allowlist or DNS-rebinding defense is claimed. No elevation beyond the authenticated local owner's source-selection authority was demonstrated; formal disposition remains required. |
| [46](https://github.com/777genius/universal-agent-plugins/security/code-scanning/46), High | `management_local_skills_apply.go:79`, Lstat | Restore ID -> exact 64 lowercase hex -> fixed path under captured StateRoot -> strict child/no-existing-symlink validation -> metadata check. An arbitrary HTTP path is not accepted. The check/open sequence assumes private state storage; it is not hostile same-user process isolation. |
| [47](https://github.com/777genius/universal-agent-plugins/security/code-scanning/47), High | `management_local_skills_apply.go:89`, ReadFile | Same identity and containment checks -> regular file <=65,536 bytes -> read -> strict single JSON document -> exact operation, backup, captured root and Skill identity. **2026-10-26 correction:** preserve the pre-open file-type refusal, open within captured StateRoot, validate and read the same descriptor, and bound the read to 65,537 bytes with an explicit size refusal. The historical separate check/read did not provide that descriptor confinement or growth bound. |
| [48](https://github.com/777genius/universal-agent-plugins/security/code-scanning/48), High | `pathpolicy/pathpolicy.go:111`, Lstat | Candidate -> absolute/clean paths -> strict `Rel` child check -> inspect base and each descendant -> reject existing symlinks and non-directory ancestors. The reported operation is within the validator. Parents above base are not checked here; Management canonicalizes startup roots. |
| [49](https://github.com/777genius/universal-agent-plugins/security/code-scanning/49), High | `packagedigest/snapshot.go:87`, Mkdir | Validated portable WalkDir inventory -> new private `MkdirTemp` parent -> literal snapshot root -> inventory-derived child directory. WalkDir does not recurse through symlink entries. Package-link safety additionally depends on corrected alert 50. |
| [50](https://github.com/777genius/universal-agent-plugins/security/code-scanning/50), High | `packagedigest/snapshot.go:92`, Symlink | Untrusted package link text -> link-text checks -> internal-tree resolution -> snapshot symlink creation. **Confirmed validation defect corrected:** the old resolver cleaned parent components before following directory links. The replacement processes components in filesystem order, bounds link expansions, and rejects root escape, missing paths and non-directory intermediates. |
| [51](https://github.com/777genius/universal-agent-plugins/security/code-scanning/51), High | `packagedigest/snapshot.go:152`, Lstat | Host/acquisition-provided package root -> absolute clean path -> inspect root -> require a real directory, not a final symlink. Management cannot supply a raw PackageRoot through HTTP. Explicit local CLI packages remain an intentional separate capability. |
| [52](https://github.com/777genius/universal-agent-plugins/security/code-scanning/52), High | `packagedigest/snapshot.go:356`, OpenFile | Validated inventory -> fresh private snapshot -> exclusive destination creation. Opened source must remain the same regular file and size; copy is bounded. Destination inventory has no children traversing a directory symlink. Link validity is separately corrected under 50. |
| [53](https://github.com/777genius/universal-agent-plugins/security/code-scanning/53), High | `sourceacquisition/acquirer.go:360`, Lstat | Source subpath -> normalized portable relative grammar -> exact Git revision/tree inspection -> private checkout -> selected-root metadata check rejecting final symlink/non-directory -> snapshot validation. No independent path escape was demonstrated. |
| [54](https://github.com/777genius/universal-agent-plugins/security/code-scanning/54), High | `source_direct.go:28`, Stat | Management source rejection used a shared CLI archive classifier that performed a local metadata probe. **Correction:** use offline `sourcespec.Parse`/`KindLocalArchive` for this unsupported Management source kind. The ordinary CLI classifier remains unchanged. No content acquisition through this Stat was demonstrated. |
| [55](https://github.com/777genius/universal-agent-plugins/security/code-scanning/55), High | `dirswap/dirswap.go:314`, ReadFile | Validated portable operation ID -> captured JournalDir + ID.json -> journal read -> strict fields/exact identity and recovery ownership checks. The old reader did not enforce leaf confinement before reading. **Correction:** `os.OpenInRoot`, regular-file validation and decoding bytes from the same descriptor. This prevents following a leaf outside JournalDir; trust in selection of the private journal root remains explicit. |
| [56](https://github.com/777genius/universal-agent-plugins/security/code-scanning/56), High | `packagedigest/snapshot.go:403`, Chmod | Previously created validated snapshot entries -> reverse seal walk -> skip symlink entries -> chmod owned paths. This is separate from the package-link defect in 50; no independent outside chmod was demonstrated. |
| [57](https://github.com/777genius/universal-agent-plugins/security/code-scanning/57), High | `installer/package.go:77`, Lstat | Host-provided package root and source-derived executable inventory -> root/temporary-root checks -> clean relative executable-path guards -> regular-file metadata -> whole-tree inspection. HTTP provides neither root nor executable inventory. Intermediate links can be followed by the metadata probe; whole-tree link safety therefore also needs the correction in 50. |
| [58](https://github.com/777genius/universal-agent-plugins/security/code-scanning/58), High | `installer/package.go:414`, Lstat | Acquired package identity -> same relative executable-inventory checks -> metadata -> reseal with exact source authority, digest and inventory. No separate path escape was demonstrated; the corrected package-tree validator remains necessary. |
| [59](https://github.com/777genius/universal-agent-plugins/security/code-scanning/59), Critical | `process/osrunner.go:637`, exec.Command | Git acquisition fixes argv[0] to `git`; normalized endpoints/refs/subpaths are separate arguments, with option boundaries and isolated checkout config. Native operations use the canonical executable captured from trusted startup; HTTP chooses only captured target IDs and cannot add command/argv/env fields. Native credential helpers and SSH configuration belong to the OS owner, not downloaded content. No request-controlled program, shell construction or option-injection path was demonstrated. Formal disposition is still required. |

The directory-swap read correction strengthens a specific missing confinement
boundary. It does not establish that a remote caller could already write the
private journal directory. Likewise, strict path checks do not establish
isolation from another hostile process with the same OS identity.

## Agent Teams

| Alert | Input -> guard -> operation -> correction |
| --- | --- |
| [580](https://github.com/777genius/agent-teams-ai/security/code-scanning/580), High | Tracked SDK provenance -> cross-repository checkout of an immutable source pin -> executed build/test code. A manual dispatch can carry default-branch cache authority despite read-only contents permission. Remove that dispatch entry point from this E2E workflow; retain PR qualification, set job cache-mode to none and assert the runner's effective mode before checkout. This is not a claim that setup-node cache settings revoke cache tokens. |
| [581](https://github.com/777genius/agent-teams-ai/security/code-scanning/581), Medium | Fixture selectors/labels/operation IDs/times -> string expressions -> CDP evaluation; the reported alert groups 14 flows. Replace the source-construction boundary with one checked-in fixed function and CDP CallArgument values. Actual user-input events and the complete operation/native assertions remain required in the new Electron run. |

GitHub documents scoped cache-token enforcement and `ACTIONS_CACHE_MODE` in its
[cache reference](https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching#controlling-cache-access-with-cache-mode).
The current upstream CodeQL cache query does not yet model that key, so the
correction also removes the privileged trigger rather than relying on an
unrecognized mitigation or suppressing the query.

## Verification and completion gates

- Alert 50: the real snapshot regression and six component-order cases failed
  before the correction. After it, the whole packagedigest suite passed on Go
  1.26.10. Valid internal links, their original text and existing digest goldens
  remain covered. Independent review found no P1/P2 in the replacement.
- Alert 54: the focused admission regression failed for directory-shaped local
  archives before the correction, then passed; valid explicit remote grammar
  remains covered without network access.
- Alert 55: both fixture-only external journal-link regressions failed before
  the correction. After it, the complete dirswap package, including existing
  recovery tests, passed in 1.916 seconds; ordinary valid journals remain
  readable. The open is root-confined, not a blanket no-symlink policy: internal
  links may still resolve within the captured root.
- App: hostile string data must remain data in the fixed dispatcher; stale
  completed operations must not satisfy a new Apply. Typecheck, helper tests,
  actual fresh Electron qualification and fresh CodeQL remain required.
- Existing Git source grammar and website policy tests were inspected and run.
  The broader Management website test reached its private TLS preview checks
  but failed locally at native Codex activation on a duplicate environment
  entry; this is not recorded as a complete pass. Current CI must establish the
  actual candidate result.
- Go source changes require the downstream native/SDK pin to be regenerated
  from the final committed source. Results on App 7667 / UAP 8645 are historical.
- Each new scan must be read after all language analyses finish. A successful
  analyzer workflow is not a successful findings aggregate. Findings that remain
  after a justified fix require specific source reasoning and formal GitHub
  disposition; no query suppression or protection bypass is authorized here.

## First published correction and platform follow-up

The first correction was published and immediately pushed as
`ae51ac682180438e7ef374f13e5f52ecbbd3c5ee`. Both language analyzers in
[CodeQL run 40206182045](https://github.com/777genius/universal-agent-plugins/actions/runs/40206182045)
completed successfully on merge `d98a0d6f903d9a46f66ac1668bb928d4086c9179`.
The findings aggregate `121207725081` still fails with **13 alerts: two Critical
and eleven High**, down from fifteen. This count does not identify the two
cleared alert IDs; unavailable per-alert API state is not inferred from it.

The native authoring matrix exposed a portability error in the new positive
test fixture. On Windows, Go's `os.Symlink` changes forward slashes in the native
link target to backslashes; the existing portable package policy correctly
rejects that target before the roundtrip assertion. Artifact `12603114757`,
SHA-256 `ba8e7f51cca15ef95287b37c77bb3f2e25761a25f269b2c2d3cc2810fff09978`,
records this exact Windows arm64 failure. The follow-up separates an unconditional
portable directory-link roundtrip from the multi-component native-link policy:
the latter must be rejected on Windows and must retain its original target and
content on POSIX. All component-order validator cases still run on every OS;
there is no skipped security assertion or relaxed production link policy.
Snapshot cleanup errors are also checked, correcting the new test's lint error.

The changed test candidate still requires fresh platform and scanner results.
This record does not assert that all GitHub alerts have closed or that an
unfinished candidate has passed CI.

## Retained local Skill receipt follow-up, 2026-10-26

After current-main reconciliation, the reader associated with alerts #46/#47
retains the exact operation-ID grammar and no-existing-symlink policy. Its
pre-open regular-file check remains so an ordinary named pipe is refused before
opening. `os.OpenInRoot` confines the subsequent open to captured StateRoot;
metadata validation and the bounded read use the same opened descriptor. Reading
at most 65,537 bytes and rejecting more than 65,536 prevents concurrent growth
from bypassing the existing size limit. Startup root selection still belongs to
the trusted process owner; this is not hostile same-user process isolation.

The complete `TestManagementLocalSkill` suite passes locally, including actual
create/edit/delete/restore, interrupted restore and retention. New public restore
cases accept a valid receipt exactly at the size limit and refuse an oversized
receipt, trailing JSON, a directory, in-root file/parent links and an escaping
file link. Refusals preserve the active Skill and its retained backup. These
checks qualify the reader behavior, not the unavailable CodeQL flow edges.
Fresh scanner results and individual formal dispositions remain required.
