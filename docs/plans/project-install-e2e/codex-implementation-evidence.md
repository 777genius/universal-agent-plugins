# Codex project component implementation evidence

Pre-code evidence recorded 2026-10-23. This implements the reviewed single
component lane from `client-plans.md`: project skills and command skills in
`.agents/skills/`, plus exact MCP entries in `.codex/config.toml`. Existing user
marketplace/plugin behavior remains separate and unchanged.

## Official pinned inputs

- Codex CLI **0.153.4**, official asset:
  https://github.com/openai/codex/releases/download/rust-v0.153.4/codex-x86_64-unknown-linux-musl.tar.gz
- Asset SHA256 `f479424eca092484dc40d87ae28c44f4cc40234a60045d6131e493800d814a30`,
  98,582,872 bytes. Binary SHA256
  `56ef98ab4032d317ab26e9b5e5a175650717351edb16ed9cde0cb6d1734d62da`,
  258,659,424 bytes.
- Parent independently ran this binary's version, top-level help, MCP help and
  `mcp list --help` in a networkless namespace with a fresh home. These prove
  version/help contracts only, without credentials, trust or model requests.
- Exact-tag native schema:
  https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/core/config.schema.json
  SHA256 `692da7699367f6f4fbbd46c0021278c1311440bcebf0bcb9b836690c05e56196`, 200,401 bytes.
- Exact-tag config-layer source:
  https://github.com/openai/codex/blob/rust-v0.153.4/codex-rs/config/src/loader/README.md
  and `loader/mod.rs`. Skill source is under `codex-rs/ext/skills/src/loader/`.

## Confirmed native contract

The pinned schema defines `mcp_servers.<name>` with native `command`, `args`,
`env`, `cwd` for stdio and `url`, `http_headers` for HTTP; it supports `enabled`.
Skills have `[[skills.config]]` disable selectors by absolute `path` or `name`.
The current official docs describe repository `.agents/skills` discovery from
cwd up to repo root and allow same-name skills from separate roots to coexist.
Managed ownership of a file does not establish which skill a session selects.

The exact config loader applies project layers only after native trust admission;
user profile, user base, system, cloud-managed, session and legacy managed layers
remain separate. Project config walks from the detected root toward cwd. The
implementation does not change `CODEX_HOME`, add trust or duplicate that settings
engine. An MCP entry may inherit/merge user or higher-priority policy fields.

Current official pages were opened before implementation:

- https://developers.openai.com/codex/config-file/config-basic
- https://developers.openai.com/codex/config-file/config-advanced
- https://developers.openai.com/codex/mcp
- https://developers.openai.com/codex/build-skills

## Implementation boundary

- Use core's validated TargetContext and scoped native-config lifecycle policy;
  native root is the selected project, with `.agents/skills` and `.codex/config.toml`
  validated independently under it. Original detected user profile stays intact.
- Add the narrowly required `CodecCodex` TOML format to the existing nativeconfig
  kernel. Reuse exact receipts, writer locks, no-follow file IO, conditional
  replacement, postconditions and rollback. Preserve foreign TOML members and
  comments. TOML decoding must reject duplicate/malformed documents before edits.
- Reuse existing loader, rendered command skills, immutable staging and native
  skill transaction semantics. Exact project objects govern update, repair,
  registry collision checks and removal; project operations never run global
  marketplace registration/removal or clean a user plugin cache.
- File-only success remains prepared with installed exact objects. Bounded
  configured disabled/shadowing warnings do not claim complete effective-policy
  evaluation. Generic activation attestation cannot manufacture project native
  discovery. Unknown versions or unavailable native observation remain explicit.
- Composed fixture journeys and genuine pinned process observations are recorded
  separately. Hosted native guard remains unchanged; no local model task, trust
  mutation, credential creation or package payload execution is authorized by
  this evidence record.
