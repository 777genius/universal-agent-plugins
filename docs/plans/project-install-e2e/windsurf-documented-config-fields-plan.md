# Windsurf: classify documented added config fields without accepting mutation

## Actual current boundary

Start from PR528 published `370be21d5095fa336e7737187e0c86e90caced72`,
tree `f96816db3d920ad2d2e7edaf070f7a0cd3e02f1b`, exactly matching local
`4bbcd088ee7d00f59ee31575e9ec3c16dce99928`. Preserve the prior diagnostic,
its original refusal, all external CI/Windows fixes and the earlier worker
branch. A fresh real git-ref read still returned this source before this plan.

Genuine runtime `40456808127`, native job `122036053909`, admitted exact clean
source and built successfully. Public add-a exited0/error=false/deadline=false
in999ms; the installed native attempt exited1/error=false/deadline=false in198ms.
Its22-byte stderr has SHA256
`a79305edd7145c6d6d6ee8e6aea8cb331cb1aa797bf1e085b3fa469a5323e68d`
and contains the same login-canceled refusal. Eight-byte stdout has SHA256
`30bd97c63fea7c0b9944d97a1dfd74ebb982be38458e6e374b3b36ad4cf3886f`.

The existing diagnostic now proves a strict-JSON object with three top-level
fields. Both authored fields, auto_update and permissions, have the expected
types and exactly match the held authored recipe. The third key/type/value are
unobserved. Expected config SHA256 is
`5bccc8c4fb7bd3ade05497c19d184bb11e25b8a14958c61b47a6eddb565a5a23`;
observed is `d84675cac14c039e735f50384f3d0cdd82a91c7bf140c42b4e211ebadaa8e844`.
The original protected-file digest guard still fails. Matching two fields does
not prove the whole config is unchanged, nor authorize adopting the third field.

Native artifact12712979175 outer SHA256
`5a745683b6b063cc0383a4084f05a17d833ce6d5118b0e3fc1b634c71511d2e3`,
inner `1b74f4ab3fea52f4a3aa90b61437d22a9e7039f7f109e217ff813e55d56a0951`,
has16 members. Frontend12713064058 outer
`e30a49220384956eca21cb41b1c337e96d240e711d497492d3d6e4257f0f9526`,
inner `cb206fa9acdf628b0044825d08ca12fa75fbd5ea0fd348a9cd3d0b6e3b5dba6e`,
has25. API sizes/hashes, bounded canonical unique ordinary closure and CRCs,
exact source/tree/clean receipts and all24 frontend command streams were checked.
The unchanged16-member native admission replays as failed/incomplete with all
five qualification flags false. Authentication/model remain false.

## Pinned authoritative names, distinct from actual new values

Retained tiny official3000.11.205 documentation was independently matched to
the complete archive member receipt in that same source-bound frontend capture.
The archive pin remains
`2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4`.
These are exact member SHA256 values, not hashes inferred from current web docs:

| Official archive member under share/devin/docs | SHA256 |
| --- | --- |
| reference/configuration/config-file.mdx | c43f1a70135662c6124ae794928a4719bcd0bcb2d198573c2bf39c76935acd91 |
| reference/commands.mdx | 996f969ba371f4dae62d70b63f7a6d5473ed48c3633abf9b120d9214cf0c8b75 |
| extensibility/skills/overview.mdx | 194e413e7a4478cb7b59980200865614a8d33694db3ae572dfc5d81270310948 |
| extensibility/mcp/configuration.mdx | 923fd0c02c081ddfa5744a6ac293c50f31654e56858af464f89b4046e1a8ee74 |

Current canonical references are https://docs.devin.ai/cli/reference/configuration/config-file
and https://docs.devin.ai/cli/reference/commands. They corroborate the documented
surface; exact release-member hashes above supply the pinned source authority.

The config reference names16 additional config-related names to classify only
if present at the observed top level, beyond the two already compared: agent,
attribution, include_gitignored_files, keymap,
local_tools_bypass_permissions, mcpServers, notify, proxy, read_config_from,
respect_gitignore, sandbox, show_hints, show_path, subagents_enabled, theme_mode
and unicode_mode. In particular, the reference says current mcpServers belongs
in dedicated mcp_config.json, with startup migration from older main configs.
This registry does not declare all16 names valid main user-config options.
Their presence in docs does not prove which one was added, its actual value,
harmlessness or eligibility for any future baseline.

## Narrow implementation after two independent plan GOs

Scope is the existing Linux Windsurf config-diagnostic helper, one new bounded
Windsurf-only contract file if needed to retain the current file-size gate, and
this plan/validation record. Do not change the shared command hook or any read,
guard, native argv, protected map, environment, permission, trust or auth policy.

Only after the existing held ordinary64KiB read matches the digest already
observed by the original guard, strict whole JSON parses uniquely, and the
existing authored recipe is valid, add an optional additional_fields section
to the existing captured diagnostic. It has exactly a documented object and
an unknown_count integer. The documented object is constructed by iterating
only the16 fixed source literals above, in fixed order, recording a key only
when that exact spelling is present. Its value is only the same finite JSON
type category already used by this diagnostic: null/boolean/string/number/
array/object/other. Factor that existing pure type projection if useful;
retain both existing authored presence/type/equality facts byte-for-byte in
meaning and keep strict duplicate-key/trailing-data parser behavior unchanged.

Count remaining unclassified top-level keys without copying their strings,
hashes, names, values, nested keys or error text into output. Exclude actually
present authored keys and actually present exact registry names from the count.
Case-folded, homoglyph, control-bearing and arbitrary private names stay unknown;
never render or dynamically adopt them. Empty documented is an actual empty
object. Wrong types remain a reported type, with no value or acceptance claim.
Do not walk nested config values or compare them with presumed vendor defaults.

No additional field section appears on refused read/digest, invalid JSON,
non-object, deadline or recipe failure. Preserve the existing whole serialized
diagnostic2KiB bound and fixed diagnostic-bound fallback. No new body read,
profile mutation, stdout/stderr member, network/service/process or exception
serialization is added. The original protected digest check must still fail
on the exact same changed file after this diagnostic is collected. No snapshot
is renewed and no native-generated field is accepted.

## Meaningful validation and publication

Use actual controlled files through the existing eligible post-native hook:

- A three-field document with each fixed documented name in turn, private value
  canaries and every JSON type yields only the static name and finite type.
- All16 documented fields together plus the two original authored values and
  unknown private/control/long-key fields stay within2KiB. Exact unknown count,
  unchanged original-field comparisons and no private text/value leakage hold.
- Exact spelling, case variants and Unicode lookalikes are distinguished. No
  unknown key/value, nested canary, host path or OS error appears in the result.
- Invalid/duplicate/trailing/non-object/oversized/changed-held-digest, symlink,
  hardlink, missing/foreign client/path/triple and expired-context controls retain
  the original refusal/omission behavior. Preserve all existing contracts.
- Held file metadata/bytes and original protected digest remain exact before/
  after every diagnostic. No projected classification becomes successful native
  completion, authentication, trust or any qualification.

Run only focused Windsurf/shared CLI inert Go contracts and the two actual
affected/whole-file size/architecture lint gates in a granted exclusive slot.
The unchanged Python envelope may replay the existing16-member failure; do not
run local Devin or expand optional tests. Two independent code reviews and root
final review precede root-only non-forced publication to existing PR528 after
a fresh actual ref lease. A fresh genuine native run supplies the third-field
classification. An unknown or unexpected result requires its own evidence-backed
decision; do not expand this plan into config acceptance. No release/new PR/VS.

## Component lifecycle assessment, separate next slice

The same pinned command docs and previously observed fixed help expose
`skills list --json`, `skills paths`, `mcp list` and `mcp get NAME`. MCP list/get
have no observed --json flag; do not invent one or borrow a skill JSON schema.
The docs describe these as component discovery/configuration inspection, separate
from ordinary prompt startup and login. They are plausible routes to observe
effective component sources before the known login-canceled startup, but no
actual list/get response or absence grammar is yet established by this lane.
Help and documentation do not prove that they execute without account/trust
effects or that a listed MCP server was initialized.

After this diagnostic slice, prepare a separate finite source-bound hosted
observation plan for those exact read-oriented commands over empty/original-user
and receipt-installed A/B. Keep original config/foreign/user/B guards and reject
any native write. Capture all actual output opaquely with existing bounds before
freezing a parser; no positive listing fact is fabricated. MCP get must address
only the prior recipe's exact owned name. Never call native add/remove/login to
make the result convenient. Unknown output, permission/auth refusal or config
mutation remains failed for raw review.

If those observations prove effective source/path/skill-body and native MCP
config attribution, a later independently reviewed public full component cycle
can cover user/A/B preservation, repeat add/unchanged update/intact repair,
A-two update, owned missing-leaf repair, fresh post-repair discovery, removal/
repeat refusal and retained caller/owned data. It must first resolve the actual
canonical Windsurf target's preserved legacy user route versus the current Devin
project harness; do not silently relabel one as the other or manufacture a
public current-Devin user installation. Reuse exact existing state/receipt/
transaction authority and preserve original data retention semantics.

Such listing evidence is component discovery, not MCP initialize/tools-list/
tools-call or ordinary authenticated agent runtime. All runtime qualifications
remain false until their own real protocol/process/cleanup evidence exists.
The known login-canceled prompt and unknown config addition remain distinct
open boundaries; this assessment does not remove either.

## Actual implementation validation

Root and CLI independently approved original plan SHA256
`dafc84a906e397d57821c12cce6d25fe49dce01c7b6d96447ab6b9a016d719cf`
before source edits. CLI and shell reviewed the complete two-file code, and
root independently reviewed its source/authority boundary. Final helper SHA256
is `06601096db60fd45cc73bc0dccb1e8cf42838e7e86f4d3e1626b99a32aa97b79`;
new contracts SHA256 is
`6b4295ad7406791429fd1f5d2c173c5175d54641146d683057579302efeedaa3`.

Actual focused inert Go contracts passed in `0.077s`, including all16 fixed
names with six JSON types, complete-registry/private-key/type/count/empty-object
controls, matching-digest parser refusals and retained Windsurf/shared argv/tool
restrictions. Actual all-enabled affected lint and unfiltered whole-file size/
complexity/architecture lint each reported `0 issues`; gofmt/diff checks pass.
No source correction was needed after either code review or these checks.
The unchanged Python envelope had already re-admitted the actual370 source's
16-member failed capture with all flags false. No local vendor/positive native
guard ran. Original protected digest/fatal/maps and all qualification flags
remain unchanged; a fresh genuine run must identify the added field.
