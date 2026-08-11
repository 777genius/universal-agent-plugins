# Public plugin registry

The public directory is Git-native: there is no account database and every
addition or update is reviewed as a pull request. Built-ins remain the existing
26 local packages and keep their CLI short names. An external package never
receives a short-name alias.

## Submit an external package

Before opening the PR, the package directory at the pinned revision must contain:

- a valid Agent Plugins 1.0 `plugin.json` with author and license metadata;
- a package `README.md`;
- at least one declared component: root `mcp.json` or `skills/*/SKILL.md`;
- no client-specific `.mcp.json` or `.codex-plugin` output.

Add exactly one file at `registry/entries/<plugin-name>.json`:

```json
{
  "schema_version": 1,
  "repository": "owner/repository",
  "revision": "0123456789abcdef0123456789abcdef01234567",
  "path": "path/to/plugin-name",
  "categories": ["developer-tools"]
}
```

The repository must be public on GitHub. `revision` is the full lowercase SHA
of a commit, not a branch, tag, or release name. The path is a normalized,
repository-relative POSIX path whose last segment matches the descriptor name.
Categories are sorted lowercase slugs. No descriptive or trust fields are
accepted: name, version, description, author, license, keywords, and components
come from the package at the pinned revision. Submitters cannot assign
`featured`, `verified`, `official`, `tested`, download, or ranking claims.

Test the same immutable source users will install:

```bash
npx universal-agent-plugins add owner/repository@FULL_40_CHARACTER_SHA//path/to/plugin-name --target cursor --dry-run
```

Then run `python3 scripts/build_registry.py` and commit the updated
`registry/index.json`. Future releases are updates by PR: change the descriptor
SHA and regenerate; the directory does not discover versions automatically.

## What validation means

`schema_only` means static Agent Plugins 1.0 package validation at that exact
revision. It is not an endorsement, ownership check, malware verdict, or proof
that tools behave safely. `runtime_evidence` appears only when the repository
already contains pinned, reviewable client evidence; it is never supplied by a
directory descriptor. External entries currently begin at `schema_only`.

The builder downloads only a bounded GitHub source archive. It rejects
redirects, links, special files, non-portable paths, unsafe MCP auth/URLs,
unbounded trees, and source/name mismatches. It does not run plugin code,
install dependencies, invoke lifecycle scripts, start containers, or contact an
agent runtime.
