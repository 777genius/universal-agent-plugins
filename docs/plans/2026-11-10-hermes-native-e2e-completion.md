# Hermes native E2E completion

Date: 2026-11-10. Follow-up to merged #814; production behavior is unchanged.

## Scope and estimates

Extend the existing pinned Hermes 0.23 smoke with actual User Skill discovery
and reading, first/repeated add, update, repair and removal. Exercise the same
Project Skill in isolated repositories A and B, including repair/removal of A
without changing B or the User profile. Preserve the native project-trust gate.
Use the existing bounded Pi HTTP MCP fixture with `hermes mcp test` to prove
native initialize/tools/list without a model or account.

Complexity: 4/10. Expected reliability after native CI: 9/10. Change size:
approximately 200-300 test-only lines (4/10). No new framework or production API.

## Evidence contract

- Pin the real Hermes checkout to `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`.
- Invoke Hermes' own `skills list` and `tools.skills_tool.skill_view`, using its
  installed Python runtime. An on-disk Skill is not discovery evidence.
- Resolve the actual interpreter through `hermes --print-runtime-command`, then
  use the pinned official `runtime_command(code=...)` API to run the observer
  file with `runpy.run_path` after Hermes bootstrap. Its stdlib-only command
  builder can run on the host Python, but the observer uses the real installed
  runtime. Do not pass a filename to `--run-module runpy`: that interface takes
  a module name. Do not alter the pinned source or rely on ambient PYTHONPATH.
- For untrusted projects, invoke Hermes' own discovery/trust functions: the
  selected project's Skill is detected but remains unavailable for loading.
  UAP must leave `skills.trusted_project_dirs` unchanged; no automatic trust.
- Verify native MCP initialize/tools/list on each User lifecycle stage with
  the loopback-only fixture. Do not claim a tools/call or model turn.
- Check native absence after removal, retained foreign Skills/MCP, YAML bytes
  around project operations, and Project B bytes around Project A mutations.
- After Project A repair, compare the restored Skill with the current source
  revision while B must retain its old bytes. For the negative Project MCP
  scenario, require the structured Hermes/project/unsupported MCP plan and
  `mutated: false`; an unrelated nonzero exit is a test failure.
- Keep stage output in the runner's disposable evidence directory; only mark
  this complete after the exact follow-up commit passes native CI.
- On lifecycle timeout, terminate its dedicated Linux process group, including
  surviving descendants, and write the summary with observed MCP methods in
  `finally`. Report `tools_call` from the captured protocol rather than a constant.

## Review before implementation

1. Source review: Hermes `hermes_cli/mcp_config.py::cmd_mcp_test` connects and
   enumerates native tools; `tools/skills_tool.py::skill_view` reads the native
   catalog. Both APIs exist at the pinned commit.
2. Scope review: no model/auth, no change to client adapters, no trust grant,
   no arbitrary host profile. All mutable data belongs to RUNNER_TEMP.
3. Failure review: missing native runtime, version drift, silent discovery
   failure, foreign-state mutation or absent MCP methods must fail the lane.

## Validation

Local: shell syntax and Python compilation. Full lifecycle qualification runs
in the existing Hermes native workflow, which installs the pinned real client
and builds the UAP executable. Queued or skipped runs are not a PASS.

Pinned source references:

- [Native launcher and `runtime_command(code=...)`](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/_launchers.py)
- [Skills listing and reading](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/tools/skills_tool.py)
- [Project discovery and trust](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/agent/skill_utils.py)
- [Native MCP connection probe](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/mcp_config.py)
