# Source PR473: fresh reconstruction of the five reviewed corrections

2026-10-24. Unpublished source8d039073 and its local Git objects were erased by
the runtime reset. Prior source98cb9383, original correction plan/amendments,
two plan GOs and two final code GOs are retained conversation history only.
They do not prove reconstructed bytes or authorize inventing old test results.
This restores their bounded contract against publishedbb6be986. Obtain two fresh
independent plan confirmations before code; run fresh relevant tests/lint and
two independent final code reviews before root publishes the exact checkpoint.

## 1. Decode archive selectors exactly once

URL parsing already decodes Fragment. Feed u.EscapedFragment() to the existing
percent decoder with literal plus preserved (replace raw + with %2B before
url.ParseQuery), and clear both Fragment and RawFragment on the cloned
endpoint before normalization. Local selector grammar is already encoded input
and remains unchanged. Test HTTPS/local parity for literal plus (c++), ampersand,
literal percent/%41, slash, Unicode and ordinary selected paths. Preserve malformed
percent, duplicate/unknown keys, invalid SHA and unsafe path refusals. Do not
change source identity, manifests, archive acquisition or auth behavior.

## 2. Fresh transitive alias completion without invented ownership

nodeRequirement must preserve precedence: existing pin, explicit root alias,
completed chosen package, Directory distribution and Git repository shorthand.
Only the final fresh/unowned transitive URL-Git/archive acquisition fallback
uses an empty operational Alias so the real acquired manifest supplies its name.
Remove the invented candidate fallback; do not reserve that name or replace an
explicit/pinned/root alias literally named candidate. Public Requirement.Validate
and SourceKey remain strict and still reject an empty alias.

Because SourceKey validates Alias before deriving the alias-free source tuple,
the narrow internal graph/acquisition key boundaries may clone ONLY an empty
Alias to a temporary valid scalar for invoking that same strict SourceKey. The
private key helper does not relax endpoint/path/selector validation, introduce
a public option, mutate the original request or persist a placeholder in graph,
lock, receipt, saved declaration or owner identity. Archive completion, like
Git, derives manifest.name only for empty/unqualified aliases. Validate true
manifest alias collisions; do not invent a new name to make them pass.

Meaningful composed tests must use actual Coordinator, production CLI resolver,
loopback TLS Git/ZIP acquisition, strict loader and digest store. Cover two
fresh archives, two URL Git tuples without Repository shorthand and a mixed
pair, distinct acquired names and RequiredBy edges, explicit root candidate,
existing pinned transitive candidate replay and moving explicit-root update.
Actual request counters must establish genuine reacquisition where required.
No client launcher/model executes. Preserve exact prior lock identity.

## 3. Controlled Git package-path diagnostics

Classify only locally established package/path/tree facts with a private typed
error: missing selected directory; exact file/symlink/executable rather than
directory; malformed/outside tree; selected submodule. Existing GitHub public
wording remains embedded unchanged. Direct Git reports a controlled static
reason plus its already validated selected package path instead of falsely
calling every failure a credential problem. Unrelated stat failures remain
unclassified, as do arbitrary runner output/error strings. Context cancellation
or deadline wins first. Never print raw Git stdout/stderr, credentials, tree
bytes, signed redirect data or arbitrary error strings through the new path.
Use tests with secret-looking runner failures to prove redaction and cancellation,
alongside actual selected-path cases; no native transport guard is weakened.

## 4. Actual ZIP CRC corruption proof

The old corrupt-payload fixture's fixed offset can alter filename/padding or a
deflate control bit while leaving content unchanged. Derive an actual compressed
payload offset through archive/zip DataOffset and flip a checked byte inside that
payload, avoiding its leading deflate control byte. Add a stored-entry regression
that first proves central metadata remains readable and the actual changed data
produces zip.ErrChecksum. Then require production recognized-archive refusal
without a byte pin or Git fallback and an empty acquisition temporary root.
Do not change production archive code, safety assertions, digest policy or caps.

## 5. Azure documentation table

Escape only the two pipes in Azure's selector alternatives inside the GFM table.
Keep its three column delimiters and all GB/GT/GC alternatives intact.

## Verification and publication boundary

Use fresh workspace Go1.26.10 and caches; coordinate at most the two bounded
GOMAXPROCS2/-p2 compiler slots now authorized by root. Run targeted new parser,
operational-key/public-strictness, controlled-error/CRC and real composed graph
cases, complete sourcespec/environment/sourceacquisition packages as appropriate,
and relevant actual Direct CLI saved lifecycle/repair/provenance regressions.
Run pinned changed-code and full affected size/architecture lint, gofmt and diff
checks. Do not rerun already-passed unrelated native platforms or fake the genuine
hosted guard. Keep original source/legacy/native-SSH/security behavior and tests.

Root owns all remote writes again after child connector mutations stalled.
Send each clean checkpoint SHA promptly for expected-head/no-force publication
to the existing source feature PR, never main/release or an unrelated branch.
Any materially new design needs its own two-GO amendment. No old execution is
relabeled current, and no source fixture implies actual native agent or release
qualification.
