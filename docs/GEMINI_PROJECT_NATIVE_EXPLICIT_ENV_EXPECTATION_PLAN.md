# Gemini: compare native probe facts against the pinned explicit-env expansion

## Source-bound actual failure

Published PR526 head9f81687411249640a11a54650888cec76c4820d5 has tree
77539fdb1b365db9b1a0c4dd570525b18c08123f, identical to local637e7e7f.
Preserve all earlier provider signature/wire-ID, research refresh and terminal
diagnostic corrections, raw evidence and strict source/receipt/cleanup gates.

Genuine runtime40459797233/job122045178580 now passes the prior provider part
union/HTTP identity boundary and reaches returned probe fact validation. It
fails with provider_error="returned probe authority/nonce/revision/cwd/marker
differs", provider_returned=false, exactly two recorded routes. Native exit144,
error=false, stdout2586 bytes; stderr1951 bytes has SHA256
7626d482fac60c559c01d9733c6a61b7ba59b8097060cb972730c648292f90bb.
This complete failed stderr remains refused; no terminal-prefix allowance can
turn this nonzero command into success.

Closed native12715011738 outer SHA256
fb0c40cfa2bec951500007cba314ebf7bbc502a6f40c45fcbbe2350cd0314b5c
has21 members; frontend12714812896 outer SHA256
05066a3ce792664e816ad1bba98135183ffafa1240b50621c60d51399f2cb44d
has25. Canonical unique ordinary ZIP/CRC closure, exact committed source/tree,
empty clean-source streams and all24 frontend command stream byte/hash joins
were verified. All lifecycle/MCP/auth/model qualifications remain false.

The exact second provider request's untrusted-context JSON and independent
tools/call probe event agree. Comparing against held provider arguments and
the captured receipt gives this complete result:

| Fact | Actual comparison with original independent expectation |
| --- | --- |
| nonce, revision, cwd, plugin_root, plugin_data, marker | exact match |
| argv | exact match, including literal third argument ${UNKNOWN} |
| pid, session | positive PID and nonempty session; independent event agrees |
| literal | exact `literal  ` followed by the original fixture HOME |
| once | exact held target locator followed by `/` |

Only literal and once differ from the current validator's unexpanded env-value
expectation. No authority path, receipt, call identity, marker or argv mismatch
is being exempted.

## Exact pinned native behavior

At google-gemini/gemini-cli@6006c14a4decfe4b891c6263ad6ad5992e404b5b:

| Source | Git blob | Full-file SHA256 |
| --- | --- | --- |
| packages/core/src/tools/mcp-client.ts | 3d2c7bebf61f1b20cb020dcebe352a786b652aba | 798702155ab6e454392140f0057cc88a1370deb0df6fe9fe45f4f2147c1ecf0d |
| packages/core/src/utils/envExpansion.ts | c17d91e1f75ebe3690d575ecffbf732630b74a57 | 87bc78e960ca70985c1d687ebc8aaab65a5a37b084f720352e2f8d501cf371a1 |
| packages/core/src/services/environmentSanitization.ts | c0aceef5806473c6d4a7894034d18d620f108e91 | a739daa890975fb8c1bccbca0ec47f10f9b305fb120991de41025f4b6fe27be9 |

Complete source files were independently fetched and git-blob hashes recomputed;
copies are under /tmp/uap-shell-fresh-oct27/pinned-gemini. mcp-client.ts
2348–2381 sanitizes the process environment and applies expandEnvVars to each
explicit MCP env value, while passing argv unchanged. envExpansion uses
dotenv-expand and specifies that missing variables become empty. Sanitization
always allows HOME. The actual Go f.env explicitly supplies the original HOME
and contains no UNKNOWN key. Thus the pinned native second pass expands the
authored literal env value to `literal  ` + held HOME, and the once env value
to held target locator + `/`. This is native behavior after public UAP
materialization; the installed source bytes remain independently frozen.

## Finite correction after two independent plan GOs

Scope: repotests/agentplugins_gemini_project_native_attempt_test.go and the
existing native contract file, plus this plan. Change only the two expected
native facts: compare Literal to exactly `literal  ` + f.base.Home and Once
to exactly f.binding.TargetLocator + `/`. Derive expectations only from the
already held original HOME/receipt target; never expand, trim or normalize a
received value or consult arbitrary ambient variables. Never accept both the
old and new forms. Do not write a generic env expansion engine or change UAP
production interpolation, package source, installed settings or native env.

Keep the complete strict fact envelope, nonce/revision/argv/root/data/cwd/marker,
PID/session, provider ID/schema/route and independent probe event ordering/
identity checks unchanged. In particular argv's ${UNKNOWN} remains literal.
Preserve exact nonzero/error/capture/cleanup failure and raw-output admission.

Add inert controls through the actual full validateFacts path, with real owned
event-file captures and independently authored held HOME/target/data/nonce.
The exact expanded native fact set with its matching initialize/list/call event
closure passes. Old unexpanded values, wrong or sibling HOME/target, malformed
literal spacing, re-expanded argv, foreign root/data/nonce/marker/revision,
wrong PID/session and mismatched probe events still fail. Replay the actual
retained request2/probe event bytes against their captured receipt/call/HOME
authority offline; no native/vendor process or hosted guard runs locally.

Run focused affected Gemini native contracts plus the retained raw replay,
then actual affected all-enabled and whole-file size/architecture lint under
the exclusive Go/lint slot, gofmt/diff checks and two independent code reviews.
Keep any temporary raw replay fixture out of the committed tree. Root alone
publishes to existing PR526 with a fresh ref/lease; no release/new PR/VS work.
Require fresh genuine source-bound success through provider/native/probe and
cleanup, and the complete separate lifecycle proof before qualification.
