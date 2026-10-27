# Copilot measured native environment and closed JSON refusal diagnostics

## Exact evidence and current source

Current PR527 branch is 5afd897d4b10de6286d1e0c97d358ec4e15f0250,
tree94aef6b1ddc3cc55c97c0917d46539c64273d670. Local baseline
a51ec63cc094c979ecbf8147ed7ad2f5eed8f8cf has that exact tree and preserves
the externally published runtime workflow blob66fb12161a3d1cfe6992cfeada8fbf44ad54fda6.

Actual genuine run40408790152/native job121914618336 used pinned Copilot1.0.103.
Closed native artifact12697784970 has outer SHA256
773c1374fa322f4f52bbcf875d230e6f8643ff53431bbfeabbc4485b4a92d544;
frontend12697996095 has SHA256
6de4ae11c713bfa6cb0ca9c9042c7678c87f1989d5bec63abbbe62a56846db77.
The installed mcp.json SHA256
2bb36b95df074e37f845fac66474a0dc1426938386b2703ae71df9453f4c8496
retains UAP_TEST_LITERAL as exactly "literal ${UNKNOWN} $HOME".
Independent native output, provider request2 and probe tools/call agree that
the native process receives exactly "literal ${UNKNOWN} " plus the held
original fixture HOME. All other eleven facts match selected source/binding,
data, nonce, revision, arguments, once-expanded root, marker, PID and session.
The current validator instead expects the unexpanded $HOME and rejects it.
Separately, original config bytes change277->423 and the retained strict
diagnostic says invalid-json; changed config bytes/cause remain unobserved.
All qualification flags remain false. These are two distinct refusals.

The publisher's current official CLI reference documents $VAR, ${VAR} and
${VAR:-default} expansion in local MCP env:
https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference#local-server-configuration-fields
That current documentation corroborates the exact pinned native capture; it
is not asserted to be immutable version1.0.103 source. Repository loader and
nativeconfig contracts preserve unknown strings during materialization; they
do not establish a universal guarantee against a client's own later expansion.

## Proposed finite native expectation

After two independent plan reviews, change only the Copilot fixture's expected
runtime Literal to exactly "literal ${UNKNOWN} " + f.base.Home, using the held
original HOME already selected and validated by the existing fixture. Never
derive it from an ambient host HOME or provider-returned value. Do not expand
arbitrary strings, change HOME, accept both forms, normalize captured facts or
modify a production adapter, portable source contract or another client.
Keep the installed/source MCP literal, its captured immutable bytes/digest and
all package/receipt/projection checks unchanged. The unset ${UNKNOWN} text,
unknown argument, once-expanded path and every other exact fact/event check
remain mandatory. This tests documented Copilot-native expansion; it does not
claim that $HOME remains literal in its executed environment.

Extend the existing actual facts/event contract with a distinct original HOME
and exact positive native value. Add refusal cases for unexpanded $HOME,
foreign HOME, project-as-HOME and changed UNKNOWN text. Construct each negative
consistently in the event and returned payload before invoking the validator,
so an unrelated event mismatch cannot mask the literal assertion. Retain all
existing wrong-authority/session/order/missing/duplicate/extra-field cases.
No new native response or local vendor execution is authorized.

## Independent profile refusal diagnostic

Keep the original protected-digest error and comparison fatal and unchanged.
Only when the existing bounded, hash-stable diagnostic has already obtained
both bodies and encounters its existing invalid-json branch, record two
standard-library json.Valid booleans for the baseline and observed bodies and
a finite enum identifying baseline, observed or both as strict-parser failures.
Also record each failing side's finite error category: syntax, unexpected-eof,
duplicate-key, body-limit, structure-limit, key, object-end, array-end,
unexpected-delimiter, nonfinite-number, trailing-data, or other. Determine
categories only through typed errors/is and exact existing static error text;
never emit an error string, raw token, key, value, path, comment or body.
Unknown errors map to other. A valid side is explicitly valid. No JSONC parse,
fallback, relaxed validator, new file read, larger bound, trust rebaseline or
profile mutation is allowed. Existing hashes/lengths and diagnostic statuses
remain intact; extra fields are optional and confined to invalid-json.
No conclusion about JSONC, changed keys or native ownership follows until a
new genuine source-bound capture provides the additional measured diagnostics.

Extend existing bounded diagnostics contracts with syntax-invalid,
syntactically valid duplicate-key, baseline-invalid, both-invalid, and
nonfinite/structural rejection controls. Check finite outputs and absence of
private sentinel keys/values/errors. Preserve all existing alias/hardlink,
unstable/read/size/deadline/change-limit and fatal protected-digest assertions.

## Scope and verification

Scope: the existing Copilot native attempt, its facts contract, diagnostic and
diagnostic-contract Go test files, plus this plan/validation record. Preserve
provider protocol and tool name, source/pins, host admission, original total and
per-command deadlines, trust/permission, native JSONL capture-review gate,
process supervision, artifact admission, cleanup and all qualification flags.
No product, shared behavior, other client or VS Code change; VS Code is deferred
by the latest explicit owner instruction.

Two independent plan GOs precede a separate plan commit and implementation.
Independent code critics must challenge the source-vs-native semantic boundary
and preservation of the unchanged fatal profile guard. Under the exclusive Go
slot run focused existing/new Copilot facts and diagnostics contracts plus
affected Required changed-line and whole-file architecture/size lint; gofmt and
diff checks. Do not run a vendor locally. Root publishes to the same PR using
fresh actual Git ref/expected_sha, force=false, preserving external changes.
A new genuine hosted attempt must independently confirm the runtime mapping
and diagnose or pass the profile boundary. This amendment itself establishes
neither complete startup nor A/B lifecycle/restart/removal/MCP qualification.
