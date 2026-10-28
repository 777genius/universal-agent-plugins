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
