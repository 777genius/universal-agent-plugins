# Windsurf protected config: bounded comparison without acceptance

## Current source and genuine observation

PR528 current head `8006c406f749b5143483e6f265d8addbc35aa88c`, tree
`d009bc23ecdb4169749bc6a67e86fee4bf924c2a`, has genuine runtime run
`40440079451`, native job `121979681744`. Native artifact `12706755449`
outer SHA256 `bffcb50bd023f93e95ddf9189d30d2f145a5e7d75c253068531d11f02098b0c9`
and frontend `12706740543` outer SHA256
`562d8470fa9d804f0747d1ca57cedcbe5dac27e48d24172507b036020abd827a`
were checked as bounded, canonical, unique ordinary-file ZIPs with valid CRCs,
exact native/frontend source identities and empty source-clean streams. Their
inner ZIPs contain 16/25 flat members. Earlier run `40397598111` at `fcea17cf`
is retained separately; neither capture qualifies a lifecycle.

Current public project installation succeeds. The exact native print command
then exits 1, error=false/deadline=false, after 188 ms. Stdout is the eight-byte
terminal sequence `1b5b3f323030346c`; stderr is `Error: Login canceled` plus LF.
The existing post-command protection check fails on the fixture-authored
original `Root/xdg-config/devin/config.json`, with digest error nil. Thus file
content differs from the authored digest. The changed bytes, changed digest,
JSON validity and authored-field comparison were not captured. Login refusal
does not establish the reason or safety of that independent config mutation.
All five qualification flags, authentication and real-model claims stay false.

Local baseline `d54c597fa2d6907cd074a461352c5a02119f71e2` has exactly the
current published tree. It preserves the original worker branch and mirrors
only four immutable published files: the external polyglot/terminal CI updates
and both external native Windows version-fixture changes. Preserve those bytes
and published ancestry; no redundant Windows fix is applied.

## Narrow diagnostic hook

After two independent plan GOs, add one closed hook immediately before the
existing `f.check` fatal mismatch, retaining the same fatal and unchanged
expected/protected maps. It runs only for client Windsurf, nonnil original
fixture, the exact authored original config path, the already recorded
native-attempt used flag, and all three native-attempt capture members. Check
map membership so a legitimate empty stream still counts as captured. Every
other client/path, pre-child mismatch or incomplete triple gets no new capture.

Record one finite object inside the existing runtime-attempt.json, with no new
archive member or output path. Include expected SHA256 and the actual SHA256
already returned by the refusing guard, a guard-error boolean, and a fixed
classification. Validate digest shape before serialization; never serialize an
arbitrary supplied digest or an OS/parser error string.

For a guard error, record only a fixed refusal classification. When its digest
was successfully obtained, a further ordinary owned single-link held read is
capped at 64 KiB through the existing projectNativeRuntimeRead. Hash these bytes
and require equality with that already observed digest before interpretation.
Absent, aliased, unsafe, oversized or changing inputs produce only fixed
read-refused or observed-digest-changed classifications. No rebaseline, retry,
adoption, repair, user-content write, native restart or new process occurs.

Use the existing projectNativeRuntimeJSON strict whole-JSON decoder. Record
only finite type labels, presence/count values and equality booleans for the
two authored fields: auto_update equals false, and permissions equals the exact
authored allow/deny/ask object. Include strict-JSON validity and top-level field
count for an object. Other values, unknown key names and all profile contents
remain unreported. Nonobject or invalid JSON gets only a fixed category; no
JSONC acceptance or permissive fallback is introduced. The diagnostic must
serialize below 2 KiB without paths, URLs, credentials or arbitrary input text.
Even matching authored fields does not prove native policy enforcement.

## Scope and meaningful inert checks

Scope is the narrow hook in
repotests/agentplugins_project_native_cli_attempt_test.go, one new Linux
Windsurf-specific helper/contract test file, and this plan/validation record.
Preserve other clients and shared source, all original HOME/XDG/PATH authority,
authored updater/deny policy, pin and receipt identities, finite native argv/cwd,
original 60-second command and 150/180-second total budgets, held-reader gates,
process supervision, cleanup, closed artifacts and qualification policy.
VS Code is excluded by the latest owner instruction.

Actual inert temporary-file contracts cover exact authored bytes, whitespace
changes, wrong-type/changed auto_update and permissions, unknown sensitive keys
and values, duplicate/invalid/nonobject JSON, oversized and symlink/hardlink
refusal, changed observed digest, original guard error, invalid digest inputs,
wrong client/path, nil fixture and missing capture members. Check that a full
native triple permits only the intended diagnostic, empty captured streams are
handled correctly, no arbitrary input text is serialized, protected digests and
file/target identity/bytes remain unchanged, and qualification remains false.
No installer, native client, model, listener or network executes locally.

Use the shared exclusive Go/lint slot only after grant for focused new and
retained shared CLI contracts, affected all-enabled changed-line lint and the
whole-file size/architecture gate. Run existing Python runtime-admission tests
and replay the retained actual closed native envelope, preserving its failure
and source identity. Obtain independent code review before root-owned
publication to existing PR528; no release or new PR. A fresh genuine capture
must supply the missing bounded projection before any separate behavioral or
authentication plan. Never weaken the still-failing protection guard.
