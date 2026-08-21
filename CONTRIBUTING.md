# Contributing

Contributions that improve portability, validation, documentation, or an
existing package are welcome.

## Add or update a plugin

For an existing in-repository package, keep its portable source under
`plugins/<plugin-name>/`: use a root Agent Plugins 1.0 `plugin.json`, put skills
only in immediate child folders under `skills/`, and add `mcp.json` only for a
vendor-documented endpoint or reproducibly pinned bundled server. Keep its
`README.md`, `SOURCES.md`, and `docs/COMPATIBILITY.md` accurate.

Before opening a pull request, test the package directly from its local folder:

```bash
npx universal-agent-plugins add ./plugins/<plugin-name> --target cursor --dry-run
```

Packages hosted in another repository can be tested without copying them into
this catalog by using an immutable GitHub source:

```bash
npx universal-agent-plugins add \
  owner/repo@FULL_COMMIT_SHA//path/to/plugin \
  --target cursor --dry-run
```

To add a public Directory listing, do not add a new flat catalog descriptor.
Follow the [external package submission guide](registry/README.md#submit-an-external-package):
preflight the exact full-SHA package source, then add its product, namespaced
distribution, immutable release, and matching policy to
`registry/directory.json`.

```bash
python3 scripts/validate_catalog.py
python3 scripts/build_registry.py
python3 scripts/build_registry.py --check
python3 scripts/build_openai_compat.py --check
python3 scripts/validate_openai_compat.py
python3 -m unittest discover -s tests
```

Do not add credentials, token placeholders in remote headers, undocumented OAuth
metadata, shell command strings, floating dependency tags, or endpoints copied
from a client-specific hosted connector.

Package descriptions must say `Community package for` rather than implying that
777genius authored, owns, or is endorsed by the upstream service.

## Pull requests

Keep pull requests focused. Explain the upstream source, what changed, how the
package was validated, and whether authentication or tool permissions changed.
Use conventional commit titles such as `feat: add example plugin` or
`fix: update context7 server pin`.

## Release checklist

1. Merge through protected `main` after `portable-catalog` and review pass.
2. Push the new `v*` tag. The workflow rejects a ref/SHA mismatch or a commit
   outside `main`, then runs both live E2E jobs.
3. The dependent publish job creates the immutable GitHub release only after
   both jobs pass.
4. Confirm the README pin, release page, and Live E2E badge are green.
