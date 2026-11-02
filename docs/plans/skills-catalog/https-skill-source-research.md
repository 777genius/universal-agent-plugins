# HTTPS Skill sources: research checkpoint

Research only, 2026-10-24. The current published `skills add` accepts GitHub raw
Skills. This checkpoint does not expose unsupported installation commands.

## Findings

- In the live first popular page, 29 of 200 listings use website sources:
  27 from open.feishu.cn and two from uizze.sh.
- Feishu serves 28 entries at
  `https://open.feishu.cn/.well-known/skills/index.json`; its preferred
  agent-skills endpoint currently returns HTML rather than an index.
- Uizze serves a valid files-list index at
  `https://uizze.sh/.well-known/agent-skills/index.json`.
- Agent Skills discovery is still a proposal, not part of the accepted format
  specification: [proposal #255](https://github.com/agentskills/agentskills/issues/255),
  [open spec PR #254](https://github.com/agentskills/agentskills/pull/254).
  Pin the accepted dialects rather than treating future schema changes as compatible.
- Vercel's implementation at
  [a80e4c9](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/src/providers/wellknown.ts)
  supports the preferred endpoint, legacy fallback, files-list entries and the
  proposed 0.2 artifact model (`skill-md` or `archive`, URL and SHA-256 digest).
- UAP [PR #473](https://github.com/777genius/universal-agent-plugins/pull/473)
  adds native Git/SSH and HTTPS archive acquisition for complete `plugin.json`
  packages. Its raw Skills integration explicitly remains GitHub-only. That PR
  is unreleased, held unmerged and under follow-up review; do not merge it as
  part of the catalog work or advertise its transports as released.

## Recommended extension

Add an HTTPS discovery source to `agentplugins skills add <origin> --skill <name>`.
The catalog is a discovery provider; the installer acquires directly from the
publisher through an independent Skill-source adapter.

1. Preserve the publisher origin/base path in the catalog DTO. Do not derive an
   install URL from the provider's listing-page slug or invent a GitHub mirror.
2. Discover the preferred index, then the legacy endpoint when unavailable or
   invalid. An explicit path scope must never silently expand to the origin's
   whole catalog. Select exactly one declared Skill name; refuse duplicates.
3. Acquire the entire declared files list for legacy entries. For the proposed
   artifact format, acquire the declared single file or full archive and verify
   its digest. Never crawl HTML or guess resource files from Markdown links.
4. Feed the sealed raw tree through existing Skill validation, normalization,
   security assessment, shared store and agent projection. Reuse reviewed bounded
   archive/download services from #473 when available; do not reimplement them.
5. Give HTTPS sources their own saved source variant. Keep existing Git pins and
   source keys unchanged. Lock origin, selected name, resolved index/artifact
   coordinates, content/file-set identity, raw tree digest, normalized digest and
   normalizer version. A mutable website does not provide an immutable Git ref.
6. Update explicitly re-resolves the name and creates a new pin. Restore/repair
   uses the recorded bytes/digests, cached or exported snapshots, and refuses
   changed or missing historical content. Do not promise cold restoration when
   neither the publisher nor a retained snapshot can supply the old bytes.
7. The discovery proxy must not become an arbitrary URL downloader. Publisher
   acquisition belongs to the CLI and retains its network, extraction,
   cancellation and path-safety policies.

Acceptance focuses on both live publisher dialects, exact-name selection,
complete resources, digest/path rejection, and saved add/update/exact restore.
Enable website install commands only after the released CLI supports this mode.
Until then show a truthful unsupported-source state, without claiming that a
GitHub repository is intrinsically required by the Skill format.
