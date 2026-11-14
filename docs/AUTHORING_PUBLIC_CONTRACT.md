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
