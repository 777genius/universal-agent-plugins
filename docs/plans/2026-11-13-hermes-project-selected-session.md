# Hermes Project selected-session integration candidate

Base: `2c299c6ada80805399d411c3a2b9b1d7b06910bf`. This local candidate connects
the existing installer SDK, directory transaction kernel, native component
mutation, retained data receipt, and explicit session planner. It is held for
review and genuine native admission. The public registry still uses `New()` and
retains its Project MCP refusal; `NewProject()` is the unregistered test
composition. Neither a public capability flip nor native success is claimed.
Remove the temporary candidate constructor/selection when the complete route is
qualified, rather than publishing a dormant feature flag.

## Selected scope and effects

The flat native root is exactly `PROJECT/.hermes/uap-session`. Its parent
`PROJECT/.hermes` remains the Project binding profile authority. Original User
configuration and credentials are neither copied nor modified. Project Skills
continue to use `PROJECT/.hermes/skills` and managed packages/data keep their
existing `.uap` locations and receipts.

| Operation | Candidate behavior and permitted effects |
| --- | --- |
| Prepare / dry run | Existing immutable package planning and ownership reads; refuse a preexisting selected root without a committed standalone receipt, even an empty root or an identical marker. |
| Confirmed install | Existing package/data transaction and native-attempt marker; exclusive directory publication creates the selected root and `.uap-owner`; the existing component mutation installs Skills and exact owned `mcp_servers` entries in `config.yaml`. |
| Repeat / inspect | Read exact root publication and component receipts; repeat is unchanged, and no native executable runs. |
| Update / repair | Reuse the same owned root/data; the existing native component transaction checks exact entry ownership, compensates partial effects, and refuses foreign edits. |
| Remove | Remove only owned YAML entries, Skills and managed package. Retain shared root, marker, standalone receipt and persistent data. No runtime history/cache deletion is authorized. |
| Explicit run | Plan an existing managed Project binding, preserve inherited OS HOME, and override only child `HERMES_HOME`; bounded verification uses the pinned CLI's `config path` and `config get mcp_servers --json`; inspect selects `mcp list`, ordinary launch adds no invented flags. |

Install, doctor, repair and remove never start Hermes, authenticate, invoke a
model, or infer runtime activation. Successful installation remains
`Prepared`/`Installed`, without an attested activation or authentication result.

## Durable root publication

`ports.NativeProjectRootPublisher` is the narrow client/composition seam.
`transaction.ProjectRootPublisher` contains no Hermes policy or client branch;
the Hermes adapter supplies the exact path, marker object and bounded marker
body. Installer and CLI compose the publisher with their existing kernel and
state store. The marker is not independently sufficient ownership.

`DirectoryMutation.Standalone` is the create-only counterpart of the existing
standalone directory removal receipt. It requires `RequireAbsent`, an exact
unchanged desired state, and a verifier. It rejects lifecycle/native metadata,
replacement and grouping. The ordinary directory mutation behavior is unchanged.
The auxiliary receipt goes in existing `State.TransactionReceipts`; package
TargetLocator, binding receipts, NativeObjects and lifecycle are untouched.
No schema, second state store or generic transaction framework is introduced.

The publication digest binds the root and immediate parent's held physical
identities, the logical root, and marker digest. `LegacyHandleIdentity` uses the
existing POSIX device/inode or Windows volume/file-index encoding directly from
the held directory handle. It neither opts into another identity schema nor
reopens a replaced name. Reusing the root requires exactly one committed
standalone receipt plus the physical and marker verification. That authority
survives removal of the first installation and can serve another installation.

Fresh staging and marker operations use held `os.Root` authority, not a
path-based temporary-directory or parent-following marker read. Named/held joins
surround the first effect/read and final verification. Bounded marker reads use
the existing rooted file helper. Before kernel handoff, cleanup removes only the
exact newly created marker and known empty directories through held parents;
foreign children, replaced identities or uncertainty are retained. After kernel
entry, cleanup is allowed only if the operation journal is absent and reloaded
state exactly equals the predecessor. An open journal or unknown state retains
kernel ownership for recovery.

Independent review added a concrete moved-staging counterexample. Cleanup now
rejoins the entire named base/staged/selected chain before its first marker read,
before deleting that marker, and before each empty-directory removal. A moved
owned stage and an empty replacement both survive the refusal. The held handle
alone never authorizes cleanup after the recorded pathname changes.

## Session source and native admission boundary

The planner verifies the committed root receipt, exact Project binding objects
and one immutable, bounded `config.yaml` snapshot. The Hermes codec checks all
owned entry digests against that same body. Duplicate YAML keys, aliases, merges,
multiple documents, unrelated settings and unowned servers refuse launch.
JSON returned by the native inventory must have unique keys and match the full
source `mcp_servers` mapping. The native path must identify the exact selected
config, not a substring or a fallback User profile. Failed, truncated, canceled
or incompletely cleaned-up native commands do not establish evidence.

The durable physical root and parent receipt, followed by the immutable source,
are rejoined before and after each native observation and before success. Actual
SDK regressions replace either the root with an identical-byte clone or its
parent while preserving the root inode, at both native boundaries. Each refuses
at that boundary without issuing a subsequent command or changing stored state.

The source preparer currently accepts only `.uap-owner`, `config.yaml` and the
known config lock. **Native bootstrap membership is not yet admitted.** An
additional file or directory, including a native-created cache/history file,
refuses launch and is left intact. The original current admission run must
establish the finite startup/write set and repeat/restart behavior before the
consumer can accept those members. Old partial macOS receipts or vendor source
alone do not qualify that policy. Consequently the candidate's post-native
source check is intentionally not claimed to pass a real bootstrap yet.

POSIX source files require the current UID, private mode 0600 and one link; the
selected root requires current UID and mode 0700. Leaf opens are rooted,
nofollow and nonblocking, and FIFO replacement refuses before reading. Windows
checks ordinary non-reparse held files/directories and single-link files, with
the actual held handle's owner equal to the current token's exact `TokenOwner`.
An elevated token may have a group default owner different from `TokenUser`;
there is no user-or-group fallback and no ACL mutation. This owner check does
not assert that Windows inherited ACLs are private. Actual Windows execution,
including differing default-owner semantics, remains a qualification gate;
cross-compilation is not native evidence.

Token owner semantics: [Microsoft TOKEN_OWNER reference](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-token_owner).

## Verification and recovery limits

Local tests exercise the actual public installer SDK with only the candidate
adapter injected and every native process forbidden: Project A/B install,
repeat, expanded package/data paths, immutable data authority, fresh-engine
update, repair of an absent owned Skill, inspect, remove, retained root/data and
unchanged original User configuration/auth sentinel. Conflicting roots and
edited owned MCP entries refuse before effects. Session planning reads the real
persisted binding and standalone root receipt.

Kernel tests use real subprocess death before rename, after rename and after
the state decision. Recovery rolls back the uncommitted root, or finalizes the
standalone receipt without requiring the original installation to remain.
Publisher tests cover root and parent replacement, marker-only forgery,
substitution before a marker write/read, cleanup of unused staging after kernel
preflight refusal, and retention/recovery after an open-intent failure.

A separate actual SDK subprocess exits after the standalone root receipt is
committed and before the binding acknowledges the native effect. The root and
package authorities remain distinct; MCP has not been written and the native
attempt remains pending. The fresh facade reports the existing generic Project
uncertainty and refuses guessed recovery/repair, preserving the complete tree
and original User files. This is the same declared boundary as existing Codex
Project process-death handling. Automatic recovery of an unrecorded full native
attempt is not implemented or claimed by the auxiliary-root transaction.

Before publication: reconcile the moving PR head, obtain corrected original
three-platform admission artifacts, admit only their measured native member
set, execute Windows owner/handle tests, replace the held candidate composition
with the qualified public route, and run public CLI add/doctor/repair/run/remove
with genuine native list/test/remove/restart evidence. These gates must not be
reported as satisfied by inert SDK tests, cross-builds, or marker equality.

## Independently audited original macOS capture

[Run 43505052472](https://github.com/777genius/universal-agent-plugins/actions/runs/43505052472),
artifact `14077169877`, source `95f8648b4ffe03b729c16c3a8b1506f922f33f4e`:
the 39,000-byte original ZIP has SHA-256
`22634998a99e7595d9802e1678179c6b1608797b0b9a51edbf0aa4260afc79db`.
Independent inspection verified its 71 closed members/CRCs, all 70 report stream
digests, 21 command status/stream joins and the chronological 19-command inventory
preimage/postimage chain. The pinned vendor source and complete bundled runtime
inventory remain unchanged. A/B native path and full MCP JSON are exact; tests,
restart, removal, absent listing and expected removed-test exit 3 agree with
the endpoint methods. Every Project command preserves the original User and
sibling root; no unadmitted shared effect appears.

This macOS capture observes twelve directories, the fixed 667-byte `SOUL.md`,
one initial config backup, and initially empty agent/error logs. Only the agent
log grows during the MCP tests/removal, and each post-removal list adds a second
config backup. Native output reports `Hermes Agent vunknown (2026.9.24)`; source
pin attribution must not be relabeled as a measured semantic version. Cold
bootstrap happens on `--version`/the guard, before native config path/get, and
the fixture uses `.uap/hermes-runtime`. Consequently this capture does not yet
qualify arbitrary installed Hermes executables, cold config-path startup,
interactive session writes, the production `.hermes/uap-session` path, or the
other two operating systems. All product/admission flags remain false.

## Ordinary source launcher and bootstrap boundary

The original source [run 43424755622](https://github.com/777genius/universal-agent-plugins/actions/runs/43424755622)
at UAP `aabcd9b3e763d4341884173d37cdd5b1f7d7caf8` records the installed banner
`Hermes Agent v0.23.0 (2026.9.24) · stable · local 2bd378ed1b8d` on Linux
job `131883336442`, macOS job `131883336854` and Windows job `131883336901`.
This is an actual User-install version observation, not Project bootstrap
qualification. The bundled `vunknown` result remains a separate runtime route.

The old source collector booted its observer under a new disposable OS HOME and
selected-profile before its initial inventory. The source had been installed at
`RUNNER_TEMP/hermes-agent` using a different custom selected-profile home. At the
pinned vendor source, `pm.environments.owning_home_root` searches checkout.parent
and the current platform default home for actual committed state. That fixture
can lose recognition of the installation owner; its large observer bootstrap
does not establish ordinary borrowed-Project effects with the installed User
preserved. The next measurement must reuse the existing source fixture/collector
with a recognized real owner and unchanged official installation, preserving
the original OS HOME and installed native User root across A/B. Owner facts must
not be copied or fabricated, and native commands must use installed `HERMES_BIN`.

The immutable input boundary is the original User/source/runtime identity plus
the exact fresh selected root and parent identities and authored configuration.
Only HERMES_HOME changes between Project selections. Native output must join exit
status, bounded streams, exact config path and complete MCP mapping, endpoint
events and a complete bounded member delta; original User, source/runtime and
sibling effects remain separately checked. The source launcher prepares the
selected data root's own dependency generation and may ensure required tools.
Its PM state and bootstrap are client-owned, distinct from UAP's marker/config;
this candidate does not adopt that tree or implement another dependency installer.

Interruption retains the original input and failed-command postimage after
whole-process settlement. A later ordinary native launch must demonstrate its
own recovery under the same launcher/source/root authority; no baseline renewal
or deletion of native update markers is permitted. The standalone root journal
alone does not establish that recovery. Also, source `--version` skips dependency
preparation while the launcher still reaches dependency activation: success in
a fresh Project home cannot be inferred from the measured User banner.

The original 95f Linux bundled job `132163259930` subsequently completed with
the existing 120-second owned-build cleanup timeout, after official payload
verification and a complete metadata census. Native admission was skipped and
no Linux Project artifact was published. The corrected cleanup budget needs its
own genuine result; this failure does not qualify the Linux route.
