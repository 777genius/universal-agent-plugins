# Claude Code project installation

Use `agentplugins add <source> --target claude --scope project --project-root <directory>`
to select an existing project explicitly. Default installation scope remains
user. Explicit project scope ships in CLI 0.1.93. Historical released binaries and
their qualification do not change when newer source is merged.

UAP projects selected portable skills, supported commands and MCP declarations
into `.claude/skills/<managed-artifact>/` as a Claude Code skills-directory plugin.
It preserves the publisher's manifest name. It does not change HOME, replace the
native user profile, edit trust, or turn existing enablement settings on.

## Read the result

- **Prepared, installation verified:** exact owned files are installed, but Claude
  reports project suppression, disabled state or a same-name user copy instead.
  The fresh operation provides a next action. This is not loaded-plugin proof.
- **Active, installation verified:** a fresh Claude list reports the exact enabled
  project path. This proves discovery, not MCP initialization, authentication or
  successful model/tool operation.
- **Failed/unknown:** native output or ownership could not be recognized safely.
  UAP preserves the uncertainty and refuses to manufacture activation.

Plain status remains host-free: it shows saved lifecycle and verifies the owned
projection, without launching Claude. It does not claim to have freshly checked
trust or name precedence. Repeating add/update/repair reobserves native state.

Start Claude Code from the selected project root. Whole plugins in a skills
directory do not inherit ordinary skills' parent-directory discovery. Review
Claude's trust dialog yourself before requesting a new verification; installation
does not grant trust. Existing native policy and per-server MCP approvals remain
applicable.

A same-name plugin under the user's skills directory can take precedence over
the project copy. A project enablement boolean addresses the id and does not
select one copy. UAP preserves the user installation and reports the blocked
project result. It does not promise independent simultaneously active versions
of one manifest name. Desktop worktree discovery can use the main checkout;
independent worktree activation is not qualified here.

Remove targets the exact owned project directory through the normal verified
transaction. Recognized inactive/suppressed/shadowed files can be removed without
native uninstall or changing the user copy. Changed, foreign, symlinked or
ambiguous ownership still refuses automatic deletion. A previously active
binding with contradictory native evidence requires a supported confirmed
reobservation before removal. Plugin data retains the existing receipt policy.

## Evidence boundary

The pinned 2.1.263 Linux spike observes untrusted listing and configuration-id
behavior. The new genuine prepared-lifecycle harness retains the hosted guard
and requires its own successful run. Trusted project discovery, MCP startup,
authentication, model requests, and desktop/worktree behavior need separate
observed qualification. Existing user native runtime requirements are unchanged.

Official native rules: [plugin loading](https://code.claude.com/docs/en/plugins/loading),
[creating plugins](https://code.claude.com/docs/en/plugins/create), and
[settings](https://code.claude.com/docs/en/settings). Current documentation does
not upgrade a historical pinned test result.
