# Delivery Status Ledger

This ledger preserves the historical delivery state of the retired
`plugin-kit-ai` v1 architecture after `v1.0.0`. It is implementation history,
not current installation or release guidance. The sole public CLI is now
`agentplugins`; see [its release runbook](./agentplugins-release.md).

## Historical Release Phase

- `v1.0.0 released`
- `community-first interpreted stable subset promoted on main`
- `release evidence refreshed after tag`
- latest deterministic patch candidate rehearsal recorded at `6d0bc9d27e51b67e5945f2c362e4f05662b6e185`

| Area | Status | Notes |
|------|--------|-------|
| Runtime foundation | done | Platform-neutral runtime, generated lookup, middleware chain, and platform registrars are shipped. |
| Descriptor system | done | Runtime wiring, scaffold rules, validate rules, registrars, and support docs are generated from descriptor definitions. |
| Generated contract docs | done | Support claims are emitted from descriptors into the generated support matrix and exposed through `plugin-kit-ai capabilities`. |
| Public contract freeze | done | `README`, support policy, and SDK stability docs now describe the current shipped contract instead of a transition state. |
| Codex GA path | done | Runtime, scaffold, validate, integration coverage, repository-owned opt-in real `codex exec` smoke, real `codex mcp get --json` preflight, and generated `codex-package` `.mcp.json` sidecar preflight exist. The supported invocation semantics are now frozen in the release audit, external runtime-health failures are explicitly outside the plugin-kit-ai stable promise, and current live evidence also records that Codex CLI `v0.117.0` does not reliably honor project-local `.codex/config.toml` for `exec` or `mcp get`. |
| Claude stabilization | done | Deterministic coverage, real-CLI smoke policy, and declared event-set review are complete. External Claude runtime connectivity failures are now handled as documented release waivers when hook execution is never reached. |
| OpenCode stable subset | done | Repo-local local-plugin-loading contract for official-style plugin subtree ownership and plugin-local dependency metadata is now stable-reviewed through [OPENCODE_STABLE_PROMOTION_AUDIT.md](./OPENCODE_STABLE_PROMOTION_AUDIT.md). Helper-based custom tools remain `public-beta`. |
| OpenCode tools beta evidence | done | First-class standalone OpenCode tools now have dedicated beta evidence through [OPENCODE_TOOLS_BETA_AUDIT.md](./OPENCODE_TOOLS_BETA_AUDIT.md) and the opt-in `test-opencode-tools-live` smoke path. |
| Cursor packaged lane | done | Packaged Cursor lane now uses `.cursor-plugin/plugin.json` plus portable `plugin/skills/**` and optional shared `.mcp.json` as the primary public path. The older repo-local `.cursor` subset remains available only as the secondary `cursor-workspace` target. |
| Quality gates | done | `required`, `polyglot-smoke`, `extended`, and `live` lanes exist in repo automation. `required` now includes generated-artifact drift checks, and `polyglot-smoke` runs on `main` plus PRs for launcher/ABI checks, generated Claude/Codex config canaries, and generated runtime-artifact drift protection. Install compatibility now has both a deterministic matrix and refreshed live raw-binary / supported-tarball / unsupported-layout evidence. |
| Community polyglot subset | done | `python` and `node` repo-local local-runtime authoring plus local and remote exported bundle handoff on `codex-runtime` and `claude` is promoted in the source tree through [INTERPRETED_STABLE_SUBSET_AUDIT.md](./INTERPRETED_STABLE_SUBSET_AUDIT.md); `shell` remains `public-beta`. Official CLI bootstrap via Homebrew, the `public-beta` npm wrapper, the `public-beta` PyPI/pipx wrapper when that release was published to PyPI, official shared authoring helpers via `plugin-kit-ai-runtime` on npm and PyPI, verified fallback bootstrap via `scripts/install.sh`, official CI setup via `setup-plugin-kit-ai@v1`, and generated `bundle-release.yml` workflow extras now make that subset self-serve for downstream repos. |
| Release discipline | done | Changelog, CI lanes, release checklist, audit ledger, release playbook, release-notes template, rehearsal worksheet, install verification, generated-sync gate, version command, release-preflight, and a dedicated root release-assets workflow now exist. Downstream Homebrew/npm/PyPI automation now follows successful `Release Assets` completion or a manual tag-scoped rerun. Release rehearsal now includes the executable-runtime deterministic gate. A full release rehearsal has been recorded. |
| Security and diagnostics | done | Threat model, diagnostics contract, checksum verification, install compatibility contract, deterministic regression coverage, and refreshed live install compatibility evidence now exist. |
| `v1.0` readiness | done | `v1.0.0` is tagged at `6e9379868a666e79d7530a02e171a160c2cb1689`. Rehearsal evidence, stable-approved decisions, and post-tag live install compatibility refresh are recorded. |

## Current Blockers

- none for `v1.0.0`

## Post-Release Notes

- `v1.0.0` tag: `6e9379868a666e79d7530a02e171a160c2cb1689`
- current `main` is ahead with evidence refresh work
- the current source tree also carries the post-`v1` interpreted stable-subset promotion ledger in [INTERPRETED_STABLE_SUBSET_AUDIT.md](./INTERPRETED_STABLE_SUBSET_AUDIT.md)
- the current source tree also carries the post-`v1` OpenCode stable-subset promotion ledger in [OPENCODE_STABLE_PROMOTION_AUDIT.md](./OPENCODE_STABLE_PROMOTION_AUDIT.md)
- latest deterministic `v1.0.x` candidate rehearsal:
  - candidate SHA: `6d0bc9d27e51b67e5945f2c362e4f05662b6e185`
  - date: `2026-03-29`
  - `make release-gate`: `pass`
  - `make test-install-compat`: `pass`
  - `make test-polyglot-smoke`: `pass`
  - generated-config/runtime-contract drift protection included in the recorded `polyglot-smoke` evidence
- any future patch tag should be cut from a newly evidenced candidate SHA, not by rewriting `v1.0.0`
