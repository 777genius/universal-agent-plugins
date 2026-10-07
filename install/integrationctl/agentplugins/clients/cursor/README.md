# Cursor adapter: selected editor package preparation

This adapter retains the manual skill/MCP package workflow. It prepares an
editor compatibility manifest at `.cursor-plugin/plugin.json`, selected skills,
and one package-local `mcp.json`. Native plugin discovery, agent CLI plugin or
skill loading, and genuine native Stop are unqualified. Plan results say
`manual_activation_required` with `prepared` activation and components. The existing `native` package
mode describes its manifest/layout, independently of discovery or readiness.
Ordinary Engine installs and direct adapter activation remain manual with
package validation. The shared activation provider retains its separate legacy
operator-attestation branch: an explicit attestation can report active and
installation_verified without adapter dispatch. That operator assertion does
not qualify native discovery, genuine Stop, or automatic activation.

The single MCP strategy in this contract is **plugin package projection**.
It never writes user or project `mcp.json`, enables a disabled server, or
registers a second copy. CLI user-config MCP lifecycle is the separate conditional
UC4 slice. It cannot be inferred from this editor package contract.

Detection distinguishes `cursor_editor` (`cursor`) from `cursor_agent`
(`cursor-agent`). These are binary-presence observations. Only editor launcher
or desktop presence selects editor package capability; leftover `.cursor` and
agent-only presence do not. The adapter never probes an ambiguous executable
named `agent`, uses the editor's version as a CLI version, or supplies guessed
CLI profile environment variables. `ExecutablePath` remains the editor path.
The current public installer accepts a client executable without a separate
surface selector; callers must not interpret that input as agent qualification.

`ResolveProfileRoot` requires a clean absolute directory, rejects file/dangling
link ancestors, resolves existing symlink aliases, and retains missing parents
without creating them. `TargetRoot` uses that physical root for the existing
`plugins/local` preparation layout, even during public identity reservation.
Subsequent writes use the shared no-follow path policy and transaction engine.
The retained layout is a preparation destination, not evidence Cursor scans it.

The exact pinned editor source uses `pathService.userHome()/.cursor/hooks.json`.
Supplied Cursor 3.22.12 Linux workspaceOpen evidence observed execution in a
fresh TEST HOME's `.cursor`, independently of `--user-data-dir`. Thus a custom
editor user-data directory is not proof of a custom hooks/profile directory.
An explicit installer root grants filesystem authority; it does not establish a
native client redirect. No shared editor/CLI physical profile is claimed.

Public `installer.New`, `ReserveIdentity`, `Prepare`/`Apply`, and `Inspect` tests
exercise two fresh TEST profile namespaces, alias identity with missing parents,
repeat without rematerialization, update, missing-package repair, and removal
under changed ambient HOME/USERPROFILE/cwd. Foreign disabled user MCP and hooks
remain byte-identical; the sibling namespace and prepared manifest survive.
No client, helper, MCP process, model or hook is executed by these tests.

`ValidateBindingProfile` exposes the existing optional client contract for an
exact recorded artifact locator and, if present, `NativeProfileRoot`.
The current shared facade does not dispatch this interface or persist a Cursor
physical directory identity. Its `ReserveIdentity` can return an existing
binding without validating a supplied profile; update/repair still require
explicit roots. UC-P must bind those operations to recorded profile authority,
expose that authority in a public roots/facts view, and refuse mismatches or
replaced directories before effects. This adapter adds no parallel state store.
The existing registry inspector's clear finding covers absence of an additional
managed executable registration only; the prepared-directory walk still owns
namespace collisions. It is not an exhaustive native registry observation.

Source evidence: macOS editor 3.22.7 bundle SHA256
`a9157bf9054d3f27a2a6910c855e028bd5e38896fa74405ca2f766abff590cb8`;
Linux editor 3.22.12 bundle SHA256
`8009c99a930bc4ccef0042e8894a92ba933ab950f8a86fe870a82b2113d70dd6`;
Linux deb SHA256
`8b9198f7dde79f7e27cc83dabf3c66a93ca5303b6919719100c82f29d9498a65`.
These source/attributed startup observations prove no genuine Stop, native MCP
request, restart discovery, Windows native I/O, or release availability.
S4 needs the supplied official artifact to be available for an isolated TEST
native discovery run. Independent code review precedes public delivery.
