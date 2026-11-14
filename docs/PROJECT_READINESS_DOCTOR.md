# Project agent readiness: facts, native proof and repair boundaries

The project-scoped agentplugins doctor command is a read-only observer. It reports what UAP recorded or safely verifies selected project-owned files. It cannot prove that a native client accepted a workspace, loaded a Skill, started an MCP server, used a tool or authenticated a vendor account.

This document describes the **source implementation in this PR**. A merged source revision is not proof that a published npm CLI release already contains the diagnostics.

## Scope and commands

Run the installed CLI from the selected project, or select an explicit root:

    agentplugins doctor --scope project --project-root /path/to/project --format json
    agentplugins doctor --scope project --project-root /path/to/project

Project B and the user environment do not inherit project A receipts or discovery. The command does not modify native files, persistent state, locks or receipts. Existing findings remain intact; a new deterministic data.project_readiness array provides one status per recorded project binding.

Each entry includes installation and client IDs, binding ID, status, materialization, activation, authentication, policy, verification, native evidence and bounded next actions. Absolute profile paths, tokens, MCP environment variables, raw config, server arguments and credentials are excluded.

A prepared_native_unverified record is **not** a failed installation. It means the file delivery is Prepared, but fresh native discovery/model tool execution remains unobserved by doctor.

## Status meanings

| Status | Meaning | Safe action |
| --- | --- | --- |
| source_rebind_required | Original source identity needs review | Rebind the saved exact source manually |
| failed | Recorded materialization/activation/authentication or verification failed | Inspect the recorded failure before repair |
| policy_blocked | Native/managed policy forbids the binding | Ask the policy owner, do not bypass |
| policy_approval_required | Native consent is pending | Approve only in a trusted native UI |
| authentication_pending | Provider account access pending | Authenticate in the native client |
| materialization_pending | Native files not materialized | Recover the existing transaction |
| prepared_native_unverified | Project delivery prepared, native use unobserved | Start the exact project and inspect discovery |
| manual_activation_required | External client activation is required | Perform the documented native activation |
| runtime_verified_in_record_only | Prior runtime evidence exists | Run a fresh native session |
| native_unverified | Binding recorded Active, runtime verification absent | Verify native selection and tool availability |
| not_qualified | Insufficient facts | Inspect authoritative receipts and native client |

These are classifications of the existing state machine, not new persisted activation states. Failure and blocked states take precedence over prepared files. Native readiness guidance does not upgrade policy or authentication.

## Agent-specific verification

| Client | Prepared project surfaces | Required separate native proof |
| --- | --- | --- |
| Codex | .agents/skills and owned .codex/config.toml entries | Project trust, effective codex mcp list --json, real tool call |
| Claude Code | Project Skills and plugin projection | Workspace trust, /plugin, /mcp, reload/restart |
| OpenCode | .opencode/skills, opencode.json or opencode.jsonc | Effective v1 **or** v2 config, native MCP status, no silent generation switch |
| Gemini CLI | .gemini/skills and .gemini/settings.json | Workspace trust, Skills discovery, MCP status |
| GitHub Copilot CLI | .github/skills and project MCP config | CLI trust and fresh project session |
| Cursor | .cursor/skills and project MCP | Editor trust, native server status |
| Pi | .pi/extensions, .pi/skills and .pi/mcp.json | Explicit Pi trust, fresh extension loading and real MCP tool |
| Kimi Code | .kimi-code/skills and .kimi-code/mcp.json | Correct selected Git root, native loaded Skill |
| Windsurf / Devin | Selected project-native surfaces | Exact observed client generation and import |
| Kiro | Project .kiro surfaces | Settings, agent selection and trust |
| Cline | Project .cline surfaces | Effective process MCP settings and Skill selection |
| VS Code | .github/skills and .vscode/mcp.json | Folder trust, explicit MCP activation |

No vendor executable, provider authentication or model is started by the new readiness matrix. File presence alone never implies effective discovery.

## Bounded Codex file audit

When the recorded project binding has exact native-object receipts, doctor may safely inspect Codex-owned Skills/MCP components:

- Exact project root and layout: each Skill directory must be at .agents/skills/NAME, MCP membership must be in .codex/config.toml. Duplicate identities and unexpected aliases are diagnosed.
- Root-confined path handling: Go os.Root and per-component Lstat check against traversal, linked files and irregular paths. Unknown or unsupported paths do not become verified.
- Bounded reads: missing Skill directories, missing regular SKILL.md, absent native TOML, invalid TOML or too-large TOML are reported. The TOML limit is 1 MiB; the object inventory limit is 512.
- Owned members: an exact recorded MCP server name must appear in the native document. Foreign names, comments, environment settings and user-owned configuration are neither adopted nor modified.
- Evidence limits: a valid document is not proof of MCP connection, model use or authentication. The real Codex native E2E is separate from this observational doctor.

If a receipt is not available or cannot safely be inspected, doctor reports unknown, not a fabricated successful native result. Diagnostic values contain stable codes and fixed text; raw project paths and config content are never emitted.

## Repair and scope isolation

A Prepared project should not be automatically reinstalled merely because it is not yet Active. Inspect native trust, selected config precedence and reload behavior first.

If a native owned file is missing, review the authoritative receipt and then choose an explicit repair with the original project scope and project root. Never repair a user profile while diagnosing project A; never adopt foreign files to make a warning disappear. A foreign MCP server must be proven working with a fresh native session after UAP removal.

Project doctor does not replace package-manager E2E, native OS E2E, source identity verification, provider authentication or model-driven tool-call proof.
