# Grok shared HTTP client fixture: reviewed validation

Reviewed code: `74f72491fdbeb722e845de71217e9d1751330020`.
Tree: `53b0e7f46404658bd385b92edddea12522c247dc`.
Base: `a1af0da362f020111a3af6a16e50cd265a708224`.
Approved plan commit: `7e5e5629d3f730df7bb2ee17bf2765882cd74112`.

Two independent GPT-6.1 Sol xhigh reviewers approved the exact plan before code
and the final implementation after inspecting executor receipts. No blocking
source findings remain.

The existing Grok child environment receives `GROK_SAMPLER_SHARED_CLIENT=0`
only when its fixture holds the private provider. Eight concrete client/provider
cases verify routing, ambient isolation, no duplicate entries, retained original
HOME/profile/state/Git/key/updater values and no process-global mutation.

The provider source stays byte-identical, SHA256
`865265e28731ceee3816ab1d5c4fc137d29d859e6851675d90fbcc867f1daa88`.
Its exact authenticated POST route, request grammar, response, failure latch,
cancellation and shutdown remain unchanged. GET `/` remains rejected. No client
pin, config, trust/permission, command, timeout or qualification flag changed.

## Actual local inert gates

- Focused Grok/shared CLI Go contracts: PASS, package1.138 seconds.
- Python runtime admission: 38 tests PASS,5.503 seconds.
- Affected all-enabled lint: zero issues.
- Unfiltered revive/funlen/gocyclo/gocognit/dupl/depguard: zero issues.
- Original a1af native archive: all20 member bodies and the summary admitted
  unchanged; failed status and allfive/runtime/auth/model false flags preserved.
- gofmt, whitespace, source-manifest and unchanged-provider checks passed.

Only inert repository contracts and artifact admission ran locally; no vendor
binary or scanner ran. The public source supports this configuration experiment,
while equivalence to the shipping1.0.70/f50904eaee6c build remains unproved.
A new genuine disposable hosted capture must show whether the pinned binary
honors the setting and what request follows. The previous failure is not promoted
to success. This validation makes no lifecycle, authentication, model, main-merge,
release or deferred VS Code claim.

## Exact review and execution records

- `/tmp/uap-additional-grok-shared-client-code-R1.md`: SHA256 `15f043736ab0deadf9b985eafbb8fd7bf148e3853cb81d5631b249a06a5157cc`.
- `/tmp/uap-grok-shared-client-code-r2-review.json`: SHA256 `8e842b6b91bc559f53f47893d14ee0b7427d42b2955ef868876d46b6897ef842`.
- `/tmp/uap-additional-final-serial-gates-1d4m_qyl/report.json`: SHA256 `1ca286a35b6c73e4dc32b6ef5525741aa8b323bb726ceda86968c06c017e00de`.
- `/tmp/uap-additional-final-serial-gates-1d4m_qyl/grok-unchanged-old-replay.json`: SHA256 `82ccf6beb6b90bd35bfac3a0ef2eb983f62286a82bc99ce86c0909503b823479`.

Local receipt paths identify this execution. The committed source, approved plan
and this validation document preserve the reviewable repository history.

## Refused second main request: bounded diagnostic candidate

The separate accepted amendment is committed before code at
`bb94f00bba12d59b80bdd2bfd600aa3af6e66d72`, following two independent plan GOs.
It addresses the actual `348e2ce8` capture: one first main exchange and one title
exchange completed, then an additional main HTTP attempt hit the original
duplicate refusal before its body was read. That capture remains failed and
all five qualification flags, runtime, authentication and real-model claims
remain false. Its second body and purpose are unknown.

The candidate adds one main-only observation slot. Original context, literal
route, loopback and authentication checks run first. Under the existing mutex,
the completed first main request/response and distinct held title fixture permit
one atomic duplicate/failure reservation. Body reading and privacy validation
run outside the lock, at most 1 MiB + 1, under the unchanged five-second HTTP
read limit and original 60/150/180-second origins. No deadline is renewed.
The unconditional deferred path settles the observation and executes the
original HTTP 400 refusal and duplicate cancellation. A third or concurrent
request remains immediately refused and can cancel a blocked observer; it
cannot claim another slot, replace the first failure or increment counters.

Only a complete bounded UTF8 strict JSON object that passes both held-key and
recursive privacy checks is retained, opaquely. No field becomes a new model,
tool, prompt or response grammar. Every refusal retains zero raw bytes. After
the original stop settles handler/connection bookkeeping, export snapshots
state under the lock, then validates and persists outside it through the
unchanged evidence/privacy guards.

The optional fixed `grok-provider.duplicate.json` receipt is at most 2048 bytes.
Its closed fields are schema version 1, role `main-duplicate-diagnostic`, attempt
index 2, a finite status, original failure `duplicate`, false response-issued,
captured byte count and SHA256. `grok-provider.duplicate-request.json` exists only
for a captured status, within the existing 1 MiB member bound and with its exact
length/hash. Refused statuses have no raw member, zero length and empty digest.
Go and Python readers join the pair to the failed all-false tuple, original
first exchange and separate title evidence, and independently recheck structure,
privacy, member isolation and byte/hash consistency. Historical captures without
this optional pair retain their original admission.

New inert contracts are authored for the actual loopback first/second/third HTTP
sequence, original five-second partial read, concurrent cancellation and stop,
ineligible/title/incomplete-first requests, finite JSON and both held-key privacy
refusals, plus coherent hash-updated artifact mutations. These contracts are
synthetic and have not yet been executed for this candidate. The existing
main/title schemas and response bytes, command/config/profile/process guards,
native pins, selection and workflow are unchanged. Before publication this
candidate still requires exact-source executed Go/Python/lint gates, original
capture replay and two independent CODE critics. A fresh genuine hosted capture
and independent raw audit are required to observe any new refused body; even a
safe captured body remains failed, unqualified diagnostic evidence.

The first code review found a Unicode-scalar mismatch: Go's existing JSON
decoder replaces escaped lone UTF16 surrogates, while Python refuses their UTF8
encoding. The new diagnostic now refuses unpaired high/low escapes and invalid
pairs, including complete nested JSON values/lines in strings or keys, under
the existing 4x text, 16384 node and 64 depth privacy envelope. Old decoders and
privacy scanners remain unchanged. Valid pairs, literal U+FFFD, raw UTF8,
escaped slash/quotes and ordinary literal backslash-u text remain data. Retained
raw bytes are never normalized. Coherent refusal and raw-preserving positive
controls are authored in both languages; exact-source execution and fresh
independent CODE review remain required.


## Final refused-duplicate source checks, 2026-10-28

Exact code `221f4a60b8d179f5e8b2ac007fa57ce010957b3b`, tree
`0a1839161a76dd50acf03301d081d0d3bd793a36`, received independent root
and Gemini CODE GOs after the Unicode correction and a one-file equivalent
contract-style correction. All diagnostic helpers, Python and original guards
are unchanged by that final style-only commit.

The actual focused Go selection completed 56 top-level entries: 233 passed
cases and the unchanged child-only fatal-helper skip outside its child
environment. Its actual parent fatal-guard controls passed. All seven new
duplicate contract groups, including actual partial-body and concurrent HTTP
cleanup, passed. The 46 runtime and 12 Grok Python tests passed on the identical
Python scripts tree. Affected all-enabled lint and unfiltered whole six-linter
gates each reported zero issues. The first style-lint failure is retained.

Complete original and current Python modules replayed the original 348
25-member and 611 21-member archives under their exact original source trees;
all bytes and failed/all-false results remain unchanged. The inherited 348
Cline capture remains independently audited. No vendor client, public installer
or scanner ran locally. This publication only adds validation documentation to
the exact reviewed code. A fresh hosted capture and two independent raw reviews
remain required to observe the refused second body. No second response or
runtime, lifecycle, MCP, authentication or model qualification is introduced.

## Supported turn-summary suppression before original config freeze

The fresh diagnostic run `40610932429` on source
`b40da640c388d807cc3dc1fbb36c38e0d1b411ef`, tree
`71b98e819bdd555154096c9c75850b9b40d3f54c`, retained the refused second
main-route request: 1,424 bytes, SHA256
`303ad4817a23b68aa02717ecedb70404362cf1397324da5a4d25d12f60a86bb5`.
Two independent raw audits admitted only failed, all-false diagnostic evidence.
The system message and wrapper around the actual prompt and fixed reply exactly
match the pinned public source's per-turn dashboard-summary helper. This is a
specific measured side call; it supplies no plugin activation or runtime proof.

The accepted plan SHA256 is
`3bc56715f4b326634f99210fc7bde69a711c2edbe7c2880fece74ec950c4593e`,
committed before implementation as
`15a2ca3b8b0396a917121d94b4e991737f4a0aec`, tree
`7962f86b94c8749ce57b5bb4776bad6596b2a207`. Root and an independent reviewer
accepted its narrow supported-setting candidate before code.

The complete two-model TOML recipe now appends the native boolean
`[features] turn_summary = false` before the existing original HOME config's
sole write, authored-byte clone and protected digest freeze. The original HOME,
both model routes and credentials, source/config/permission/process guards and
all deadlines remain unchanged. No environment override, request allowance,
additional response, retry, profile adoption or qualification flag is added.
The independent complete-map contract adds only the expected boolean subtree;
new inert controls exercise the complete frozen recipe, changed/invalid feature
data without authority adoption, and ambient feature/model isolation.
The existing config diagnostic compares its original single-model recipe;
the two-model title recipe therefore retains its finite `recipe-mismatch`
refusal after a config mutation. No semantic diagnostic allowance is added.

Public source tree `2bdd1d6a6369de0e8c68132ea4539e9abd9e14a8` defines the
boolean feature, config precedence, actor spawn-time snapshot and early return
before dashboard-summary work. Unset later title refresh defaults to this same
feature, so the recipe also disables that later display-only refresh. The
separate initial-title model, request validator, fixed response and counter are
preserved. The public tree is not proven equivalent to the shipping
1.0.70/f50904eaee6c binary; the fresh genuine run must establish effectiveness.

All main/title handlers, refused-duplicate capture and privacy/export controls,
readers, collectors, workflow, pins, production and other client source remain
preserved. Any unexpected additional request still receives the same refusal
and original cancellation. Even a clean native exit reaches the unchanged
intentional failed raw-review barrier with runtime and every qualification flag
false. Exact code reviews and measured gates are pending for this amendment;
publication and a fresh hosted observation require those checks, followed by
two independent raw audits. This record conveys no new native qualification.

The author's bounded development selection ran the new configuration controls
and existing complete-map test: four top-level tests and ten cases passed,
with no skips (package JSON elapsed0.019seconds). The raw JSON log SHA256 is
`a3267a561a123b587f25dc8ae3015f2a883edd779c4293ab675f06e3046da66a`.
An initial new-test expectation incorrectly required semantic projection after
the unchanged single-model recipe guard; that failure is retained. Corrected
controls require its exact finite refusal, separately verify TOML/map changes,
and prove that observed bytes and protected authority are not adopted. These
are owned inert loopback/configuration checks, without a native client, public
installer or scanner. Final committed-source gates remain pending.
