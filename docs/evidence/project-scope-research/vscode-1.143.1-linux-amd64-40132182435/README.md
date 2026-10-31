# VS Code 1.143.1: genuine shipped CLI help/version

> **Historical snapshot.** The observations and qualifications below belong to
> their recorded source. The [original repository directory](https://github.com/777genius/universal-agent-plugins/tree/3b93148c356fc5db6fe101d81d943160a5176b6d/docs/evidence/project-scope-research/vscode-1.143.1-linux-amd64-40132182435) preserves the
> files present before raw-output cleanup. Adjacent metadata and source remain;
> original manifests describe that snapshot, not the current directory inventory.
> Retrieve removed raw files there; see the [project evidence index](../../PROJECT_INSTALLATION.md).

Measured at reviewed source `0a34182aa7f424a52a9ff9a49177d88dbf2fb711` in
[run 40132182435](https://github.com/777genius/universal-agent-plugins/actions/runs/40132182435),
[job 120986058838](https://github.com/777genius/universal-agent-plugins/actions/runs/40132182435/job/120986058838),
on actual GitHub-hosted Ubuntu 24.04. The workflow completed successfully.
Artifact 12576470352, `vscode-1.143.1-precode-publisher-linux-amd64`, had a
5,805-byte outer ZIP, SHA256
`5f9f68cf934db20059327f77788ee4a33baaee65b47a82ab3f6478d1667fe651`.
Its sole member is the admitted 24,354-byte `research.zip`, SHA256
`9bfb6dca7280a56b11d8066cadbe310aa22d49ad481a0171e5aca24738937698`.
All seven flat raw text members are preserved byte-for-byte in the original
repository snapshot. The ZIPs were not committed; `provenance.json` records all
text lengths and hashes.

## What was verified

The implementing worker and root independently verified both archive byte
counts/hashes and CRC, all seven member bytes and all six stdout/stderr stream
size/hash pairs. Both
native commands exited zero with empty stderr. The exact version stdout is
1.143.1, publisher commit `e646868227c3e3a9e3f24277c8c7220be42c0be7`, x64.
Help was 8,469 bytes and version 53 bytes. The successful same-process proofs
record real/effective UID/GID 1001, no supplementary groups, only loopback, a
network namespace distinct from the parent, zero capability sets and
no-new-privileges. Each probe had a distinct fresh home/cwd/profile. The shipped
ELF ran its shipped cli.js with ELECTRON_RUN_AS_NODE=1; no editor/workbench or
extension host was launched. Cleanup completed in 0.25 seconds with empty streams.

The collector recorded complete publisher archive integrity before parsing:
350,844,741 bytes with the reviewed publisher SHA256. It consumed 4,262 TAR
members and 1,034,475,520 expanded bytes; regular payload was 1,031,068,984 bytes.
No symbolic links were present/omitted. Required ELF/CLI/product file hashes and
identities are in the original summary, checked by the collector before/after
commands. Interpretation did not reacquire or rerun those deleted native assets.

## Observed command boundary and conclusion

The native help explicitly describes `--add-mcp` as a **user-profile** writer.
It exposes process diagnostics, extension/profile controls and chat invocation;
it does not provide a read-only workspace MCP/skill inventory command. None of
those additional commands was run. A project adapter cannot be based on this
user writer or on help/version alone.

The implemented help/version pre-code observation gate is complete. Both independent
interpretations are GO for a new concrete workspace observation/design plan only. Original
`native_project_qualified:false` is unchanged. Workspace skill/MCP attribution,
UAP ownership/maintenance, effective Local harness behavior, trust, extension
host/UI, model/authentication and restart remain unobserved.

Primary pinned workbench/API source may guide the next plan, but only a genuinely
shipped-host observation can populate those additional evidence stages. No local
vendor process, installer, extension, MCP server or account operation was executed
while downloading or interpreting this artifact.
