# Codex concrete TOML parser recovery — fresh pre-code plan

The reset erased unpublished finald8daad5a/source296acbc2 and their objects.
Published base0d1319f947c7f4403691b3a60b1193baa0074ee9 still contains concrete
tomledit/go-toml imports inside nativeconfig, which the unchanged contract
dependency-closure guard correctly rejects. Restore the previously reviewed
concrete backend extraction through two fresh independent plan GOs before code,
then fresh meaningful tests/lint and two fresh code GOs. Historical GOs/tests
are context, never evidence for reconstructed bytes.

## Exact first slice: architecture without losing behavior

Move retained TOML syntax parsing/editing into concrete adapters/codextoml,
outside the clients/domain/ports contract's nativeconfig closure. Preserve
tomledit/go-toml dependencies for that real consumer and useful legacy tests;
do not remove capability, widen guard snapshots/allowlists/platform sweeps or
change go.mod just to turn the guard green. Keep TOML-specific server projection
and pure desired-receipt/digest handling in the neutral kernel where appropriate.

Expose only a small nativeconfig CodexDocument interface and parser function:
copy-returned MCP entry map as json.RawMessage plus render-from-current-entry
map. The concrete backend retains original bytes/TOML AST/foreign semantic
settings; nativeconfig owns hujson neutral edits, whole-entry receipt/digest,
FileIO, locks, original snapshots, CAS/rollback and committed cleanup.

A per-Kernel WithCodexDocumentParser method returns a configured copy retaining
the exact injected FileIO/lock acquirer. No global registration, implicit default
parser or nativeconfig import of the concrete package. CodecCodex Inspect/Apply
refuse a missing parser before reads/locks/effects; ordinary codecs are unchanged.
Pure DesiredReceipt remains parser-free and effect-free. Reject literal/typed
nil documents before native methods, release acquired locks and perform no write.
Clone original input bytes before crossing the parser seam and clone raw entry/
no-op output bytes so injected parsers cannot mutate retained snapshots.

The concrete backend preserves existing malformed/duplicate TOML refusals,
inline/dotted/parent/child collection forms, exact original no-op bytes, foreign
settings/comments, complete owned nested-entry removal and all requested native
MCP values. Reparse rendered TOML and compare foreign semantics and desired MCP
entries before returning bytes. Neutral canonical JSON keeps UseNumber; conversion
back to TOML retains exact signed integer values, including above2^53, rather
than silently introducing float rounding. No serializer rewrite for other codecs.

Supply the concrete parser explicitly on configured copies at all Codex project
mutation, registry/prepared inspection, object verification and native-settings
observation boundaries. Do not replace injected kernels with New() or alter user
Codex CLI/profile/trust/auth/model behavior. Update affected direct fixtures to
configure the concrete backend explicitly; retain every old TOML lifecycle and
refusal assertion at the concrete consumer plus new seam/codec regressions.

## Fresh proof and incremental publication

First verify the unchanged exact TestContractLayerDependencyClosure on its actual
internal/archtest package, including its unchanged platform sweep. Run complete
nativeconfig, concrete codextoml and preserved Codex client packages, affected
ProjectCodex provider/public lifecycle contracts, and pinned changed-code plus
full-file size/architecture lint. Meaningful seam tests cover missing parser
before FileIO/locks, injected I/O/lock preservation, adversarial byte mutation,
literal/typed nil with cleanup, raw-entry/no-op copy isolation and large integers.
No vendor/native/model/MCP/trust process executes during local checks.

Freeze this first source slice for two fresh independent code reviews, commit
reports and send each clean checkpoint immediately to root for feature-branch
publication. Root may integrate this approved extraction to remove the inherited
architecture blocker and rerun the unchanged guard. This is not all-client or
current-native qualification.

## Retained separate recovery obligations

The lost final Codex correction also tightened exact client/scope/original-user/
artifact/managed-layout/direct-delivery/binding/removal authority, physical
non-alias staging before projection, and exact current stdio data/package reuse.
Its public SDK A/B/pin-repair/restart/data and composed TLS Git/ZIP journeys must
be reconstructed through a separate reviewed authority recovery slice after this
architecture slice. Do not silently drop these known obligations or claim that
earlier native passes qualify changed source. Preserve the existing genuine
macOS/Windows evidence at its original source/run until a current-source replay.
