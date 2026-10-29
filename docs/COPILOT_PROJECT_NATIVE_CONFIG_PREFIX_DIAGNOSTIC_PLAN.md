# Copilot original config: bounded leading-comment diagnostic

## Current source and actual failure

Published PR527 source `f7421c5f1d6252dee7991b6bc34765663e277936`, tree
`65345a3d38219264db0f798ef3e3c491a216148a`, equals clean local
`c450dc9d5c86ecbfddb6d1ec563c071d9f6d507a`. Preserve its measured native
environment expansion, exact provider/probe identity checks, all external fixes
and original protected-config guard.

Genuine runtime40456499106/job122033009727 failed at that guard after the
provider-returned flag became true. Native12712639477 outer SHA256
`9ac80b05aea67079697538fed26bf5ac1986294c747bef647b35a99bb11dceb2`, inner
`29a79bdd280785c67f38d02952d8e1c8a88c35decfe09de3517e2dd007a8caad`, has21
ordinary canonical unique members. Frontend12712809326 outer SHA256
`bc5140f5d6e0aaf57c57e2925b9890baead353261157e149eba4794a928ded36`, inner
`b7425486ceea7e3fc5c9acf70388cd1555d01ea68a3b7dcb12dfa16e2720ae10`, has25.
API sizes/digests, archive closure/CRCs, all24 frontend command-stream hashes,
exact committed source/tree and empty patch list were independently checked.
The current native envelope re-admits as failed with all five qualification,
vendor-authentication and real-model flags false.

Its finite diagnostic records baseline277 bytes/valid JSON and observed423
bytes/invalid JSON, strict failure on the observed side, reason `syntax`.
Expected digest is
`a8a85b0ae210181acfedbf8d99319c00c4dde6792c3f9d7d04d7a9bb57983f62`;
observed digest is
`6ebd1de8072bf24f5d3ba857cf680caf17a19bd6eb57bb1b7591de9226a45eb0`.
No observed config bytes, prefix or semantic equivalence have been captured.

## Hypothesis and evidence boundary

The current primary [configuration-directory reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference)
distinguishes automatically managed `config.json` from user-editable JSONC
`settings.json`, and identifies trusted folders as internal config state.
It does not prove the bytes written by pinned1.0.103. An older upstream
[issue3403](https://github.com/github/copilot-cli/issues/3403) reports a comment
header at1.0.49; the bot says that issue was resolved at1.0.94. This is only a
reason to investigate a possible leading-comment format. It cannot establish
the current prefix, equivalence, or permission to accept changed config.

This slice is diagnostic only. It neither grants general JSONC support nor
changes protected authority. In particular, parsing a suffix must never make
the native attempt pass or replace the original byte/digest baseline.

## Finite implementation after two independent plan GOs

Scope: the existing Copilot config diagnostic Go file, its existing contract
file, and this plan. No production adapter, shared parser, native environment,
argv, trust setup, provider, workflow, acquisition, source pin or other client.

Only inside the current `invalid-json` branch, after the existing original
baseline digest check and one held ordinary bounded read whose digest equals
the already observed changed digest, add an optional finite prefix observation.
The baseline must already have parsed as a strict JSON object and the observed
error must be classified as `syntax`; other existing branches remain unchanged.
Do not perform a new filesystem read or extend the existing16KiB diagnostic
limit or original clock.

Privately scan only a contiguous leading sequence of LF-terminated `//` lines
and JSON whitespace. Require at least one comment line, at most16 comment
lines, and at most4096 total prefix bytes; require UTF-8 and no non-JSON control
characters. Stop before the first non-prefix byte, which must be `{` for a
candidate object. Do not strip inline/block comments, trailing commas, BOMs,
embedded control data or comments elsewhere, and do not execute anything.
Use slices of the held body; no unbounded line collection or normalization.

The new optional diagnostic object has a fixed schema and finite status:
`absent`, `prefix-refused`, `suffix-refused`, or `captured`. It may report only
bounded prefix byte count, line count, SHA256 of the complete exact prefix,
suffix byte count, the existing finite strict JSON failure category, and
optional exact semantic-object/trustedFolders equality booleans. No raw prefix,
comment text, JSON keys/values, parser error/context, arbitrary paths, provider
credentials or alternate configuration are emitted. Missing/failed comparison
remains unreported, not guessed false or assumed equal. Keep the complete
serialized diagnostic below its existing bound.

For a candidate suffix, use the unchanged strict parser, including duplicate
keys, finite numbers, structure and trailing-data refusal. Compare the entire
parsed suffix object to the original strict baseline, and compare the exact
trustedFolders value separately. Do not coerce/sort/relax values. This private
projection does not change the outer `invalid-json` status, original syntax
facts, protected digest result, fatal guard, frozen config bytes or flags.

## Meaningful inert controls and publication gate

Exercise the actual diagnostic through its existing held-read seam. Cover
bounded complete leading-comment syntax with independently stated bytes/digest,
equal objects, changed trust, extra semantic fields, whitespace, private canaries
in comments/keys/values, malformed/duplicate/deep/nonfinite suffix JSON, inline
and block comments, BOM, missing LF, prefix byte/line limits and unexpected
trailing data. An injected canary must never appear in serialized evidence.
All existing no-read-on-guard-error, hash stability, original-baseline retention,
plain-JSON format diagnostics and hostile source/config controls remain active.
Prove that a captured equal suffix still returns the changed digest and retains
`invalid-json`; no semantic equality can satisfy the original fatal guard.

Run focused affected inert Go contracts, all-enabled changed-line lint,
unfiltered whole-file size/complexity/architecture lint, format/diff checks and
the actual current failed Copilot-envelope replay. Use the exclusive validation
slot; no native client or positive hosted-eligibility guard runs locally.
Obtain two independent code reviews and fix findings before root-only publication
to the existing PR527 through a fresh real ref and expected-head lease.

The fresh genuine capture must establish whether this hypothesis matches the
pinned client's actual output. Any later fixture-format or authority amendment
requires its own concrete source-bound plan/reviews. The complete user/A/B
lifecycle, restart/removal/repeated removal and actual native tool-session proof
remain required. No release/new PR or deferred VS Code work is included.

## Actual implementation validation

Shell and desktop independently approved the final plan SHA256
`0db0c55dcd6502e0ce516189f8de34fc7882482f63515f9b2c2f11f5ab36cfd8`
before implementation, then reviewed the complete two-file code against the
separate plan commit `881d3dbab157e031741cbb165ac4fa20c8f24c11`.
Both gave final code GOs with no findings. Diagnostic source SHA256 is
`9ef2839d772a3bba46361e5d465729951948c95193ec05b22e163d9e11144f41`;
contract SHA256 is
`71fa655aa3a9aa89f1c3d0e4c27f83ab198685d03f27e24a06c22ef8c2ccc02d`.

Focused Copilot and retained strict-runtime JSON contracts passed in `0.047s`.
The actual inert child exercised the original fatal guard for both whitespace
and equal-object leading-comment changes. All-enabled affected lint and
unfiltered whole-file size/complexity/architecture lint each reported `0 issues`.
Formatting and whitespace checks pass. The exact prior f742 native envelope
re-admitted with21 members and unchanged inner SHA256
`29a79bdd280785c67f38d02952d8e1c8a88c35decfe09de3517e2dd007a8caad`:
failed/incomplete, all five qualifications false, authentication/model false,
and the original syntax diagnostic without hypothetical prefix evidence.
No vendor process or positive hosted eligibility guard ran locally. A fresh
genuine source-bound capture remains necessary before interpreting the prefix.
