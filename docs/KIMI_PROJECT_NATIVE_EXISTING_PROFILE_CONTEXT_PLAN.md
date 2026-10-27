# Kimi installed observers: reuse the actual existing profile without provisioning

## Current source and measured boundary

Published PR524 head705fa429eaad9403113f4fcddb57d3a4f99c96ba has
treee8d9fa5d0a6adc90aee91f7032a1cddec1b3af3e, identical to reviewed local
62ea70f4b2e3c7589cab2fbae774345fcf2d6ca3. Preserve its genuine held-marker
handoff and every prior external schema/Windows/context and Python diagnostic
change. This follow-up addresses the distinct installed-component context
failure; it does not claim the single-tool run has passed its later gates.

At previous source55f5d848fb61899f2c84bd7cccc7592549718b5f/tree2f20a8e8,
runtime40456546065/job122031436251 completes observe-empty and all four public
add operations, then observe-installed exits1/error=false/deadline=false with
stdout0. Its exact401-byte stderr SHA256 is
6ef2c6c28e6dce941d029cc2dd3d6a233b17594c75a78dd9b3e15d6174107741.
The finite diagnostic says installed/context/enter/os, observer_entered=false,
service_recorded=false and all cleanup values null. No later API/service ran.

Closed native12713940918 outer SHA256
5a59f0a45ca0139e9cd3ad26e8840adbc9aee4b2e160f3bf863e9c0510ffbef4
and frontend12714100073 outer SHA256
2d7907fd2d633c5fbf8207f90b378a1329da9ac548205e52c558f91ef64af859
have31/25 bounded canonical unique ordinary members, verified CRCs and exact
committed source/tree; all qualification flags remain false. Separate read-only
research40456546030 succeeded with complete source-bound capture, not lifecycle
or MCP qualification.

## Source-derived cause and original profile authority

At exactly the context stage, both installed Python entrypoints call
shared.legacy.runtime_environment(home, root/tmp). That is a fresh-profile
factory: native.profile_environment unconditionally mkdir's home/config,
home/cache, home/state, home/data and home/npm-cache, then writes fresh empty
npm/git configuration files. Repeating it after observe-empty necessarily
encounters its existing first directory. A source-only inert temp-directory
reproduction of two calls produces FileExistsError before any service. The raw
capture establishes only context/OSError, not the private exception text or
filesystem body; the narrower FileExistsError diagnosis comes from this actual
source path and deterministic inert reproduction.

The Go-owned original fixture already creates ordinary home, tmp and
root/xdg-config, root/xdg-cache, root/xdg-state and root/xdg-data directories.
Its closed environment supplies those exact original XDG paths, while the
fresh-profile factory chooses different home/config-style paths. Reusing that
factory with exist_ok, deleting directories or truncating existing profile
files would neither preserve the original environment nor prove ownership.
Installed observation must consume the actual already-created Go-owned profile
without creating a replacement or adopting a native rewrite.

## Finite nonmutating environment handoff after two plan GOs

Scope is scripts/project_native_collectors/kimi.py and kimi_attempt.py, their
two existing inert test modules and this plan. No shared research/profile
factory, supervisor, production adapter or Go fixture change.

Add one Kimi-only private helper used by both installed entrypoints. It accepts
the already request-validated root/home and a closed mapping of exactly12 held
environment values. Each caller reads only these named os.environ entries;
never copy os.environ or accept arbitrary extra keys. Validate exact string
types, keys and equality against the existing Go fixture authority:

| Held key | Required value from the existing fixture |
| --- | --- |
| HOME, USERPROFILE | exact request home, which is root/home |
| TMPDIR, TEMP, TMP | root/tmp |
| XDG_CONFIG_HOME | root/xdg-config |
| XDG_CACHE_HOME | root/xdg-cache |
| XDG_STATE_HOME | root/xdg-state |
| XDG_DATA_HOME | root/xdg-data |
| GIT_CONFIG_NOSYSTEM | exact held value1 |
| GIT_CONFIG_GLOBAL | exact original os.devnull path |
| GIT_TERMINAL_PROMPT | exact held value0 |

Return those validated held strings, rather than synthesizing a different
profile. Retain the existing observer's fixed PATH=/usr/bin:/bin, LANG=C.UTF-8,
NO_COLOR=1, TERM=dumb and KIMI_CODE_NO_AUTO_UPDATE=1 controls. No ambient PATH,
credentials/tokens/provider/proxy/loader settings, host eligibility markers,
unrelated application state or npm configuration enters the vendor child.
The original Go command environment remains unchanged; the private provider's
already-authored HOME/.kimi-code config and recorded original HOME stay exact.

Before returning an environment, check root/home/tmp and each four existing XDG
directories through held descriptor-relative O_DIRECTORY/O_NOFOLLOW traversal.
Require canonical exact paths, current original UID/GID, ordinary private
directories and stable held/linked identities with checked closes. Refuse a
missing directory, alias, wrong owner/type/mode, replaced ancestor or any
inconsistent held value. No mkdir, writes, unlink, chmod, repair, config-body
read, alias resolution or fallback is allowed. Validate only the finite existing
directory set; do not traverse arbitrary profile contents or user credentials.

Replace only the two calls to the fresh-profile factory at the existing
context stage with this helper and the explicit held mapping. The general
read-only research lane still uses its unchanged fresh-profile preparation.
Preserve both independent real hosted/platform guards, every existing API/
provider/probe/receipt check, original budgets, diagnostic/quarantine behavior,
permission/trust policy and actual cleanup requirements. No later failure is
accepted or relabelled by this correction.

## Meaningful inert contracts and genuine follow-up

Use real owned temporary Go-layout directories and the actual new helper,
without setting any hosted marker or invoking a native guard/client. Compare
the complete finite output to independently expected held values; inject
unrelated ambient private/proxy/loader canaries and require their absence.
Call the helper repeatedly with existing original profile directories and
private sentinel/config files; verify exact full held metadata and bytes stay
unchanged and no home/config-style replacement directories or npm/git files
are created. Preserve an existing nonempty original git/npm/config sentinel;
the helper must not open, truncate, repair or adopt it.

Negatives cover each wrong/missing/nonstring/extra held key, foreign home/tmp/
XDG paths, absent/regular-file/symlink directory, aliased ancestor, changed mode,
and mocked finite wrong UID/GID plus deterministic held/path replacement at a
validation boundary. Every failure returns no partial environment, performs no
write. Invoke the actual helper with both installed entrypoints' finite input
shapes and review both sole context-stage call sites to ensure neither calls
the old factory or launches a service before validation. Do not bypass/mock a
positive native hosted guard in order to exercise a local entrypoint. The
actual negative hosted-guard tests remain unmodified and active.

Run the existing complete Kimi collector/attempt/shared-service Python suite,
new ownership/nonmutation controls, AST and diff checks. No Go or vendor/model
execution is needed. Independent code reviews precede root-only publication to
existing PR524. No pin, public mutation, auth workaround, release/new PR or VS
Code work. Require fresh genuine source-bound runs to pass unchanged context
admission and expose the next actual boundary; full lifecycle, restart/removal
and MCP qualifications still require their complete native evidence.

## Completed local validation

Both independent plan reviews approved this finite existing-profile handoff.
Code review required removal of an added Path.resolve call from the new helper;
the final helper uses lexical canonical checks and held no-follow traversal.
The original earlier request guard remains unchanged. Tests explicitly forbid
Path.resolve while exercising both an unchanged original profile and a matching
alias-shaped environment, so an alias cannot be silently resolved into authority.

All 54 collector, single-tool and shared-supervisor tests passed on the final
bytes (16.459s), including real owned temporary directory nonmutation, exact
held environment controls, missing/wrong keys, type/mode/owner/group refusal,
symlink/ancestor rejection, deterministic held-directory replacement, complete
descriptor closure and failed-close refusal. All four Python ASTs and diff
whitespace checks passed. These checks ran inert source contracts and the
existing private supervisor child; they did not set eligible hosted markers or
execute a native client, installer or model. No Go was needed. Genuine
source-bound hosted context/API/cleanup and full lifecycle proof remain required.
