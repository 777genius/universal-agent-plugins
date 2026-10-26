# Qoder 1.1.78: genuine bounded inventory refusal

> **Historical snapshot.** The observations and qualifications below belong to
> their recorded source. The [original repository directory](https://github.com/777genius/universal-agent-plugins/tree/3b93148c356fc5db6fe101d81d943160a5176b6d/docs/evidence/project-scope-research/qoder-1.1.78-linux-amd64-40130660701) preserves the
> files present before raw-output cleanup. Adjacent metadata and source remain;
> original manifests describe that snapshot, not the current directory inventory.
> Retrieve removed raw files there; see the [project evidence index](../../PROJECT_INSTALLATION.md).

The first populated scope collector ran at exact source
`e29237fe546d8aebde98adaae59d346142143d2d` on genuine hosted Ubuntu 24.04:
[run 40130660701](https://github.com/777genius/universal-agent-plugins/actions/runs/40130660701),
[job 120981483006](https://github.com/777genius/universal-agent-plugins/actions/runs/40130660701/job/120981483006).
It failed its observation step and successfully uploaded artifact **12575528479**.
The 6,264-byte ZIP SHA256 is
`acf3d04abec66d0fde470e76fb501efd7bf20621c86c72ee15cfeb5719ee9bb8`.
All 15 flat text members are preserved byte-for-byte in the original repository
snapshot; the ZIP is not committed.
`provenance.json` records every member length/hash. All four native and three
auxiliary stdout/stderr pairs were verified against their recorded hashes.

## What actually ran

The parent measured exact git HEAD, acquired the pinned official 1.1.78 archive
and verified its 182,982,112-byte executable. The child proved original real and
effective UID/GID 1001, empty supplementary groups, a network namespace distinct
from the parent, only loopback, all capability sets zero and no-new-privileges.
The four version/install-help/list-help/uninstall-help commands each exited zero
with empty stderr. These are fresh actual help observations, not installs.

The first stage inventory then refused `home/.qoder/logs/latest`: a native-created
symlink, mode 0777 and 42-byte target length. The collector intentionally did not
read its target bytes or follow it. No list or install command was reached, so
this artifact contains no populated registry schema, source attribution or
installation outcome. The complete source fixture snapshots and six file pins
were retained; all eight qualification fields remain false and `success:false`
is unchanged. Independent bounded cleanup completed successfully.

## Consequence

The collector's strict refusal worked as designed. A concrete reviewed amendment
may record bounded **link metadata only** for this exact native log alias while
keeping every source fixture and all other links/specials strict. This failure
must not be normalized to successful project discovery, and metadata admission
must never read a target path, relax source pinning, change runtime authority,
add native commands or promote qualification. No client or acquisition was rerun
locally during artifact interpretation.
