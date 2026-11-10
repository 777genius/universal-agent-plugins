# Hermes native E2E completion

Date: 2026-11-10. Follow-up to merged #814. The initial Linux extension changed
only the test harness. Cross-platform qualification also exposed the profile
selection defects covered by the [profile correction plan](2026-11-10-hermes-windows-profile.md).

## Scope and estimates

Extend the existing pinned Hermes 0.23 smoke with actual User Skill discovery
and reading, first/repeated add, update, repair and removal. Exercise the same
Project Skill in isolated repositories A and B, including repair/removal of A
without changing B or the User profile. Preserve the native project-trust gate.
Use the existing bounded Pi HTTP MCP fixture with `hermes mcp test` to prove
native initialize/tools/list without a model or account.

Complexity: 4/10. Expected reliability after native CI: 9/10. Change size:
approximately 700-900 test/workflow/documentation lines (5/10), plus a narrow
profile-selection correction and its regressions. No new framework or production API.

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
- On lifecycle timeout, terminate its dedicated POSIX process group, including
  surviving descendants, or the owned Windows process tree with bounded
  `taskkill /T /F`. Write the summary with observed MCP methods in `finally`. Report `tools_call` from the captured protocol rather than a constant.

## Review before implementation

1. Source review: Hermes `hermes_cli/mcp_config.py::cmd_mcp_test` connects and
   enumerates native tools; `tools/skills_tool.py::skill_view` reads the native
   catalog. Both APIs exist at the pinned commit.
2. Initial Linux scope review: no model/auth or trust grant, no arbitrary host
   profile. All mutable test data belongs to RUNNER_TEMP. The later adapter
   correction is reviewed separately in the profile correction plan.
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

## Platform qualification addendum

The first extended Linux native run passed at UAP head
`b3ed848effaca7d436c1306e06f2d24e67b9e0b2`:
[run 42802367363](https://github.com/777genius/universal-agent-plugins/actions/runs/42802367363).
Its retained summary reports four initialize/tools/list exchanges, no tools/call,
no model turn and lifecycle exit 0. This is evidence for that Linux revision.

The pinned Hermes [README](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/README.md#quick-install)
documents macOS and native Windows as well as Linux. Its
[POSIX installer](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/scripts/install.sh)
contains both Darwin architecture pins. Native Windows is supported through a
separate PowerShell installer; it must not be described as WSL-only.

Before implementation: reuse the current lifecycle unchanged on Ubuntu 24.04
and macOS 15, make the negative Project MCP snapshot comparison portable with
Python byte equality, and include OS/architecture in evidence names. Keep the source pin,
native observer, real MCP exchanges and all ownership/trust assertions. Update
the stale draft documentation with the exact Linux evidence and retain pending
native-platform and release gates. This initial matrix plan did not require a new framework.

Self-review ratings: scope 10/10 (existing smoke only), source confidence 10/10
(pinned official installer), complexity 2/10 (matrix and portable comparison),
qualification confidence pending actual macOS CI. A queued lane is not proof;
native Windows and complete cross-platform support remain unqualified here.

Integration decision after Windows source review: require the same lifecycle
on all three operating systems. Move the existing shell sequence to a small
shared Python test driver, use Hermes' native PowerShell installer on Windows,
and retain POSIX/Windows-specific process cleanup only at the process boundary.
This removes shell-path conversion and duplicate lifecycle scenarios without
adding production APIs. macOS and Windows remain pending until actual native
CI passes; released-package and Directory qualification remain separate gates.

Cross-platform source review then found that UAP dropped `HERMES_HOME` and
`HERMES_DATA_DIR_SUFFIX` from its environment snapshot and selected the POSIX
default on Windows. The adapter and environment-capture correction is scoped
in the separate profile plan above. Every native lane now selects the explicit
nondefault `<fixtureHome>/selected-profile`; coinciding with the vendor default
can no longer hide an ignored override. Injected platform-default and NewOS
composition tests cover the complementary default selection contract.
