# Devin native configuration comments — pre-code review correction

2026-10-23. This is a narrow supplemental amendment to the twice-reviewed client
plan and frozen adapter132b8c38. Independent R1 found that strict JSON currently
rejects normal commented native Devin configuration. Automatic approval review
rejected the initial corrective source command because this shared parsing
amendment had not received two independent plan/fix GO reports. The command did
not execute; the source remains frozen and clean. Commit this plan and both
independent GO reports before implementing the correction.

## Evidence and defect

The genuine3000.11.205 distribution's pinned official
reference/configuration/config-file.mdx has an explicit JSON with Comments
section supporting line, inline and block comments. It also declares the
dedicated .devin/mcp_config.json and higher mcp_config.local.json native files;
the matching MCP documentation shows this native configuration. References:
https://docs.devin.ai/cli/reference/configuration/config-file and
https://docs.devin.ai/cli/extensibility/mcp/configuration. This is native file
grammar evidence, not native project execution or trailing-comma support.

The new adapter's readDevinProjectDocument passes native main/local policy
bytes into the strict portable JSON decoder, so an unrelated native comment
blocks installation. The new CodecDevin also currently takes native target
mcp_config.json through strict parseDocument(false), rejecting commented target
files before an otherwise exact owned patch. Legacy CodecWindsurf must retain
its current grammar and user behavior.

## Narrow implementation

Reuse the existing comments-only native AST mechanics already used for Gemini:
extract its private syntax admission into parseCommentsOnlyDocument(body,label)
without changing Gemini's accepted/rejected grammar or existing error text.
CodecGemini delegates with its existing label; only the new CodecDevin opts in
with a Devin label. Every other codec, especially legacy CodecWindsurf, keeps
its existing branch. Preserve the original AST and commentsOnly render flag,
existing trailing-comma rejection/cleanup, duplicate-key checks, native name/
path validation, no-follow IO, writer locking, entry digests and ownership,
transaction effects/rollback and receipt rules. Do not add a second writer or
change mutation/recovery semantics.

Add one small read-only nativeconfig.StandardizeCommentedJSON helper using that
same comments-only syntax admission followed by existing hujson.Standardize.
The adapter's native main/local policy inspection calls it before the existing
strict unique-key object decoder. Original files remain unmodified and exact
bytes are retained; the normalized bytes are only ephemeral inspection input.
Duplicate keys and undocumented trailing commas refuse, rather than accepting
all JSONC grammar accidentally. Do not relax portable plugin/MCP/manifest JSON,
change a general shared strict decoder, infer trust/policy, migrate old config,
or rewrite local overrides.

## Meaningful verification

Add a current-Devin native codec regression with line/block comments and exact
foreign disabled entry/policy trivia. Add/remove only the owned member and
verify foreign comments/member bytes survive. Refuse duplicate keys and trailing
commas through both the target codec and the read-only normalizer. Assert legacy
CodecWindsurf still rejects the same commented document; run affected existing
Gemini comments/grammar tests to prove its behavior and error contract remain.

At the composed provider boundary, accept unrelated commented config.json,
config.local.json and a foreign-name mcp_config.local.json while preserving each
file byte-for-byte. Refuse commented legacy main mcpServers and same-name local
shadowing with native/user/B inventories unchanged. Retain the exact scope,
artifact, receipt/data and attestation regressions, plus visible-write rollback
and committed-cleanup cases. Existing project CLI/SDK evidence remains separate
from genuine native discovery. Run bounded affected codec/provider suites,
changed/full-file lint and diff checks; freeze the correction for independent
R1/R2 implementation refresh before parent publication.

No native executable, trust/model/account/MCP process, installer retry, new
acquisition, hosted guard change, user capability expansion, main/release merge
or native runtime qualification is authorized by this syntax correction.
