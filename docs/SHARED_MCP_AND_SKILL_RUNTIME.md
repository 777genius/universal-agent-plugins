# Share MCP packages and prepare Skill runtimes

Use one explicit dependency for a shared component. Keep the component's package,
client configuration and running process as separate concepts. This guide uses
existing UAP contracts; it does not introduce automatic MCP merging or a new
runtime executor.

## Two plugins, one shared MCP package

Suppose `plugin-a` and `plugin-b` both need the same MCP implementation. Publish
that implementation as a standard package, for example `acme/shared-mcp`, instead
of copying its `mcp.json` into both consumers. Each consumer keeps its own standard
`plugin.json` and adds this package-root `uap.toml`:

```toml
schema_version = 1

[dependencies]
"acme/shared-mcp" = "^1.0"
```

Use the same canonical source in both declarations. Matching bytes obtained
through unrelated Registry and Git identities do not make those sources one
publication authority. The dependency declaration is a UAP manager contract;
another Agent Plugins client does not automatically understand `uap.toml`.

The shared package owns its standard `plugin.json` and `mcp.json`. For example:

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "shared-mcp",
  "version": "1.0.0"
}
```

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
  "mcpServers": {
    "shared-service": {
      "type": "streamable-http",
      "url": "https://mcp.example.com/mcp"
    }
  }
}
```

These package names and endpoint are illustrative, not published services. Replace
them with reviewed packages and their real configuration. Define account setup in
the package instructions; do not commit credentials into either manifest.

The user selects consumers and targets in the selected environment's `uap.toml`.
Here `plugin-a` and `plugin-b` are their registered Directory aliases:

```toml
schema_version = 1

[dependencies]
plugin-a = "^1.0"
plugin-b = "^1.0"

[targets]
codex = ["plugin-a", "plugin-b"]
```

For an explicit user environment, find its files with `deps status`, review the
manifest, resolve it, inspect the graph and install the exact lock:

```sh
agentplugins deps status --scope user
agentplugins deps lock --scope user
agentplugins deps tree --scope user
agentplugins install --locked --scope user
```

Use [project scope](PROJECT_INSTALLATION.md) instead when the environment belongs
to a project. Keep the same selected scope and root for later operations.

Both consumer constraints must be compatible. The shared dependency inherits
the union of their selected targets and has one selected package version in that
environment. An incompatible requirement is a conflict to resolve, not permission
to install an ambiguous second native resource. Package-local targets and locks
do not override the user's chosen graph.

### Updating and removing consumers

Review `deps tree` before changing the manifest. Remove only `plugin-a`'s direct
declaration and target assignment, then run `deps lock` and the same locked
install. The shared package remains while `plugin-b` or another direct root needs
it. Removing the final reachable consumer removes the unneeded owned bindings;
an unreachable cycle does not keep itself installed. The immutable package store
and allocated `PLUGIN_DATA` remain retained under the existing removal policy.
Manual client uninstallation, when required, remains an explicit next action;
`--external-uninstalled` is an attestation, not a way to perform it automatically.
See [dependency lifecycle and recovery](UAP_DEPENDENCIES.md).

One shared package does **not** promise one process across agents or projects.
A stdio server is launched by its client. Do not combine existing MCP entries
merely because names or URLs match: accounts, permissions, data directories and
ownership can differ. Do not rename a conflict or adopt someone else's native
entry silently. Inspect the owner and the selected client/profile before acting.

## Skills already have an artifact lock

The saved Skill source, bundled scripts, resources and bundled lockfiles are
pinned by UAP. That does not automatically install every library a script imports,
pin the host interpreter, or freeze a remote API. See
[Skill source identities](AGENT_SKILLS_INSTALLER.md).

An author should state the supported interpreter/package-manager versions,
platforms, required credentials and reviewed execution command in the Skill's
instructions. A `compatibility` description is useful information, not proof that
the host has those requirements. Follow the host's documented Skill path contract:
MCP's `${PLUGIN_ROOT}` and `${PLUGIN_DATA}` guarantees do not automatically apply
to arbitrary Skill shells or Markdown.

### Python example: a script and its adjacent lock

For a standalone script, declare inline metadata and create its lock in the
**author's writable checkout**, not an installed immutable package. This minimal
example has no third-party dependencies; a real Skill lists its actual imports.

```python
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
import json

print(json.dumps({"ok": True}))
```

Save it as `scripts/check.py`, then create and commit its lock:

```sh
uv lock --script scripts/check.py
```

Ship both `scripts/check.py` and `scripts/check.py.lock`. In a reviewed execution
context with the documented uv/Python versions, consume that lock rather than
silently resolving new dependencies:

For a strict lock requirement, check the adjacent file before invoking uv.
On macOS/Linux:

```sh
test -f scripts/check.py.lock || { echo 'Missing script lock' >&2; exit 1; }
uv run --locked --script scripts/check.py
```

In PowerShell, use `if (-not (Test-Path -LiteralPath 'scripts/check.py.lock' -PathType Leaf)) { throw 'Missing script lock' }` before the same `uv run` command.

`--locked` rejects stale locks. Some uv releases warn and continue unlocked when
a script lock is missing, so the explicit existence check is necessary for a
strict guarantee. Dependencies, environments and caches
can still require writes and downloads, and uv can acquire Python when allowed by
its configuration. Specify the host policy separately; this is not a sandbox or
an offline guarantee. Do not regenerate a lock inside UAP's managed package/store
or silently remove `--locked` after an error. Use a separate user-owned runtime
workspace when the tool or script needs writable adjacent files.

### Node example: consume the shipped npm lock

An author ships matching `package.json` and `package-lock.json`, records supported
Node/npm versions and documents a reviewed entry point. In a **separate writable
runtime copy** of that package, not the immutable managed tree:

```sh
npm ci --ignore-scripts --omit=dev
node scripts/check.js
```

`npm ci` requires a matching lock. `--ignore-scripts` prevents dependency lifecycle
scripts; it does not make the later explicit `node` execution safe. Packages that
need a build step or native addon require an explicit supported preparation
policy, not a global switch enabling arbitrary setup scripts. Never install
packages globally merely to make a Skill work.

Selected MCP packages already have UAP's reserved locked-npm preparation path.
Arbitrary Skill scripts do not automatically opt into it. Reuse that path only
for packages that satisfy its existing declaration and validation contract; do
not invent another runtime lock or treat any bundled npm lock as authorization.

## Explain the next action, not a misleading ready badge

Keep these facts separate in instructions and user-facing status:

| Fact | Useful wording and next action |
| --- | --- |
| Package bytes installed | "Installed." This alone says nothing about script execution. |
| Runtime not checked | "Runtime requirements have not been checked. See the Skill's prerequisites." |
| Required tool absent | Name the missing tool and supported version; do not reinstall the Skill repeatedly. |
| Client activation pending | Follow the adapter's reported activation action. |
| Account setup pending | Complete the client/service sign-in separately. |

These are wording recommendations, not new CLI statuses or claims that UAP
currently diagnoses every runtime. Keep technical identities in details, not the
ordinary success message. Do not execute package setup merely to inspect a Skill
or render its catalog card.

## References and verification boundary

The existing composed `TestInstallDependencyGraphLifecycle` exercises shared
package edges, retained locks, explicit updates and sequential consumer removal.
It is not proof of shared running processes, account isolation or arbitrary
runtime execution. The portable package-manager suite adds broader graph,
configuration and failure coverage without changing that distinction.

Primary runtime guidance: [Agent Skills scripts](https://agentskills.io/skill-creation/using-scripts),
[uv script locks](https://docs.astral.sh/uv/guides/scripts/#locking-dependencies),
[uv run --locked](https://docs.astral.sh/uv/reference/cli/#uv-run--locked),
and [npm ci](https://docs.npmjs.com/cli/v11/commands/npm-ci/).
Design tradeoffs and research evidence are in the
[MCP/runtime report](research/dependency-runtime-20261029/mcp-runtime-policy.md).
