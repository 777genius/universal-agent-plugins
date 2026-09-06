# E2E and competitive launch plan

This document records the current launch closure for the Universal Agent
Plugins CLI. Implementation details belong in the workflows and test suites;
this page keeps the user-visible evidence and honest limits in one place.

## Current closure

| Area | Evidence | Result |
| --- | --- | --- |
| CLI source | release `aa9e03e4e6bc9eb044aedde8be1d1ff4ea514a2c`; post-release main checkpoint `a281a54c77f618903868008f838858aa3acbf0ca` | The public launch claim is bound to the released commit; later standard-first authoring work on main is not represented as already released |
| Security scanner | [`lintai v0.1.3`](https://github.com/777genius/lintai/releases/tag/v0.1.3), run [`33974725553`](https://github.com/777genius/lintai/actions/runs/33974725553) | Agent Plugins 1.0 scan contract released for every CLI-supported platform; noisy path and remote-instruction matches narrowed |
| Native release | [`agentplugins-v0.1.51`](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.51), run [`34001219785`](https://github.com/777genius/universal-agent-plugins/actions/runs/34001219785) | Six platform builds, checksums, release manifest, and native runtime proofs passed |
| npm release | [npm workflow `34001764906`](https://github.com/777genius/universal-agent-plugins/actions/runs/34001764906) | Trusted Publisher, provenance, signatures, and public lifecycle verification passed |
| Public package | `universal-agent-plugins@0.1.51` | `latest` points to `0.1.51`; registry integrity matches the staged tarball |
| Lifecycle | Existing isolated Linux test host, fresh disposable `/tmp` sandbox | Public `0.1.51` completed `add`, `info`, `update`, and `remove` for explicit Codex, Cursor, and Kiro targets against the reviewed upstream Context7 distribution; final `doctor` reported zero installations and zero open operations |
| Public search | `npx --yes universal-agent-plugins@0.1.51 search contex7 --details` in a fresh disposable home | Typo recovery returned the reviewed upstream Context7 primary and the target-free `npx universal-agent-plugins add context7` command; alternate-source provenance remains available only in detailed output |
| Public warning canary | `npx --yes universal-agent-plugins@0.1.49 add /path/to/disposable-fixture --target codex --dry-run` | Four non-blocking findings returned exit 0 without a confirmation prompt; the default showed three notes plus one hidden count, while `--security-details` showed all four; no client files were changed |
| Public signed-index canary | Public `0.1.51` Context7 lifecycle on Directory sequence 37 | The short alias resolved to `upstash/context7@769c6cd22c3d95462d1f55d789e9532cabefa5a9//plugins/agent-plugins/context7`; the signed identity was preserved through add, info, update, and remove |
| Directory | [sequence 37](https://777genius.github.io/universal-agent-plugins-registry/registry/schemas/1/latest.json), run [`34014230574`](https://github.com/777genius/universal-agent-plugins-registry/actions/runs/34014230574) | Exact production observation passed for 28 products and 36 distributions; Context7 is official-upstream first for all 10 supported clients and retains the reviewed bridge as fallback; snapshot digest `sha256:f03cbf0e4e12e0f01d59497765ef547e5ed83df9cc11223bf1c9a64d3d06ba7f` |
| Discovery | [sequence 40](https://777genius.github.io/universal-agent-plugins-registry/discovery/latest.json), run [`34014943629`](https://github.com/777genius/universal-agent-plugins-registry/actions/runs/34014943629), 3,024 records | Exact production verification and an independent portable signature check passed; snapshot digest `sha256:9eba42619f65bd505af0914d5f67c72d0e9ef65a6416306761024d2d02c38605` |
| Security Index | [sequence 8](https://777genius.github.io/universal-agent-plugins-registry/security/latest.json), run [`34017085675`](https://github.com/777genius/universal-agent-plugins-registry/actions/runs/34017085675), 2,751 subjects | 2,746 exact package revisions were assessed with LintAI 0.1.3 and policy v2; 5 checks were unavailable, 2,365 had no blocking finding, 381 had warnings, and none were classified as blocking; snapshot digest `sha256:7f24057d9e37d9905cc2371621e25dc201000347a95594ec7001d71ea24242a7` |
| Feed freshness | run [`34017918030`](https://github.com/777genius/universal-agent-plugins-registry/actions/runs/34017918030) | Directory, Discovery, and Security signatures were reacquired from public Pages and all retained more than 48 hours of validity |
| Product site | main `a281a54c77f618903868008f838858aa3acbf0ca`, [Pages run `34017177213`](https://github.com/777genius/universal-agent-plugins/actions/runs/34017177213) | Production Pages completed successfully; the public landing and Context7 page returned complete HTML, and the landing exposes the target-free `npx universal-agent-plugins add context7` command |
| Route payload | product PR [`#152`](https://github.com/777genius/universal-agent-plugins/pull/152) | Gzip payload is 58,511 bytes for a plugin page, 55,448 bytes for `/download/`, and 84,185 bytes for the catalog home page |
| Legacy registry landing | registry PRs [`#272`](https://github.com/777genius/universal-agent-plugins-registry/pull/272) and [`#273`](https://github.com/777genius/universal-agent-plugins-registry/pull/273) | A headless public-browser proof reached the product catalog with no failed requests; signed machine feeds and deep assets remain at their stable URLs |

The npm package is published by GitHub Actions with npm Trusted Publisher
provenance. No long-lived npm token is required by the release workflow.

## Security behavior before installation

The CLI evaluates an acquired package before changing any managed client files:

1. It reuses a signed Security Index assessment only when the package tree,
   `plugin.json`, LintAI version, policy version, and policy digest all match.
2. Otherwise it runs the pinned LintAI release in the isolated staging tree.
3. The result is cached by that same exact identity. A package, scanner, or
   policy change invalidates the cache.
4. Non-blocking findings never add a confirmation prompt. The default output
   shows at most three install-relevant notes and summarizes hidden details;
   `--security-details` is available for a full audit view. Repository-only
   maintenance notes such as GitHub Actions pinning stay out of the normal
   installation path.
5. Blocking findings stop non-interactive installation unless
   `--accept-security-risk` is explicit; an interactive terminal asks for
   confirmation and defaults to no.

The signed Security Index is a separate, optional feed. An unavailable,
expired, stale, malformed, or mismatched feed cannot bypass the local scan. A
successful automated check means no configured blocking pattern was found; it
is not a guarantee that a package is safe.

## Reproduce the public checks

```bash
npm view universal-agent-plugins version dist-tags --json
npx universal-agent-plugins add context7
```

The first command confirms the public package and `latest` tag. The second is
the normal interactive path: the CLI detects compatible clients and lets the
user choose one or several. Target-specific automation can use
`--target codex,cursor,kiro`.

The exact CI evidence is available from the repositories:

- [native release run 34001219785](https://github.com/777genius/universal-agent-plugins/actions/runs/34001219785)
- [npm publish run 34001764906](https://github.com/777genius/universal-agent-plugins/actions/runs/34001764906)
- [LintAI release run 33974725553](https://github.com/777genius/lintai/actions/runs/33974725553)
- [Directory sequence 37](https://777genius.github.io/universal-agent-plugins-registry/registry/schemas/1/latest.json)
- [Discovery sequence 40](https://777genius.github.io/universal-agent-plugins-registry/discovery/latest.json)
- [Security sequence 8](https://777genius.github.io/universal-agent-plugins-registry/security/latest.json)
- [signed-feed freshness run 34017918030](https://github.com/777genius/universal-agent-plugins-registry/actions/runs/34017918030)
- [public Directory](https://777genius.github.io/universal-agent-plugins-registry/)
- [public product site with the verified feed mirror](https://777genius.github.io/universal-agent-plugins/)

## Scope and honest limits

- Agent Plugins 1.0 packages are the input format; each client still has its
  own native projection, activation, and OAuth rules.
- The release workflow proves the CLI lifecycle and native binary delivery in
  isolated CI environments. It does not claim that every package works at
  runtime in every client.
- Discovery metadata supports search. It is not an endorsement or a substitute
  for package-specific runtime validation.
- The language selector stays hidden until localized landing routes are
  published. Exposing the existing locale data before those routes exist would
  create broken navigation; this is a deliberate launch-scope decision, not a
  missing client capability.
- LintAI reports known static patterns under a versioned policy. It does not
  execute package code and does not certify a package as safe.
- No real user project, account, OAuth consent, or private service was used by
  this security extension. Release and signed-index publication ran in
  GitHub-hosted CI; the public CLI lifecycle used only a fresh disposable
  sandbox on the existing isolated Linux test host. No VM, LXC instance, or
  snapshot was created.

## Ongoing release contract

1. Keep LintAI, policy, signed assessment, and CLI evaluator identities exact.
2. Keep six-platform release and public npm lifecycle checks green for every
   versioned release.
3. Publish Security and Discovery as separately signed feeds; preserve the
   last-known-good snapshot whenever a refresh is incomplete.
4. Add client-specific runtime evidence only in disposable test surfaces and
   keep every claim package- and client-specific.
5. Track Agent Plugins 1.1 separately until its specification is stable.
