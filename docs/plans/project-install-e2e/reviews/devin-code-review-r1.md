# Current Devin project adapter — independent code R1

Reviewed frozen `132b8c38226f87a39724637cf5ac11b7e0978ea9` in
`work/uap-project-windsurf` on 2026-10-23. Reviewer did not author the adapter.
**NO-GO pending the native-comments correction below.** The separately reviewed
shared host-version prerequisite `64c7ee6d` already has both implementation GOs;
its approval is not repeated or widened here. The selected-versus-sibling facts
amendment has both plan GOs before the exact three-line collector exclusion.
No vendor/native client, launcher, model, trust/auth or MCP process was executed.

## Blocking finding

`clients/windsurf/project_policy.go:readDevinProjectDocument` directly passes
native config.json/config.local.json/mcp_config.local.json into the strict JSON
decoder. `nativeconfig.parseCodecDocument` also routes new CodecDevin target
mcp_config.json through strict JSON admission. Current official config-file
documentation explicitly supports JSON comments and the pinned distribution's
documentation agrees. Ordinary unrelated comments in a main/local file or
target therefore block an otherwise qualified project installation.

Correct native policy reads and CodecDevin only using the existing comments-only
AST grammar, keeping duplicate-key/name/path checks, original foreign bytes,
receipt digests and ownership/rollback intact. Preserve legacy CodecWindsurf,
portable JSON and Gemini's existing grammar/error text. Reject undocumented
trailing commas rather than broadening to all hujson syntax. The author accepted
the finding and froze amendment `e09d06d3`; its R1 plan is GO in the separate
report, with R2/source/implementation closure still required.

Required regressions: commented unrelated main/local files and target patches
preserve exact foreign comments/disabled entries; commented legacy MCP and
same-name local shadowing still refuse without effects; duplicate/trailing
input remains refused in both reader paths; legacy Windsurf strict and Gemini
comments contracts remain unchanged.

## Inspected remaining boundaries

The new automatic lane requires the selected primary absolute clean devin or
devin.exe and exact host-observed version 3000.11.205. It does not infer current
Devin from legacy editor presence, probe from SDK/adapter, rewrite HOME, choose a
fallback profile or execute native mutation. An absent original legacy profile
is legitimate only under the previously reviewed project SDK admission.
The original user root must still agree exactly with TargetContext.

Plan/client/scope/native-root/artifact and exact managed package layout are
checked before effects. Direct activation requires the delivered package path;
pre-staging alone permits it absent. Previous/desired native receipt paths,
kinds, IDs, duplicates and source paths use the reviewed shared validation.
Binding inspection rejects copied scope/artifact/package/native roots. Active
stdio reuse verifies selected package/data/cwd launcher coordinates and reserved
environment values without rewriting immutable bytes. Project migration and
same-name higher local policy refuse; other imported/effective policy is
truthfully unobserved.

The dedicated current codec retains command/args/env without native cwd and
explicit remote transport, separate from legacy Cascade serverUrl. All writes
use the reviewed component transaction/kernel; the authored tests cover stale
ownership, collision, visible replacement rollback and committed cleanup
receipts. Project attestation is refused, and configured results remain prepared
with installation verification. Exact recorded removal does not require a
current executable/version and does not call default-local native remove.

The selected KnownTargets exclusion follows the existing sibling-only contract;
primary Request/prepared client evidence remains authoritative, while all
unselected sibling root/executable/BindingID checks remain unchanged. Four
compatibility goldens change only Windsurf's project scope list. Legacy user
source/selection and useful tests are retained. Public TLS Git/ZIP and SDK
scenarios are honestly fixture composition with no native runtime claim.

Independent execution of the final corrected frozen checkpoint will be recorded
with closure below. The author's existing passes remain separate evidence,
not independently rerun results in this interim report.

## Additional exact client authority finding

Concur with independent R2: `validateDevinProjectBinding` must also require literal `binding.ClientID == "windsurf"`. Scope/root/artifact/object checks alone allow a copied contradictory owner to reach direct binding inspection. Require refusal in direct validation/inspection with a copied-client-ID unchanged-effects regression. The author accepted this original exact-client authority correction; final R1 remains pending both this and native-comments closure at a frozen SHA.

## Final corrected source and integration closure

**GO at clean frozen `d69091b3328a1b626d9475643e8e97a8bf040b4a`.** Both accepted blockers are closed. Both native-comments plan GO reports precede source4f5d9558. CodecDevin and read-only policy inspection now reuse the existing comments-only AST admission: duplicate keys at every nesting level and undocumented trailing commas refuse. Strict normalized JSON is used only for read-only inspection; original file bytes and AST remain the mutation/ownership authority. Gemini's existing comments grammar/error text and legacy CodecWindsurf/portable strict admission are preserved. New tests cover unrelated commented main/local policies without rewrites, commented legacy/shadow refusal, foreign target trivia preservation, duplicate/trailing refusal and legacy strictness. Exact literal Windsurf ClientID is required by shared project binding validation, with direct and inspection copied-codex refusal.

Independently inspected all three manual union conflicts in the true merge of reviewed d39: Codex TOML dispatch remains first, Gemini plus Devin comments-only parsing remains separate; project server dispatch retains Copilot and Devin with all prior codecs; supportedCodec retains Codex, Copilot, Devin and every previous codec. No authored Windsurf/provider/SDK path changed from4f5 to the merge, and every other client directory is byte-identical to d39. No new semantic merge resolution found.

The authored public TLS Git/ZIP/SDK/provider/legacy passes remain separately attributed evidence. Parent reports the other independent reviewer ran final full nativeconfig PASS3.471s and preserved legacy client PASS0.006s. This R1 did not repeat those results or execute a native client. Local compiler capacity was exhausted; parent explicitly directed static final closure instead of redundant broad rebuilds, reserving the next slot for genuinely new Source corrections. The complete static review and targeted unchanged-authority regressions leave no code blocker. Genuine project discovery/trust/session/MCP/native loading and release qualification remain pending; prepared/installation evidence must not be reported as attestation.
