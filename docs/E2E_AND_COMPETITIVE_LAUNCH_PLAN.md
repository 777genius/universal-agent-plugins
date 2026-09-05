# E2E and competitive launch plan

This document records the current launch closure for the Universal Agent
Plugins CLI. Implementation details belong in the workflows and test suites;
this page keeps the user-visible evidence and honest limits in one place.

## Current closure

| Area | Evidence | Result |
| --- | --- | --- |
| CLI source | `9b1bf99ed5cc5b5bb02a825ed90d5ef07ffa8464` | Security checks, grouped discovery search, typo recovery, URL-backed filters, and route-scoped site data are merged |
| Security scanner | [`lintai v0.1.3`](https://github.com/777genius/lintai/releases/tag/v0.1.3), run [`33974725553`](https://github.com/777genius/lintai/actions/runs/33974725553) | Agent Plugins 1.0 scan contract released for every CLI-supported platform; noisy path and remote-instruction matches narrowed |
| Native release | [`agentplugins-v0.1.50`](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.50), run [`33993393392`](https://github.com/777genius/universal-agent-plugins/actions/runs/33993393392) | Six platform builds and native runtime proofs passed |
| npm release | [npm workflow `33994016590`](https://github.com/777genius/universal-agent-plugins/actions/runs/33994016590), attempt 2 | Trusted Publisher, provenance, signatures, and public lifecycle verification passed after npm finished processing the published version |
| Public package | `universal-agent-plugins@0.1.50` | `latest` points to `0.1.50`; registry integrity matches the staged tarball |
| Lifecycle | Fresh GitHub-hosted sandbox in release CI | `add`, `info`, `update`, and `remove` passed for Codex, Cursor, and Kiro; OpenCode repair passed |
| Public search | `npx --yes universal-agent-plugins@0.1.50 search contex7` in a fresh disposable home | Typo recovery returned the reviewed Context7 primary and the target-free `npx universal-agent-plugins add context7` command; `--details` retains alternate-source visibility |
| Public warning canary | `npx --yes universal-agent-plugins@0.1.49 add /path/to/disposable-fixture --target codex --dry-run` | Four non-blocking findings returned exit 0 without a confirmation prompt; the default showed three notes plus one hidden count, while `--security-details` showed all four; no client files were changed |
| Public signed-index canary | `npx --yes universal-agent-plugins@0.1.49 add 'discovery:upstash/context7//plugins/agent-plugins/context7' --target codex --dry-run` | Exact public Context7 revision passed in a fresh disposable home after sequence 4 reached the product mirror; no LintAI binary was downloaded and no client files were changed |
| Directory | [sequence 35](https://777genius.github.io/universal-agent-plugins-registry/registry/schemas/1/latest.json), run [`33993993072`](https://github.com/777genius/universal-agent-plugins-registry/actions/runs/33993993072) | Exact production observation passed for 28 products and 35 distributions; seven withdrawn revisions remain explicit revocations |
| Discovery | [sequence 38](https://777genius.github.io/universal-agent-plugins-registry/discovery/latest.json), run [`33966346960`](https://github.com/777genius/universal-agent-plugins-registry/actions/runs/33966346960), 3,024 records | Exact production verification passed; snapshot digest `sha256:9d29e278406b2f10cfbf632ab538bbc675aaeb7f38d0b3bd31dc64409fea96c7` |
| Security Index | [sequence 4](https://777genius.github.io/universal-agent-plugins-registry/security/latest.json), run [`33975646164`](https://github.com/777genius/universal-agent-plugins-registry/actions/runs/33975646164), 2,752 subjects | 2,747 exact package revisions assessed with LintAI 0.1.3 and policy v2; 5 acquisition or scan checks unavailable; 2,366 have no blocking finding, 381 have warnings, and none are classified as blocking; snapshot digest `sha256:b41e8cd7ae9aaf5f630fcb5bda9bec7f7cb815abc0b758a6f0f40fc9b8c8161b` |
| Product site | main `9b1bf99ed5cc5b5bb02a825ed90d5ef07ffa8464`, [Pages run `33993369525`](https://github.com/777genius/universal-agent-plugins/actions/runs/33993369525) | Signed feeds load, one primary package is shown per product, alternate distributions stay under `Other sources`, typo search and shareable filters work, and client pages copy a valid command |
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

- [native release run 33993393392](https://github.com/777genius/universal-agent-plugins/actions/runs/33993393392)
- [npm publish run 33994016590](https://github.com/777genius/universal-agent-plugins/actions/runs/33994016590)
- [LintAI release run 33974725553](https://github.com/777genius/lintai/actions/runs/33974725553)
- [Directory sequence 35](https://777genius.github.io/universal-agent-plugins-registry/registry/schemas/1/latest.json)
- [Discovery sequence 38](https://777genius.github.io/universal-agent-plugins-registry/discovery/latest.json)
- [Security sequence 4](https://777genius.github.io/universal-agent-plugins-registry/security/latest.json)
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
  this security extension. Its final release, signed-index publication, and
  browser proofs ran in GitHub-hosted CI or fresh disposable local directories;
  it did not require a VM, LXC instance, or snapshot.

## Ongoing release contract

1. Keep LintAI, policy, signed assessment, and CLI evaluator identities exact.
2. Keep six-platform release and public npm lifecycle checks green for every
   versioned release.
3. Publish Security and Discovery as separately signed feeds; preserve the
   last-known-good snapshot whenever a refresh is incomplete.
4. Add client-specific runtime evidence only in disposable test surfaces and
   keep every claim package- and client-specific.
5. Track Agent Plugins 1.1 separately until its specification is stable.
