# Private authoring contract v1 (Phase 6 PR A)

Opt in only through `commands.App{PublicContract: true}` and `App.Execute` with
fresh `authoringcli.NewPluginKitRoot` or a fresh mounted `NewAuthorCommand` tree.
There is no new build/environment/public flag switch. Default v1 and the private
`vertical-slice-v1` mode keep their existing input and output contracts.

The required outer document is `{schema_version:1, command, result, data}`.
`result` is `success` or `failure`; the serializer retains literal HTML characters
and a terminal newline. JSON selection follows the final actual `--format` value,
consumes flag values according to the fresh tree, and stops at `--`.
Unknown input never supplies operation text. Leaves use `author.<leaf>`;
Skills use `author.skills.init` / `author.skills.validate`; groups use `author`
and `author.skills`. Capabilities enumerate implemented factory leaves only.

Required data includes numeric `authoring_schema_version:1`, `engine_version`
(`standard-first-slice/1`), injected `revision`, `requested:{operation,mode}`,
`effects:{attempted,committed}`, `next_actions`, and the existing report policies,
coverage, identities and deduplicated finding references. Attempted means the
service ran; it does not imply filesystem mutation. Syntax/help leave all package
policies `not_evaluated`. Exit codes are 0 success/help, 2 syntax, 1 policy/I/O/
cancellation/cleanup/output failure; existing exitx annotations retain precedence.
Cleanup retains committed state and affected paths. Output failure returns 1
without attempting another document. Human output uses the same safe projection.

Optional data: `error`, `inspection`, `help`, `commands`, explicit `root`, client/
doctor/capability details, and withheld affected-path hashes. Inspection joins
canonical components by stable IDs, with at most 128 display entries. Names and
versions have conservative 64-byte display limits; unsafe values are withheld
whole. Bare executable requirements are bounded labels; bundled paths, argv,
headers, env, manifest/MCP bodies, extensions and parser messages are never dumped.
Only `--include-root` exposes the selected absolute root; scratch is always private.

Reads default to exact CWD with no ancestor search. Minimal init derives identity
only from a simple positional name, uses deterministic neutral descriptions and
requires explicit remote URL, Node runtime or hybrid `--mcp-template` selection.
Explicit name/destination/description callers and all transaction checks remain.
Public missing-manifest findings become `missing_standard_manifest`, with remapped
references. Legacy guidance names v1 1.2.4 and says migration is unavailable.
Optional additions are compatible; removals or semantic changes bump the numeric
authoring version independently of the outer envelope. Version factories, release
roots, v1 shims, distribution and public activation belong to dependent PR B.

## Darwin acquisition profile (PR 1; native proof pending)

Writable macOS local authoring uses `packageview-local-darwin-v2` on supported
local APFS. During each Reader.Open-through-Lease.Close acquisition interval,
the source tree and ancestor bindings establishing its selected location and
containment must remain quiescent, including the gap between core decoding and
component capture. Static package content remains untrusted. The reader retains
metadata-first type checks, legacy identity/alias exclusion, contained resolution,
bounded private capture, offline operation and observed-change failure. It does
not protect acquisition from an active concurrent source or ancestor writer.
Violation can cause a forbidden open or an out-of-scope/excluded read before an
error; repeated checks do not equal Linux's inode-bound acquisition or prove an
atomic source revision. Mutation-plan rechecks, public stage validation, installer
invariants and full native macOS release gates remain required.

`report.identity.read_profile` exposes this revision, including read-only APFS
compatibility captures. Linux/Windows keep v1; ScopeID and TreeAlgorithm remain
unchanged. Capture identity hashes the profile, so Darwin capture digests change;
comparable complete tree content digests need not change. No new public schema,
flag or unsafe acknowledgment is introduced. No-cgo Darwin is unavailable.

On macOS, use a locally available APFS project and let saves, builds and checkouts
finish before running authoring commands. Keep the project and its directories
unchanged until the command returns. Detected changes fail the command; rerun
after editing finishes. Validation checks untrusted package contents, but does
not isolate reads from another process actively replacing source files. Files
requiring download are unavailable to offline validation.

See [the acquisition ADR](./adr/0006-standard-first-authoring.md#darwin-acquisition-clarification-2026-09-07-pr-1)
for the exact interval, ancestor/scratch scope, unsupported automount/network
paths and deliberate security reduction. Public activation and native release
qualification belong to PR 2; this contract is not a macOS release qualification.
