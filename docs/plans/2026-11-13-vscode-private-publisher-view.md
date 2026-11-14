# Conditional private publisher view for VS Code native research

Status: scoped source changes and inert tests are authorized. No local mounts, native execution, policy or foreign metadata changes; genuine hosted native admission is still required.
Base: `751da6a36567c7fe589701f2d1d0b23eafebbb99`.
Scope: the dedicated trusted Project research fixture; no production VS Code
adapter, User/local/hooks behavior, generic planner, or shared process runner.

## Evidence and gates

Actual `0d250a1251fd987ccd11ccf88677acb26fcb11d4` failed first `empty-a`:
SIGTRAP, publisher isolation proof present, endpoint absent, 586 diagnostic bytes,
and all five then-recognized signature flags false. Its cause remains UNKNOWN.
The subsequent `751` source adds terminal-failure handoff and two source-backed
SUID-helper literals to the existing boolean classifier. Its genuine native
result was pending at plan creation. During independent plan review, the root
review reports actual751 sandboxUnavailable=true, with the failed stage retained
and planned child/display cleanup proved. This combined boolean does not identify
a SUID subtype or prove AppArmor causation; implementation still requires root
review of the original final-source artifact and concrete interpretation.
Historical Ubuntu24 helper failure is context, not proof of the current cause.

Before ANY implementation: review the original final-source `751` artifact,
require an actual newly recognized native fatal signature relevant to publisher
sandbox startup, and obtain root approval of the concrete interpretation and
amendment. A boolean signature alone does not establish AppArmor causation.
Obtain TWO independent plan reviews before code. An unrelated/unknown failure,
unsupported mount recipe or refused stock-profile transition leaves this plan
conditional; do not implement a speculative workaround or supersede queued CI.

## Fixed private view

The ordinary frontend invokes the SAME hash-verified root controller through
fixed root-owned `/usr/bin/sudo` and `/usr/bin/unshare`, with fixed `--mount
--propagation private` and existing exact controller/input-root operands. No shell,
arbitrary command, caller-supplied mount path/options, forked unsupervised helper,
PID/user namespace or privileged native client. Preserve the original cgroup,
process containment, original work/final deadlines and cleanup reserve.

Before any mount, prove the controller mount namespace differs from the original
frontend namespace and relevant propagation is recursively private. Read the
actual mount table; do not infer privacy from successful unshare alone. Namespace
setup precedes native privilege drop. Root already has scanner bind-mount
authority; the native child still runs UID/GID1001, empty supplementary groups,
all capability sets zero, NNP1, in the existing isolated network namespace.

Hold the original root-owned `/usr/share` directory through a nofollow O_PATH
descriptor and retain inode/device/owner/mode and mount identity. Refuse symlinks,
substitution, a preexisting `code` leaf of ANY type, and non-admitted ancestor
identities. Never mask, adopt, overwrite or remove a foreign existing code leaf.

Create only task-owned root directories under the existing sealed Base: fixed
`publisher-view/lower`, `upper`, and `work` leaves. Upper/work must be fresh,
disjoint, on the same admitted filesystem, with work empty and the required
OverlayFS d_type/xattr support. Refuse unsupported/nested-overlay constraints,
shared layers or option defaults whose security effects cannot be established.
Use a descriptor-anchored read-only lower bind at the owned lower leaf, with
mount-local noatime if host access-time preservation is asserted. Verify its
identity and mount flags; never remount the original host mount.

Mount the fixed overlay at `/usr/share` ONLY in this private namespace, using
that held lower and owned upper/work. Require a new exact overlay mount ID and
root-owned upper-backed merged-root mode0755; chmod only the owned upper-backed
root, never the lower inode. Overlay metadata comes from upper, not lower.
Require canonical `/usr/share/code` absent in both lower and merged views, create
this fresh owned leaf, and read-only-bind the ALREADY sealed/hash-verified
publisher tree there. Hold and verify source/target identities, exact new bind
mount ID and read-only flags. Keep executable0555/resource0444 policy, no setuid
helper enablement and unchanged genuine publisher bytes/pins.

## Admission and preservation

Use `/usr/share/code/code` and genuine canonical appRoot
`/usr/share/code/resources/app` consistently in requests, observer expectations,
publisher checks and receipts. The original sealed publisher remains the source
identity. Record actual root/native mount namespaces, exact admitted mount IDs,
stock profile file identity/hash and relevant unchanged sysctl values. Require
actual POST-EXEC native AppArmor label from the verified supervised publisher PID
(not a pre-exec verifier self-report), and the expected
stock VS Code profile. Freeze the exact installed profile path, hash, attachment
rule and expected label from actual image evidence before code; never select a
convenient alternative profile at runtime. Canonical path alone is not attachment proof.

Keep standard Electron sandbox arguments and the existing setpriv/NNP order.
NNP may refuse an LSM transition: actual current-kernel label plus normal startup
and unchanged privilege proofs are mandatory. No aa-exec, NNP delay, extra native
capabilities, policy load/reload, profile edits, host chmod, sysctl change,
--no-sandbox, alternative runtime, host spoofing, or fallback on refusal.
This qualifies an owned PRIVATE VIEW using stock policy, not an unchanged stock
host launch. Preserve native workspace/MCP trust UI and all existing refusals.

The overlay is NOT an immutable snapshot of all `/usr/share`. Other host writers
can change lower contents. Only the canonical publisher comes from sealed inputs;
no foreign lower file becomes trusted merely by being visible. Compare the held
original host parent inode/owner/mode/ctime/mtime and mount identity independently
of the covered pathname, and verify original code-leaf absence after release.
Read-only/noatime lower access can avoid task-induced atime changes, but ordinary
reads of stock policy may affect atime. Do not claim all metadata byte-for-byte
unchanged: the prerequisite does not measure/qualify atime; preserve bytes and stable identity/ownership/
mode, and refuse unexplained stable-field changes. Never restore foreign metadata
to disguise external writes. Host policy and sysctl have NO write authority.

## Cleanup and failure

Retain the original native failure separately from cleanup errors. Every owned
native/driver/display/scanner tree must be PROVEN reaped by existing supervision
before releasing its mounts. Never set TreeReaped manually or accept a failed
runner as proof. Release exact mounts in reverse acquisition order: canonical
publisher bind, overlay, lower bind (with scanner ordering retained as applicable).
Before each unmount, verify held target/source identity and exact mount ID; after
release verify the underlying expected view. No lazy/forced/global unmount.
EBUSY, changed identity, unproved process cleanup or incomplete release withholds
Base recursive deletion, joins the failure, and keeps ALL qualification false.

A namespace does NOT disappear merely because the controller exits: another
member process, namespace descriptor or bind reference can pin it. Do not export
namespace descriptors or create persistent namespace binds. Admit final lifetime
and outer-process cleanup through existing containment evidence; namespace
teardown on final reference release is additional protection, not deletion or
success authority. Crash paths must leave host parent/policy/sysctl untouched;
owned residual upper/work directories remain unqualified evidence, not proof of
successful cleanup.

## Files, review and validation

Expected dedicated files (approximately 500–800 changed lines including tests;
roughly 2–4 hours implementation/contracts, no hosted queue estimate):

- `scripts/research-vscode-project.py` and its inert contracts: fixed launch and
  closed mount/profile proof admission; no general namespace wrapper.
- New `research/vscodeworkspace/trusted_publisher_view_linux.go` and focused
  tests: fixed setup, held identities, exact release, failure ownership only.
- Existing `bootstrap_linux.go`, `model_linux.go`, `workbench_linux.go`: view
  lifetime, preservation, publisher identity and cleanup ordering.
- `capture_linux.go` and the exact duplicated verifier in driver.ts: closed
  actual namespace/label proof, keeping existing UID/caps/NNP/net proof.
- `trusted_observe_linux.go`, `trusted_observation_validation_linux.go` and
  runner.ts only where exact canonical publisher/appRoot expectations require it.
- Explicit frontend/artifact schemas and dedicated tests, plus this plan.

No edits to shared process, generic ownership/config kernel or production client.
If a larger rewrite or new dependency is needed, stop for a concrete amendment.
Review pass1: privilege/path/foreign metadata and mount escape boundaries.
Independent pass2: lifecycle/crash/EBUSY proof and qualification honesty.

Inert tests: wrong/shared namespace, nonprivate propagation, existing foreign code
leaf, aliased/replaced lower/upper/work, mismatched filesystems, unsupported layer
features, changed mount IDs, wrong overlay mode, writable publisher bind, forged
label/UID/caps/NNP/net, malformed/oversized proof, EBUSY and cleanup failure. Verify
failure preservation and no Base deletion or promotion on every incomplete path.
Run focused Go/Python/Node, pinned full TypeScript compile, make lint/test-core.
Do not run vendor clients, mounts or privileged proof locally.

Final-source genuine hosted admission must prove host before/after facts from
outside the private view, real profile attachment, UID/caps/NNP/network/cgroup
continuity, all existing trusted A/B/restart/repair/remove native cases and raw
nonce/source joins, and complete exact mount/display/process cleanup. Review
original bounded artifacts independently before any acceptance/promotion. Queued,
cancelled, inert-only or evidence-only results never qualify native support.

## Primary references

- [Kernel OverlayFS](https://kernel.org/doc/html/latest/filesystems/overlayfs.html):
  upper/work constraints, upper directory metadata, copy-up and shared-layer risks.
- [mount_namespaces(7)](https://man7.org/linux/man-pages/man7/mount_namespaces.7.html)
  and [unshare(1)](https://man7.org/linux/man-pages/man1/unshare.1.html): private
  propagation and mount-table isolation.
- [namespaces(7)](https://man7.org/linux/man-pages/man7/namespaces.7.html): references
  that pin namespaces beyond process exit.
- [Kernel NNP](https://docs.kernel.org/userspace-api/no_new_privs.html): inherited
  no-new-privileges and LSM transition limits.
- [Ubuntu24 release notes](https://documentation.ubuntu.com/release-notes/24.04/)
  and [AppArmor attachment syntax](https://www.apparmor.net/man/4.0/apparmor.d/):
  stock per-application userns permissions and executable-path attachment; actual
  installed policy and kernel behavior still require native evidence.

## Authorized prerequisite: bounded read-only host-policy measurement

This separate prerequisite is authorized for implementation only after root and
independent review of its concrete design. The private mount-view implementation
above remains GATED. Root independently verified actual751 original archives and
39 source/artifact joins: sandboxUnavailable=true identifies only a sandbox-startup
message class; failed stage retained, both trees planned/reaped, cleanup complete.
Its image was20261111.370.1; actual0d used20261101.360.1. Neither old artifact contains
installed/loaded policy facts. No AppArmor or SUID-subtype causation is inferred.

Use the sealed fixed Go controller, trusted mode only. Start held measurements
before execute; finish after execute and scanner cleanup, before Base deletion.
Never skip the existing native failure capture because measurement failed; retain
and join the original failure plus a closed measurement refusal afterward.
One flat c.keep artifact `trusted-host-policy.json`, maximum16KiB, contains only
fixed-field facts/hashes/numeric identities and static refusal enums. Extend the
frontend's exact member list and strict schema; no raw profiles, arbitrary logs,
profile enumeration output, generic sudo script or host framework.

Fixed inputs and limits:

- `/etc/apparmor.d/code`: at most16KiB, root-owned ordinary single-link file,
  descriptor-held nofollow/nonblocking capture with full stable path/fd checks.
  Record actual digest/identity without a required hash before first measurement.
  Project only the known Ubuntu grammar: abi4.0, tunables/global, profile vscode
  with attachment `/usr/share/code{/bin,}/code`, unconfined flag, userns permission,
  optional local/code include. Unknown syntax gives a closed refusal, never raw
  content. Official package hash is research context, not installed-state proof.
- `/etc/apparmor.d/local/code`: fixed optional path, maximum16KiB. Record only
  absence or bounded digest/identity. Presence refuses FUTURE mount admission;
  do not interpret or emit its potentially private arbitrary contents.
- `/sys/kernel/security/apparmor/profiles`: maximum256KiB,4096bytes per line.
  Expose only exact vscode membership/mode; no other names or raw list. Require
  the target projection stable across capture. Membership does NOT establish
  that loaded policy bytes match the installed file hash or resolve includes.
- Fixed sysctls: kernel/apparmor_restrict_unprivileged_userns,
  kernel/apparmor_restrict_unprivileged_unconfined, kernel/unprivileged_userns_clone
  as0/1; user/max_user_namespaces as bounded decimal<=2147483647. Each read<=32bytes.
- Held root, /usr and /usr/share ordinary-directory identities and canonical
  code-leaf absence. Observe0777 without changing it. Compare inode/device,
  owner/mode/mtime/ctime before/after; access time is excluded, not measured or
  qualified, and never restored.

Securityfs/proc files may report size0: use a dedicated bounded fixed-path reader
with held identity/type checks, rather than weakening ordinary-file size equality.
Missing required inputs, unsafe aliases/types/owners, overflow, unknown grammar,
changed file identity or unstable target membership produce static measurement
refusal. This is observation only, never runtime/native qualification or proof
of the actual publisher label; that post-exec proof remains a future gate.

Expected edits: new trusted_host_policy_linux.go and focused tests, trusted-only
bootstrap_linux.go hook, scripts/research-vscode-project.py exact artifact/schema
contracts, and this plan. Approximately250 implementation/220 inert-test lines,
subject to review; no shared runner or production client changes. Tests cover
known/unknown grammar, duplicate/extra fields, bounds, nofollow/nonblocking wrong
objects and replacement, unstable identity/membership, optional local include,
virtual-file size0, closed failure artifact, and preservation of native failure.

Implementation authorization update: root and cli_gap_audit approved ONLY the
read-only prerequisite. Add fixed optional abi/4.0, tunables/global and
tunables/alias digest/identity facts; nofollow disable/code and force-complain/code
entry presence; AppArmor enabled fact. No recursive include collector or dpkg
execution. The official known-file SHA is an explicit comparison/reference only,
not a required actual-host hash. Native stage execution remains before any final
measurement refusal is joined. The private mount implementation is still gated.


## Actual 79af prerequisite receipt and remaining gate

The original trusted run43380446623/job131736726329 checked out
`79af2028af50da4ac56f7bec03c376386544f8d9`, tree
`cf4c131b3bd8b8279c42904ff79276d1032ce8f6`, on Ubuntu24.04.5 image
`20261101.360.1`. Its inert contracts passed; first native empty-a failed
publisher-startup/publisher-startup-failed with SIGTRAP, isolation proof present,
endpoint absent,586 diagnostic bytes and sandboxUnavailable=true (other four
signature flags false). This identifies a sandbox-startup message class only;
neither a SUID subtype nor AppArmor causation is established.

Original native artifact14017523300 was independently checked: outer13509bytes
SHA256 `b2841893624a81c6d604b6a8c925633e96ec4b1d7871e344bc73131f5a08f571`;
inner48209bytes SHA256
`978333812057aceee07c0bf816d8217da0076ae4a6aa0ab0b8ea123894a13b9a`;
both ZIP CRCs pass. The original paired frontend artifact and native artifact
previously passed41 stream byte/hash joins. Controller retains the stage failure;
empty-a and Xvfb both planned_stop/tree_reaped=true, controller cleanup complete,
frontend cleanup complete, and controller_reaped=true. Collection and all native
qualification remain false. No completed native stage is admitted.

The host-policy receipt is observed/reason none, with identical before/after:
known-ubuntu-vscode-userns grammar, known_profile_match=true and installed code
SHA256 `9fd4a3cc5f3bf64aefbed6ff4cab30fd60563d655ae2896197f152d07b726e9a`.
Optional local/code, disable/code and force-complain/code are absent; AppArmor
enabled=true, exact loaded vscode membership mode=unconfined, code leaf absent.
Sysctls: apparmor_userns=1, apparmor_unconfined=0, userns_clone=1,
max_user_namespaces=58121. Fixed include hashes:
abi/4.0 `e510bb8f6788b45e48de2f859a6f94a7b8416cbac5a1051814cfce925fa911bd`;
tunables/global `2c1d72013b53bf103fb8abe2199ce5775f8493df1acd3ef497046156c09fdf7c`;
tunables/alias `e7b8b278db3f7aca74e0176f22cbd3adc3c89212cc9e66a90a2787b3ceee8030`.
All captured files root-owned ordinary mode0644, single-link. Held root/usr/share
directory facts respectively: device66305; inode2/1701/23364; mode0755/0755/0777;
UID/GID0; mountID28. Stable captured metadata is identical before/after.
This receipt explicitly sets native_qualified=false and
loaded_equals_disk_qualified=false. Disk hashes and loaded membership do not
prove loaded bytes equal disk, resolve every include, or prove post-exec native
attachment.

The current2dd source changes only the deterministic inert-test timestamp fixture
relative to79af; this does not turn79af into final-source2dd native acceptance.
Root must independently review source/runtime equivalence and interpret these
facts before authorizing ANY private-view code. No such implementation is
authorized by this amendment.

Additional mandatory mount-topology gate before any future setup: read a bounded
actual mountinfo projection for the held original /usr/share subtree and REFUSE
any existing descendant mount beneath it, an ambiguous/unreadable topology,
overflow, or topology drift. A plain lower bind plus overlay does not establish
preservation of nested mounts. Do not recursively clone/adopt foreign descendant
mounts or introduce a general mount framework. Root mountID28 alone cannot prove
this gate: the current receipt does not measure descendant topology. Verify the
same absence and held original mount identity again before release, retaining
original failure and withholding Base deletion on uncertainty.

## Explicit implementation authorization

After independent original79af artifact review, runtime-equivalence review to2dd,
and matching all four fixed policy/include hashes against the official Ubuntu
package without installing or executing it, root explicitly authorized this
dedicated private-view implementation. This authorizes source and inert tests
only; no local mounts, native execution, policy or foreign metadata changes.
The no-descendant-mount/topology-drift gate above is mandatory. Actual post-exec
label, unchanged isolation and final-source native lifecycle/cleanup remain
unproved until genuine hosted admission; all refusal paths stay unqualified.

Concrete size amendment: root approved up to1050 changed lines including plan and
inert tests for held layers, partial mount release, and actual post-exec proof.
This replaces the earlier500–800 estimate; no new framework, dependencies, shared
runner changes or omitted guards are authorized. Inert tests cannot qualify a
mount, label transition or real native lifecycle; genuine final-source CI remains
mandatory.

Actual current2dd trusted run43383113491/job131750009531 subsequently failed the
same first publisher-startup stage: SIGTRAP, proof present, endpoint absent,
585bytes, sandboxUnavailable=true only. Actual image20261101.360.1; policy remains
observed/none and before==after, with the same fixed policy/include hashes,
loaded membership and sysctls. Empty-a and Xvfb planned/reaped and cleanup complete.
This confirms final-source failure evidence, not native acceptance or causation.

Second concrete size amendment: root approved up to1200 changed lines for fixed
actual-sudo-parent namespace authority and live-driver cgroup joins. The current
controller mount namespace must differ from the actual held immediate sudo
parent namespace, which must equal the frontend's original namespace value.
Fixed no-fork unshare and sealed unix.Exec preserve this direct ancestry; refuse
any unknown parent/executable/start identity without scanning or fallback.
Recheck before each mount/remount. Publisher cgroup must match the actual live
supervised driver, whose dedicated cgroup differs from the outer controller's;
join driver PID start identity around the bounded membership read. No shared
process runner change or local native/mount execution is authorized.

Final reviewed size amendment: root approved up to1300 physical changed lines
for the fourteen dedicated private-view files, including the plan and inert
tests. The frontend additionally admits the closed view summary, requires full
three-mount release on success, and joins original share identity/mount ID to
the independently captured host policy before artifact publication. The source
and inert review do not qualify actual mounts or the native lifecycle.
