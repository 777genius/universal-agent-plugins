# Pinned Claude project discovery spike

> **Historical snapshot.** The observations and qualifications below belong to
> their recorded source. The [original repository directory](https://github.com/777genius/universal-agent-plugins/tree/3b93148c356fc5db6fe101d81d943160a5176b6d/docs/evidence/project-scope-native/claude-2.1.263-linux-amd64-spike-2026-10-24) preserves the
> files present before raw-output cleanup. Adjacent metadata and source remain;
> original manifests describe that snapshot, not the current directory inventory.
> Retrieve removed raw files there; see the [project evidence index](../../PROJECT_INSTALLATION.md).

This Linux amd64 spike ran the official Claude Code 2.1.263 binary in disposable
user/A/B directories on 2026-10-24. It establishes the untrusted native listing
schema, nested-cwd discovery boundary, enablement-id behavior, and native
skills-directory uninstall refusal. It did not run the UAP prepared lifecycle,
grant trust, log in, send a model prompt, or initialize an MCP server.

`observations.json` preserves selected command records, exact stdout/stderr,
explicit fixture-only environment, cwd, exit and timing. `provenance.json` records
official package identity and measured digests. The archive SHA-512 matched
`package-metadata.json` before extraction. Paths name disposable fixture authority;
no current user profile or credential values were supplied.

The untrusted native row is `(suppressed)@skills-dir`, version `unknown`, scope
`project`, enabled false, empty path and a nonempty explanatory notes array. It
supplies no installed package identity. With a same-name user copy, the user row
and suppression row coexist. Trusted winner selection remains unobserved; current
official rules describe user skills-directory precedence separately.

Running the list command from A/nested omitted the parent whole plugin. Project
enablement toggled the same user-origin id inside A without changing B. Native
uninstall refused this origin and instructed directory removal. Removing only
the disposable A directory removed A's suppression row; B remained suppressed.

For reproduction in a new disposable scratch directory, acquire the pinned
archive from the URL in provenance, verify its metadata integrity, safely extract
its regular `package/claude` file into `bin/package/claude`, and run `spike.py`
then `followup.py`. The scripts deliberately use a separate fixture-user HOME,
never a project's directory as a synthetic native user profile. They write only
their explicit fixture trees. No native trust is granted. A new reproduction is
a new observation and must keep its own hashes and outputs.

The genuine source-only UAP harness is now named
`TestAgentpluginsClaudeNativeProjectPreparedLifecycle`. Its independent evidence
contract distinguishes passed owned-file lifecycle from blocked native skill
discovery and unevaluated MCP/model operation. A hosted run is still required;
these spike records cannot substitute for that qualification. The historic
[macOS project failure](../claude-2.1.263-darwin-arm64-40011624952/README.md)
remains preserved; its original raw output was absent, so this spike does not
retroactively assign its exact parsing cause.
