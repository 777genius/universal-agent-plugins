# VS Code profile registration planner

`vscodeprofile.Plan` and `VerifyOwned` are public pure APIs for one explicit
`chat.pluginLocations[PluginRoot]` boolean in an in-memory JSONC settings
snapshot. Callers supply the actual selected settings path/profile, package ID,
complete package and projection SHA-256 digests, absolute plugin root and trusted
previous receipt. They retain filesystem, consent and activation responsibilities.
The library never discovers a profile or reads environment/files.

Source admission: VS Code **1.140.0**, commit
`07f806f999227108933c2e30515b26eecc1fda74`, from `microsoft/vscode`.
Copilot development moved from the archived extension repository into that main
repository. `ConfiguredAgentPluginDiscovery` in
`src/vs/workbench/contrib/chat/common/plugins/agentPluginServiceImpl.ts`
reads the location map, trims keys and skips `false`. Its independent upstream
`configuredAgentPluginDiscovery.test.ts` covers absolute path resolution.
`workspacePluginSettingsService.ts` demonstrates use of the native JSONC parser;
this library accepts comments and trailing commas throughout settings. These are
source/parser contracts. No native executable, extension tuple, model launch,
profile discovery, trust or activation is qualified by this library slice.

Operations:

- `Install` creates an absent selector with true, or verifies a fully receipted
  repeat without changing bytes. An unowned identical true is foreign; an
  unowned false stays disabled. Both refuse adoption. A receipted absent selector
  requires `Repair`, never a filtered install.
- `Update` verifies the previous selector/value before binding new package and
  projection digests. Settings path, profile, root and package ID cannot move.
- A native owned true-to-false change is accepted as disabled. Repeat, update
  and repair preserve the exact false bytes and return a false receipt. Persist
  that receipt even when `Changed` is false, so later absence repair stays false.
  Once false is receipted, an external change to true is drift and refuses.
- `Enable` is a distinct operation with a trusted previous receipt and
  `EnableSnapshotDigest: SnapshotDigest(reviewedDisabledBytes)`. It writes true
  only if the exact current snapshot still matches. The caller obtains consent.
- `Remove` removes only the proven key, accepting the native true-to-false
  transition; it preserves siblings and all comments, retains the location
  object and settings file, and returns no receipt. A proven absent key is a
  byte-exact no-op.
- `Repair` requires the same recorded identity and validated receipt. It restores
  only the absent exact key with the recorded boolean. Present false is never
  overwritten, altered non-booleans/duplicates/alias collisions refuse, and a
  changed package/projection cannot be repaired implicitly.

`VerifyOwned` verifies current identity and registered bytes, reporting disabled
for the accepted false transition. It refuses absence. Ownership digests bind
version, selector, exact root/value, selected settings/profile and complete
package/projection identity. Foreign formatting/values never affect ownership.
Receipts are integrity records, not authentication: do not construct a receipt
to adopt an existing selector. The caller must persist trusted receipts and bind
writes to the exact preimage through the existing nativeconfig no-follow/lock/
CAS/readback boundary. Returning planned bytes performs no write.

Bounds apply to inputs and emitted outputs: **4 MiB**, **64 container levels**,
**65,536 nodes** (containers, keys and scalar values), and **4 KiB per identity
string**. A lexical budget pass precedes hujson's recursive parse. Empty bytes
represent absent settings; other inputs must be one UTF-8 JSONC object. As an
interim admission restriction, `//` comments terminated by bare CR refuse
before AST parsing. CRLF/LF line comments, CR whitespace, CR in block comments
and escaped CR in strings remain supported; foreign bytes are never normalized.
Terminal `//` comments at EOF are accepted through a temporary hujson parsing
terminator removed from AST trivia before packing. Original input and actual
output budgets apply; no synthetic newline reaches results or digests.
Duplicates are refused in the edited shape: the root location selector and all
location keys, including equivalent escape spellings. Unknown foreign settings, duplicate
foreign keys, number lexemes and lone surrogate escapes remain opaque. Location
keys with lone surrogate escapes refuse ambiguous Go/native Unicode decoding.

Identity paths must be clean absolute POSIX or drive-rooted Windows paths;
UNC/device paths and forward/mixed Windows identity separators are outside this
slice. Foreign drive-rooted Windows keys with either or mixed separators are
normalized only for collision comparison; receipt identity and map keys remain
exact. Foreign leading-separator drive URI paths (`/C:/...` or `\C:\...`) also
participate in comparison, without broadening requested Windows identity forms.
Lexically resolvable aliases, ECMAScript trimming aliases (including
U+FEFF, excluding U+0085) and Windows case aliases of the requested root refuse
collisions. Physical symlink/case identity, remote/tilde
resolution and unsupported metadata remain caller qualification responsibilities.

No-ops return exact original bytes; refusal returns `ErrConflict`, original bytes,
`Changed:false` and no authoritative new receipt. Returned bytes/receipts do not
alias inputs. Digests and edits are deterministic. Foreign lexemes/comments are
retained in hujson's AST without a whole-settings DTO roundtrip or normalization.
Tests import only the public API and independently normalize JSONC trivia before
standard-library decoding, ending line comments at CR or LF; this checks a
native-compatible grammar contract, not a native Code parser or desktop test.
U4 must compose persisted effects, client lifecycle,
profile IO/locks/CAS/recovery and later native qualification.
