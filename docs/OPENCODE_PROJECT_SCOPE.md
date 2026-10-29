# OpenCode project scope

OpenCode project delivery installs MCP entries into the selected project's
`opencode.json` or `opencode.jsonc`, and skill directories into
`.opencode/skills/`. Rendered portable commands use the existing skill projection.
UAP keeps its package, data and machine receipts under that project's `.uap/`.
The detected user OpenCode profile remains separate.

```sh
agentplugins add owner/repo --alias docs --target opencode \
  --scope project --project-root /path/to/project
agentplugins deps sync --locked --scope project --project-root /path/to/project
agentplugins repair <installation-id> --target opencode \
  --scope project --project-root /path/to/project
agentplugins remove docs --target opencode \
  --scope project --project-root /path/to/project
```

`--project-root` selects an existing directory explicitly. When omitted in
project scope, UAP uses the invocation directory. It does not search upward for
a different project. Add and maintenance commands default to user scope.
Argumentless install in CLI 0.1.94 selects a current-directory project manifest
unless an explicit scope is supplied.

Existing JSONC selection, comments and foreign settings are preserved. If both
config variants exist, UAP refuses the ambiguity. Exact logical MCP keys are
preserved, including names containing slashes or internal spaces; skill names
remain safe directory identifiers. Disabled foreign MCP entries reserve their
keys and are not adopted. Removal consumes only exact recorded native ownership
and leaves unrelated entries, skills and retained application data intact.

Successful file configuration reports **prepared activation with installed
verification**. This proves the selected native files and ownership. OpenCode
merges user/project/ancestor, custom-directory, inline and managed settings, and
skill discovery also considers compatible directories. These settings can
change what a fresh native session sees. UAP does not grant trust, change your
OpenCode home or overwrite higher-priority policy to force discovery. Activation
attestation cannot convert the file observation into native runtime proof.

Start OpenCode from the selected project and inspect its effective configuration
and MCP listing before relying on a component. The implementation evidence
separates local production contracts, independent review and hosted native
qualification in [the checkpoint record](plans/project-install-e2e/opencode-implementation-evidence.md).
