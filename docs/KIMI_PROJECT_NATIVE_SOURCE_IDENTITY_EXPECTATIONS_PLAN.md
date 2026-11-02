# Kimi: two exact source-bound observer expectation corrections

## Baseline and actual observation limits

Use published PR524 source91a3c54ac74002c0bf67977d3107230f9bbbca30,
tree17957bb4494ad7139a1e5e6fe8a26bbd27f418ce. Preserve its exact private-root
fix and all prior reviewed/external source. New run40493669986 progressed
through context: component job122158328387 completed empty observation and
four public installs, then refused skills-a/validate; single-tool
job122158328503 reached its first provider request and initialize/list events,
then refused approvals/validate before any approved tool call. Both retain
quarantined response bodies, failed/incomplete/all-five-false qualification.

The four closed archives, API-correlated hashes, exact source/tree/clean
records and48 frontend stream joins are recorded in
`/tmp/uap-shell-fresh-oct27/kimi91a-source-stream-audit.json`, SHA256
1b68024b3b0a872c9e396bd5c9d1ad1cae34127a39b702215649294a5cfeb970.
Component native12730973797 has31 members, frontend12731208487 has25;
single-tool native12732047373 has23, frontend12732022524 has25.
Do not describe the two source contradictions below as the captured response
values or the conclusively measured causes. They are independently provable
observer defects at the two actual failure boundaries; fresh native evidence
must decide whether another mismatch remains afterward.

The separate finite-refusal diagnostic plan a8604e9c is available but is not
part of this correction. Do not combine its instrumentation into this slice
or publish duplicate patches while an earlier exact-source run is active.

## Exact source proof

All sources below belong to pinned MoonshotAI/kimi-code
b8dd3a134060c6e80ba4a1659335cd63b68483f2. Full files are independently
retrieved and Git-blob/SHA256-verified under
`/tmp/uap-shell-kimi91a-pinned-source`.

| Source | Git blob | SHA256 |
| --- | --- | --- |
| features/skill/catalog/registry.ts |9d07a1b16b125c4e4b4f07358d34d42437434fc2|265fa39bde3a7a0a58d7772b2769122b58da00316bb4e87bbd39bd91c74762d2|
| app/plugin/manager.ts |f917017e1524964566dc0404f4aafa26fa6b137d|214440d3e0bc69b83f37110878923ff427cff3f898ad5c14ca6999af5e266316|
| agent/loop/turnOps.ts |ec3921a37b4333e8034ccfa0d69380026b819e98|eaa5dcfa8b21784a0c77da2d4e818d34a051a9cfc769e5aa02e4111522847746|
| agent/toolApproval/toolApprovalService.ts |96f36f15ec9fcaf00f4ec373e935004ed14857e2|f1f79066f2184a112774fb1f025cb25580f991f4a9c14aa73104a0cfc439036d|

Paths in this table are relative to packages/agent-core-v2/src.
`PluginManager.pluginSkillRoots()` uses source extra and a held plugin.id.
`InMemorySkillCatalog.registerPluginSkill()` stores the bare plugin skill
separately and stores the public listSkills row with exact name
`${plugin.id}:${bareName}`. The workspace route builds that catalog and calls
listSkills/toProtocolSkill; its blobde5525e6/SHA
9f820a2cb91eef9a1a24a0fb439fece7da9da9c6bdd956045b16b57c32ba31ec
was also verified. Current UAP user activation registers the installed plugin
at original HOME/.kimi-code/plugins/installed.json; it does not copy that
plugin skill into project/.kimi-code/skills. The independently authored user
plugin ID and bare skill name are both uap-kimi-native-user. Therefore the
expected native user row name is exactly
uap-kimi-native-user:uap-kimi-native-user, source extra and the same owned
SKILL.md path/digest/description. Current observer expects its bare package
name. Project direct skill names remain exactly bare a/b/c names.

`turnOps` initializes nextTurnId=0 and assigns the first TurnPrompt its
current nextTurnId, then advances it. The single-tool observer creates a
fresh session, verifies message_count/last_seq, then submits one prompt.
Approval service carries context.turnId into interaction tags; pinned
kap-server approval route b90f1023 maps that numeric tag to turn_id. Current
observer requires turn_id>=1, contradicting this fresh first-turn identity.
Require exactly integer0 for this selected first-turn approval, rather than
accepting an arbitrary nonnegative or later turn. Bool is not an integer.

## Finite source delta

1. In the component Go observer request authoring only, construct the expected
   native skill name from the already held fixture scope/package identity:
   exact user role maps to pluginID:bareName; a/b/c remain unchanged. Keep
   package/manifest/registry/NativeObject/file-frontmatter names, source extra,
   description/path/digest and all public/receipt authority unchanged. Do not
   rename installed files or normalize an incoming native name. A small pure
   private helper must reject unknown/mismatched fixture role/name inputs.
2. In the existing Python single-approval validator only, require the exact
   fresh first turn integer0. Preserve the same current session/main agent,
   provider-owned call ID/tool/arguments/action/display/approval-ID/timestamp
   checks and ordering. Missing, bool, string, negative, positive/later turn,
   duplicate/unknown field and all current permission negatives still fail.
   Preserve ResponseSchemaRefusal separately and the graceful initial-capture
   branch; no additional approval, retry, request or remembered scope.

Scope: one existing Go component lifecycle request-authoring file, one bounded
new private Go expectation helper/contracts file if needed, existing Python
attempt validator/tests and component collector tests, and this plan/validation
record. No production adapter, installed source, shared process/host guard,
context/environment, native pin, provider behavior, diagnostics, cleanup,
deadline, archive admission or qualification change. Keep source history of
the two corrections separable in local commits even if root publishes them
together after final review to avoid unnecessary duplicate native runs.

## Required meaningful verification

Go contracts must use the actual authoring seam with real held user/project
proof fixtures to show that only expected native user name differs; selected
path/digest/description/source remain the existing exact receipt authority,
all four scopes reject malformed roles/names, and no passed receipt/name is
adopted as a new package or native path. Python installed-skills contracts
must accept exact source-shaped qualified user+A/B/C rows and reject bare
user, foreign plugin prefix, wrong scope, duplicated/missing/project sibling,
wrong path/description/source and malformed fields. Keep the generic catalog
grammar strict; only the expected held request name changes.

Single-tool contracts must use actual session/prompt/approval observer seam:
source-shaped first turn0 reaches the existing one-call approval and ordinary
shutdown path; every wrong turn class/value remains a refusal before that
action. Correct the old inert service's fabricated turn1 to source-derived0
so existing session/permission/schema/privacy cases remain meaningful, not
masked by the stronger first-turn equality. Explicitly retain wrong tool/
session/agent/provider call/action/display/args and unknown-key controls,
ResponseSchemaRefusal behavior, no-call-before-approval events and no blanket
scope in the outgoing approved request. No native/vendor runs locally.

Obtain two independent plan GOs before code and two exact-code reviews after.
Run affected existing/new Kimi Go contracts plus the complete Kimi/supervisor
Python suite, formatting/AST/diff and affected/whole-file lint in the granted
exclusive Go slot. Root alone publishes the frozen plan/code on fresh PR524
head. Both installed lanes then need new genuine source-bound raw proof;
single-tool native-history capture remains deliberately unqualified even if
the provider/probe exchange advances. Any next grammar/permission discrepancy
needs a separate evidence-backed plan; neither source correction qualifies
authentication, model service or a previously failed artifact.
