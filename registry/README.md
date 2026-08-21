# Public plugin registry

The public directory is Git-native: there is no account database and every
addition or update is reviewed as a pull request. Built-ins remain the existing
26 local packages and keep their CLI short names. An external package never
receives a short-name alias.

## Submit an external package

The review source is `registry/directory.json`. Before editing it, confirm that
the package at the proposed revision contains:

- a valid Agent Plugins 1.0 `plugin.json` with author and license metadata;
- a package `README.md`;
- at least one declared component: root `mcp.json` or `skills/*/SKILL.md`;
- no client-specific `.mcp.json` or generated `.codex-plugin` output.

Test the same immutable source users will install:

```bash
npx universal-agent-plugins add owner/repository@FULL_40_CHARACTER_SHA//path/to/plugin-name --target cursor --dry-run
```

Use a public GitHub repository and the exact lowercase 40-character commit SHA,
never a branch or tag. Then edit `registry/directory.json` in one focused PR:

1. Add a product with its stable identity, aliases, categories, minimum
   capabilities, default distribution, and sorted distribution IDs.
2. Add a namespaced distribution such as `owner/plugin-name` and list that ID
   on the product. Its `kind`, `status`, and `packager` are reviewed claims.
3. Add an immutable release whose `package_source` is the exact
   `owner/repository`, full SHA, and normalized package path. Record its package
   version, components, manifest digest, and `agentplugins-tree-sha256-v1` tree
   digest.
4. Add the one-for-one entry in `release_policies` for that release sequence,
   including status, minimum installer version, supported targets, delivery,
   and only evidence IDs already present in the Directory source.

For an update, append a new monotonically increasing release and matching
policy. Do not rewrite an existing release tuple or its package bytes; policy
status and current evidence may change through review.

Generate and verify the deterministic review files:

```bash
python3 scripts/build_registry.py
python3 scripts/build_registry.py --check
python3 -m unittest tests.test_build_registry
```

Commit `registry/directory.json` and any changed `registry/review-preview.json`
or `registry/review-search.json`. The legacy catalogs and flat index are frozen
and must not change.

## What validation means

Static schema and package checks at the pinned revision are not endorsement,
ownership verification, a malware verdict, or proof of safe runtime behavior.
Do not add or promote evidence claims from the direct-package preflight:
`current_evidence` may reference only exact, reviewable observations already in
`registry/directory.json`, bound to that distribution, release sequence, tree
digest, client, and outcome. The builder does not run plugin code, install
dependencies, start containers, contact an agent runtime, or prove OAuth and
tool behavior.
