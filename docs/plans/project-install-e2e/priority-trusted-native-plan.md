# Priority clients: separate trusted disposable native evidence

2026-10-25. This amendment adds positive native observations without changing
the existing untrusted project tests or production trust behavior. It is
source-only qualification, with exact source/client identities and raw artifacts.

## Authority and supported setup

The owner requested complete project E2E, prioritizing Claude Code and Codex.
All execution retains the existing pinned client, disposable hosted-machine,
scanner, explicit environment and current-source matrix gates. Fixtures author
trust only for their own canonical A/B directories in the original disposable
user configuration, before recording preservation baselines. UAP never writes
trust; project directories never become HOME or native user profiles.

Codex's [official configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
documents `projects.<path>.trust_level`. The exact pinned 0.153.4 source at
[`3d2ee51ca2d5db578f328aa75e20aa22c0197c9a`](https://github.com/openai/codex/blob/3d2ee51ca2d5db578f328aa75e20aa22c0197c9a/codex-rs/config/src/config_toml.rs)
defines `ProjectConfig.trust_level` and recognizes `TrustLevel::Trusted`.
Its [writer](https://github.com/openai/codex/blob/3d2ee51ca2d5db578f328aa75e20aa22c0197c9a/codex-rs/core/src/config/mod.rs)
uses `[projects."<path>"]` with `trust_level = "trusted"`. The release tag
`rust-v0.153.4` was independently resolved to that commit. Both A and control B
are explicitly trusted, so B's absent project servers are not explained by
untrusted-config suppression. Actual pinned-client output must qualify this
setup; documentation alone never passes a native stage.

Claude's [official permission guide](https://code.claude.com/docs/en/permissions#what-runs-before-you-trust-a-folder)
documents setting `projects["<path>"].hasTrustDialogAccepted=true` in
`.claude.json`. Its [container guide](https://code.claude.com/docs/en/devcontainer#persist-authentication-and-settings-across-rebuilds)
confirms that explicit `CLAUDE_CONFIG_DIR` moves that file inside the selected
configuration directory. The fixture records this separately authored trust
input. Pinned 2.1.263 native listing/details must establish whether it works;
a suppression result remains a genuine blocker rather than a bypass candidate.

## Codex MCP scope and minimum implementation

Add a separate opt-in source test, leaving the skills-only untrusted test intact.
Reuse `nativeFixture`, `writePackage`, `nativeScriptedProvider`, pinned scanner
provisioning, `commandsRunInstaller`, `startNativeRPC`, `nativeDecodeFacts` and
`nativeAssertEvent`. No new acquisition, process, MCP server or model harness.
Expected size is approximately 180-220 Go lines: one lifecycle test, one session
helper and one small persisted-binding helper. Add the required source-only
matrix test name and its existing selection assertion; no historical lane changes.

The fixture writes the ordinary user provider configuration with a loopback
provider and `requires_openai_auth=false`, plus canonical trusted A/B entries.
Both projects have real Git markers and remain outside HOME/CODEX_HOME. A starts
without `.codex/config.toml`; UAP creates the entire initial project document.
Keep independent user, B and A foreign sentinels, and hash them after every
public operation and native session. Do not seed any user MCP server, account,
credential, external endpoint or automatic plugin install.

Public UAP adds the existing local `native-proof` stdio fixture into A with
project scope and no-save. Persisted exact scope/client binding, owned MCP
receipts and data receipt supply expected package/data paths. Fresh app-server
sessions initialize, start an ephemeral read-only thread and list MCP status.
Require the exact `default` and `explicit` native configuration names once,
empty pluginId, complete pagination and declared inspect_runtime tool. Direct
tool calls carry a random nonce and correlate initialize/tools-list/tools-call
events; compare revision, project managed root, recorded data path, default and
explicit cwd, exact args and literal/substitution bytes. A user plugin cache
path cannot satisfy the assertion. Do not reuse `nativeSession`'s user-plugin
identity/cache assumptions. Never start a model turn; require zero captured
Responses requests throughout.

Drive initial add, unchanged add/update and removal. Retain the tool's bounded
marker through replay and after removal. Replay requires mutated=false plus
exact state/config bytes. Saved activation remains Prepared even after the
separate native observation. The existing project component test independently
covers changed update and missing-skill repair; this narrow added test does not
claim trusted MCP runtime replay across update or missing-component repair.

Run fresh B sessions before/after A changes and fresh A/B sessions after removal.
Require no project fixture servers and no new nonce-correlated MCP events in
those absent cases. Hash original user/B/foreign bytes after each boundary.
Record exact source/tree/client/installer identities, authored trust input,
native RPCs, event receipts and explicit evidence limits. MCP initialization and
direct fixture tools can pass; real model behavior, OAuth, arbitrary policy,
external services, network isolation and release qualification remain untested.

## Claude positive discovery scope

Add a separate opt-in trusted project listing/details test using
`newClaudeProjectNativeFixture`, the public installer and existing bounded
diagnostic capture/skill-byte assertions. Seed the exact A trust entry in
`ConfigDir/.claude.json` before baselines. Trust both A and control B; B uses the
same original profile and has no A plugin. Preserve existing disabled foreign
user/A/B entries.

Require fresh exact enabled project identity/version/canonical installPath in
A, exact native plugin details and rendered guide/ordinary-command bytes, no A
entry in B, and removal followed by fresh A/B absence. Record scope explicitly;
a user-scoped same-name plugin is not project proof. Preserve user/B bytes and
the authored trust entry. Estimated size is 100-150 lines reusing current
fixture and capture helpers. No session, model, MCP initialization or trust-dialog
execution is claimed. Existing suppression and failed/indeterminate gates stay.

## Review and qualification

Two independent plan rounds precede code. Review frozen changes and meaningful
parser/selection contracts, then publish immediately. Genuine macOS/Windows
native artifacts on the assembled SHA, independently interpreted with hashes,
qualify results. A local compile or a historical client pass is insufficient.
