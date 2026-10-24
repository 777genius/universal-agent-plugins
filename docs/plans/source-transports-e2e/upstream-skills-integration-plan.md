# Forward integration of accepted standalone Skills and source transports

2026-10-24. Root observed that source PR473 has no runnable merge CI because
main advanced to008370dd6767fecd19ebf25902f099b102226ba5. Its accepted changes
include standalone versioned GitHub Agent Skills (e3d84d05/#478), desktop and
terminal polish (#488), and landing changes. Preserve them all. This is a
forward merge into the source feature, never a main/release mutation or a
rewrite of accepted upstream PRs. Obtain two independent fresh plan GOs BEFORE
semantic conflict resolutions, and two fresh implementation reviews plus
meaningful current-byte checks after the integration.

## Frozen inputs and actual conflict inventory

The reconstructed source implementation is b171c142647297d532764fd120508a07cec79979;
its actual fresh test/lint evidence is preserved by88220d575b5987760be03ee818edec14ada8616e.
Those leaf results do not certify the upcoming merge. Read-only merge-tree
against exact upstream008370dd produced conflict-bearing treeb36dc1c5 and16
conflicted files: deps_resolver, deps_staging, deps_sync, manifestedit/edit,
dependencymanifest/decode, sourceacquisition/acquirer, statev2/store,
domain/state+types, environment/graph+model, installer/inspect+package,
usecase/binding+group_commit+installation. Automatic merges also touch identity,
environment session and README; review their interactions rather than assuming
textual success proves the contract. New upstream Skills adapter/discovery,
normalizer, picker UI/bridge/API/schema, qualification, release, scanner and
landing files stay accepted upstream bytes unless a necessary interaction is
identified, planned and reviewed. No blanket ours/theirs conflict choice.

## Combined identity, grammar and lock contract

Retain both field families: transport SourceKind/SourceURL/ArchiveSHA256, URL Git
and archive sources, plus upstream StableSourceKey/SkillSourceFacts and locked
Skill kind/name/requested path/resolved path/raw digest/normalizer. Clone Skill
facts where upstream clones revision metadata; do not lose facts in installation,
grouped commit, binding or persisted state. Retain transport projection helpers
and extend them to carry the upstream fields once rather than duplicating source
construction. Existing GitHub source keys and historical locks remain unchanged.

GitHub plugins, direct URL Git plugins, archives, Directory packages and normalized
GitHub Skills remain distinct source-key namespaces. A GitHub Skill retains the
accepted github-skill key including expected skill name and requested path;
requested identity is independent from relocated resolved path. URL Git and
archive keys remain endpoint+package tuple identities, never GitHub coercions.
StableSourceKey is used only for the upstream normalized Skill authority case;
it must not become a caller-controlled override of arbitrary transport identity.

Manifest decoding/editing retains kind+skill AND archive+sha256 syntax and strict
unknown/duplicate fields, exact-one-source and selector validation. Existing
Skills are GitHub-only: a direct URL Git/archive/registry source may not acquire
kind=skill semantics through the combined grammar. Preserve both strict public
Requirement validation and fresh unowned transitive empty-alias behavior; the
private key scalar remains ephemeral and never changes explicit candidate or
an existing pin. Locked selector validation checks all relevant source-family,
URL/repository, kind/name, path, selector, commit/archive and normalization facts.
Requirement reconstruction preserves both URL and Skill fields exactly.

## Resolution, materialization, reuse and cold repair

Share immutable selector resolution without discarding the feature's native
transport branch. Exact GitHub Skill requests route to upstream resolveSkill
before ordinary plugin loading, preserving relocation lookup and raw acquisition,
deterministic normalization and source facts. Ordinary GitHub/URL Git requests
retain actual source transport and verified pin semantics; archive behavior stays
unchanged. Do not normalize a plugin or send a raw Skill through plugin loading.

The snapshot cache key retains full canonical source key+immutable revision+
algorithm+artifact digest and, for Skills, expected name/requested path/raw digest/
normalizer. Distinct raw inputs or normalization contracts cannot share a cached
snapshot merely because normalized artifact bytes happen to match. Pinned Skills
use acquireLockedSkill (raw digest verification then normalized tree verification),
while ordinary source pins use existing acquirePinnedSource. Source projection
from a lock uses skillSnapshotSource only for exact Skill locks and the existing
identityFromLocked path for ordinary GitHub/URL/archive/Directory locks; retain
upstream malformed-lock errors before effects.

Upstream moved build/applyDependencyRepairs from deps_sync to deps_repair. Keep
one implementation in that upstream location and combine per-target recorded
revision reconstruction: transport URL/archive pin metadata AND upstream Skill
facts/raw digest/normalizer. Missing per-client Skill provenance still refuses
cold repair. Directory publication identity continues to take its existing path.
Repair never resolves a moving ref or substitutes latest bytes for an old pin.
Preserve scoped transaction, scanner/authorization, state ownership and alias
checks; no fallback through legacy engines or trust/auth shortcuts.

## Git extraction policy and retained guards

Upstream now distinguishes plugin acquisition's allowed unrelated root gitlinks
from strict raw-Skill tree completeness through allowRootGitlinks and its public
AcquireGitHubSkill APIs. Keep that policy separate from the source feature's
native transport boolean. Use a named private common extraction path or separate
explicit flags so a native URL transport never accidentally grants raw Skill
gitlink leniency. Raw Skills refuse source submodule/incomplete trees; plugin
policy remains the upstream policy. Retain native Git environment isolation,
immutable revision checking, sparse selection, file/path/tree validation,
executability and digest guards, controlled own-path diagnostics, cancellation
priority and runner secret redaction. No native client executes in these checks.

## Meaningful current-byte validation and review

Before the merge commit, inspect every resolved conflict and the automatic
identity/environment-session interactions. Verify accepted UI/landing/Skills
consumer files remain unchanged where no interaction requires edits. Preserve
all old tests; change no assertions merely to remove an integration failure.

Run full environment, sourcespec, manifest decoder/editor, sourceacquisition,
skillsource, state/domain/usecase/installer affected suites and public CLI saved
source plus standalone-Skill flows. Existing upstream Skills tests must continue
to exercise deterministic normalization, raw tree and normalized artifact digest,
relocation and per-target cold repair. Existing/new source tests retain real TLS
Git/ZIP, saved add/sync/update/pin repair/replay, same endpoint different package
tuples, explicit candidate/pin precedence, controlled redaction and actual CRC.

Add narrow combined regressions for grammar/source-family refusal and round-trip
of both field families; exact source namespace distinction; cache separation on
Skill normalization facts; Skill + direct transport source projection and repair
of recorded per-target old revisions without moving-ref substitution. Use actual
composition and inert/loopback fixtures rather than tautological helper copies.
Run full relevant pinned changed-code and unchanged full-file size/architecture
lint; no architecture snapshot, platform list, baseline or security guard edits.
Run existing upstream picker qualification checks as required by its workflow;
if infrastructure blocks a gate, record the actual blocker rather than a pass.

Freeze small clean checkpoints with exact input SHAs and current evidence for
root publication. Record independent review findings and fixes, then obtain two
fresh final code GOs. Root alone publishes the forward-merge feature and uses
normal GitHub CI; no source leaf pass, archived evidence or hosted client pass
is relabeled as proof for changed integration bytes or all-client completion.
