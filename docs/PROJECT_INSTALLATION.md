# Install plugins for one project

**Released:** explicit project scope is available in CLI 0.1.93 and later.
CLI 0.1.94 also includes automatic project selection for argumentless `install`
and strict validation of an existing lock before installation or repair.
Explicit scope remains useful in scripts and when restoring a chosen project.
Check `agentplugins version` before relying on 0.1.94 behavior; channels can
update at different times. Use the [native/Homebrew installer](NATIVE_INSTALL.md)
if your selected channel is older, or keep explicit scope with the 0.1.93 CLI.

Use `--scope project` to keep a project's plugin declarations, installed
ownership and native client configuration separate from your user environment.
Select the existing project directory with `--project-root`:

```sh
agentplugins add https://git.example.com/team/review.git \
  --alias review --target cursor \
  --scope project --project-root /path/to/project
```

The same complete-package sources work in user and project scope: Git over
HTTPS or SSH, supported provider browser links, ready HTTPS archives, and local
packages. See [Direct complete package sources](DIRECT_PACKAGE_SOURCES.md) for
selection syntax, private Git credentials and archive limits. A local directory
or local archive uses the existing imperative flow; it does not become a portable
dependency declaration.

For a complete local plugin package, select Claude Code, Codex, or both explicitly:

```sh
agentplugins add /path/to/my-plugin --target claude,codex \
  --scope project --project-root /path/to/project
```

Use `--target claude` or `--target codex` for one client. Local input does not
take a saved dependency `--alias`. The [project evidence index](evidence/PROJECT_INSTALLATION.md)
separates historical failures, blocked observations and successful checks from
the final feature-source native run; each client's activation limits still apply.

## Select the environment on every command

Add and maintenance commands default to user scope unless project scope is
explicitly selected. In CLI 0.1.94, argumentless `agentplugins install` selects
project scope when the exact current directory contains a regular `uap.toml`
file. It never searches parent folders.
A directory or symlink named `uap.toml` is refused during automatic selection;
select the scope explicitly to resolve the ambiguity. Without that local
manifest, install selects the user environment. Explicit `--scope user` or
`--scope project` always wins. Other commands, including install with a source
argument, continue to default to user scope.

With `--scope project`, omitting `--project-root` selects the invocation
directory. `--project-root` requires `--scope project` and an existing directory.
The install preview shows the selected scope, project root where applicable,
and manifest and lock paths.

In CLI 0.1.94, an existing `uap.lock` makes argumentless install
strictly validate the manifest and locked pins before installation or repair.
A mismatch fails without changing
the environment; use the explicit `update` path to advance pins, or `deps lock`
after deliberately editing dependency declarations. Only a missing lock permits
first-install resolution and lock creation. `--locked` additionally requires an
existing valid lock; `--locked=false` does not relax an existing lock's validation.

```sh
cd /path/to/project
agentplugins deps status --scope project
agentplugins install --locked --scope project
agentplugins update review --target cursor --scope project
agentplugins list --scope project
agentplugins remove review --target cursor --scope project
```

`deps status` reports the selected scope, agreement paths, supported targets
and excluded targets. Dependency commands take their targets from `uap.toml`;
they do not accept an inherited `--target` override. Explicit add, update,
repair and removal commands retain their normal target selection rules.

Clients have different native project capabilities. See the
[current project support table](PROJECT_CLIENT_SUPPORT.md) for each target's
paths, components and remaining activation boundary. UAP selects the admitted
adapter for that target and refuses unsupported project requests before source
acquisition. In particular, [ChatGPT has no native project installation](CHATGPT_PROJECT_SCOPE.md),
including with `--no-save`. A client's user installation path is never used as
a substitute for missing project support.

## VS Code project preparation

Select VS Code explicitly for an existing local project folder:

```sh
agentplugins add ./my-plugin --target vscode --scope project \
  --project-root /path/to/project --dry-run
```

Review the plan, then omit `--dry-run` to apply it. The project adapter prepares
skills in `.github/skills/<name>` and exact MCP `servers` members in
`.vscode/mcp.json`. That file accepts its native comments and trailing commas;
UAP does not select an alternative `mcp.jsonc`. Unrelated servers, inputs,
sandbox settings and comments retain their ownership.

The result is **Prepared**, with verification of the installed files. Open the
selected folder in VS Code, review workspace and server trust, and inspect its
skills and MCP configuration. The pinned workbench observation establishes
component metadata and project separation; it does not establish MCP startup,
tool calls, authentication or model execution. Remote workspaces and multi-root
workspaces are outside this project adapter's admitted scope.

Go SDK composition must select the project registry with `all.Project()` or
include the explicit `vscode.NewProject()` adapter. The default user registry
does not admit VS Code project requests. The CLI selects the appropriate registry
from the resolved scope before planning or source acquisition.

## Portable intent and local state

| Location | Contents | Transfer with the project agreement? |
| --- | --- | --- |
| `<project>/uap.toml` | Dependency intent and target assignments | Yes |
| `<project>/uap.lock` | Exact source revisions, archive checksums and package digests | Yes |
| `<project>/.uap/` | Local receipts, locks, recovery journals, managed packages and retained plugin data | No |
| Client-specific project paths | Native skills and the exact MCP entries owned by UAP | Recreate from the agreement |
| User content store | Verified immutable source packages shared across environments | Optional cache, not installed ownership |

Keep the portable pair in version control when you want collaborators to
restore the same dependency selection. Treat `.uap/` as local generated state
and add it to your ignore policy as appropriate; UAP does not edit an existing
`.gitignore` automatically. Back up plugin data separately when needed.

The original user client profile remains user authority. UAP does not rewrite
HOME or turn the project into a replacement user profile. Project A, project B
and the user environment have separate receipts and mutation locks. Project
removal consumes only exact recorded ownership and retains plugin data and
the shared content store by default.

## Maintain and recover the selected project

An intact locked install is a no-op. Repair restores missing owned components
and damaged managed packages from the recorded pin. It preserves externally
edited native skill directories and MCP entries and reports their ownership
conflict for review. `update` explicitly advances a moving source; `repair`
restores the installed pin instead of following the newest branch or archive
endpoint.

```sh
agentplugins deps sync --locked --scope project --project-root /path/to/project
agentplugins repair <installation-id> --target cursor \
  --scope project --project-root /path/to/project
agentplugins deps recover --scope project --project-root /path/to/project
```

Run recovery for the same selected project after an interrupted operation,
then run locked install/sync again. A recovery journal does not authorize
changes to another project's paths. UAP refuses conflicting ownership,
changed native entries, unsafe aliases and changed project identity instead
of adopting or deleting foreign files.

## Restore in another checkout

Copy the project's `uap.toml` and `uap.lock` into the new existing project
directory, install the required clients, and run:

```sh
agentplugins install --locked --scope project --project-root /path/to/new-project
```

From the new project directory in CLI 0.1.94, `agentplugins install` infers
project scope and strictly uses an existing lock. `--locked` additionally
requires the lock's presence. Explicit scope/root flags still take priority.

The explicit transfer commands support project scope too:

```sh
agentplugins deps export /path/to/exchange \
  --scope project --project-root /path/to/project
agentplugins deps import /path/to/exchange --install \
  --scope project --project-root /path/to/new-project
```

Choose an exchange directory different from the selected project; its agreement
already lives at that project root. Export/import preserves the exact two
files, validates the complete pair, and uses the existing confirmation and
locked preflight rules. It does not transfer receipts, native settings,
credentials or plugin data. Cold-cache restore acquires and verifies the
exact recorded bytes; availability and private-source access are still needed.

## Native activation remains visible

Installed project files establish configuration and ownership. A native client
may additionally require a fresh session, workspace trust, an explicit session
configuration, authentication or manual UI action. A prepared result remains
prepared until that client's supported activation boundary is satisfied.
`--activation-complete` does not override a project adapter's unsupported,
shadowed or unobserved discovery result.

UAP does not grant workspace trust or authenticate a client to make installation
look active. Inspect the client's effective project configuration and follow
the reported next action. [OpenCode's project guide](OPENCODE_PROJECT_SCOPE.md)
illustrates the difference between installed configuration and observed native
runtime. The [current project support table](PROJECT_CLIENT_SUPPORT.md) links
client guidance and evidence. The [reviewed client plans](plans/project-install-e2e/client-plans.md)
retain the original proposals and their later observation-driven amendments.
