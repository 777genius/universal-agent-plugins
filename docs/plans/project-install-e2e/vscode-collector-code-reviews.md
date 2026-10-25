# VS Code publisher collector: independent code reviews

2026-10-24. These are receipts of actual independent reviews. They approve only
the pinned help/version research collector; no workspace adapter, native project
discovery, MCP, extension-host, trust or model behavior is qualified.

## Initial checkpoint and findings

The initial local frozen source was
`619b07347aedd8707225027eef0c3b49d287e4c3`. Root initially gave GO after
independently tracing archive integrity before parsing, bounded fixed-header
extraction, omitted symlinks, descriptor containment, namespace/privilege proof,
finite commands, output/deadline bounds, anchored cleanup and the exact-head
workflow; 11 inert tests passed in 0.168s. That initial GO was superseded.

Independent reviewer `apm_vercel_scope` gave **NO-GO**: failure of the raw
artifact cap still allowed the workflow's `always()` upload of that raw
directory. It also required rejecting GID 0 and checking/recording matching
real/effective original UID/GID. The reviewer independently passed 11 collector
and six capture tests; no vendor executable was acquired or launched.

## Revised frozen checkpoint

Local `d6014d88aabffe7589ede046fbcd006f8e620b65` fixes both findings. It stages
raw files through a bounded ZIP_STORED envelope in a separate upload directory;
invalid source trees yield only a bounded false-qualification failure ZIP,
retaining the original error. The workflow uploads only the exact independently
admitted ordinary, single-link, at-most-32MiB ZIP. Source member count, actual
bytes and stable ordinary identities are checked before publication. The parent
and same-process proof now reject root group or real/effective identity mismatch.

Root independently reviewed that narrow delta: **GO**, dedicated 13 inert tests
passed in 1.136s. `apm_vercel_scope` independently reviewed the same delta:
**GO**, independently reran the 13 collector plus six capture tests, Python
compilation and diff checks. Its prior extraction/CRC/no-follow/anchored cleanup/
real guard review remains valid. Both reviewers explicitly ran no vendor code.

Only after both final GOs did root atomically publish the two-commit chain at
remote `4cba9df5cab26f12fafa4c0e68a119953485e652`, identical final tree
`13532aa65ea0c5af431331896d8437a82ae1e3d2`; the initial checkpoint maps to
remote `c4e4e348`. No intermediate unreviewed workflow was admitted. This record
permits the genuine hosted collection and subsequent interpretation; a further
reviewed workspace-evidence/adapter plan is still required.
