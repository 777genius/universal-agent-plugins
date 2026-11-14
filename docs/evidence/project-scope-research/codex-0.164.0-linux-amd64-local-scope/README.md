# Codex 0.164.0 disposable scope observations

> **Historical snapshot.** The observations and qualifications below belong to
> their recorded source. The [original repository directory](https://github.com/777genius/universal-agent-plugins/tree/3b93148c356fc5db6fe101d81d943160a5176b6d/docs/evidence/project-scope-research/codex-0.164.0-linux-amd64-local-scope) preserves the
> files present before raw-output cleanup. Adjacent metadata and source remain;
> original manifests describe that snapshot, not the current directory inventory.
> Retrieve removed raw files there; see the [project evidence index](../../PROJECT_INSTALLATION.md).

Observed 2026-10-24 during the owner-authorized practical spike. This is a local
disposable-authority experiment, not a hosted native-matrix pass, installer
lifecycle qualification, or release evidence. No UAP production implementation
was changed or executed by this spike.

The official `@openai/codex@0.164.0` npm package supplied the measured native CLI.
Its native executable SHA-256 was
`b93f6811d3f653f0fb83be4050d857fb9abbb111dfa2ce160563327daff7f306`.
The npm archive integrity was
`sha512-TWj7UGyPRe3Pj/A3ddNAxs1te3a/W/xAd+J0LHrrFTmsSfahGMOLF/vn88D3QCIgAT3B1rm1y/Z3NYE0rx6cZQ==`.
Unused voice resources were removed from the disposable package during host disk
exhaustion; the measured CLI executable was unchanged. Nothing was installed into
a real user profile. One disposable user authority was outside two sibling
projects; both projects reused that same authority. No project masqueraded as
native user configuration.

`observations.json` retains sanitized native command outputs, selected read-only
RPC responses and raw evidence hashes. Absolute paths describe disposable test
locators only. Each fresh app-server received `initialize`, `initialized` and
read-only `skills/list`, `config/read`, or experimental `plugin/list`. No login,
credential, thread/start, turn/start, model inference, MCP initialization/tool
execution, hook execution, or trust-bypass flag was requested. Ordinary native
trust entries in the disposable user profile distinguished trusted/untrusted
fixtures. This experiment did not establish network isolation or observe all
native background behavior. Do not treat the experimental plugin RPC as a
production-supported UAP dependency.

## Concrete observations

- `skills/list` reported both the same-named user skill and exact selected
  repository skill, with distinct paths/scopes; it did not merge their bodies or
  list the other project's repository skill.
- Trusted A loaded its `.codex/config.toml` model sentinel and project MCP entry.
  Untrusted B retained the user sentinel and excluded its project MCP entry;
  `config/read` supplied the disabled project-layer explanation. Direct B skills
  were still listed. Config trust and skill discovery are separate boundaries.
- Native CLI `plugin add --help` had no project scope option. With only a repo
  catalog, native CLI marketplace/plugin list was empty and plugin add could not
  find the plugin, including when `-C` explicitly selected the project.
- Experimental app-server inventory recognized repo catalogs and project plugin
  enablement. A fresh enabled A loaded its namespaced plugin skill from native
  user cache, not directly from the repo plugin path.
- Trusted A and B using identical marketplace/plugin/version with different
  inert bodies shared one native cache identity. B's inventory resolved B's
  source, but B skill discovery loaded A's cached body. Repetition preserved A.
- Giving B a distinct marketplace identity selected B's body in a distinct native
  user-cache path while A retained A's body.
- Explicit native `plugin marketplace add <A>` then `plugin add ... --json`
  succeeded and wrote marketplace/plugin enablement into the disposable **user**
  config. Native plugin list from B then showed that user-installed A plugin.

These observations support keeping UAP's exact project skill/config component
route. They do not qualify MCP operation, native session skill use, user-project
name override, or a project-owned native plugin catalog/cache lifecycle.

Official documentation checked during the spike:

- https://learn.chatgpt.com/docs/build-skills
- https://developers.openai.com/plugins/build/plugins
- https://learn.chatgpt.com/docs/app-server

Current documentation distinguishes repository catalogs, trusted project config,
user plugin cache, and experimental app-server plugin methods. The observed CLI
versus app-server inventory difference and same-identity cache coupling are
version-specific experimental findings, not assumptions about future clients.
