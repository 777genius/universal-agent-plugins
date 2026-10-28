# Gemini research: exact measured MCP-refresh stderr

## Observed failed boundary

Current PR526 eef6c202abf02489338db54c0ffbb9cb2c283ca6/tree
3f17100dcc7de10e24e635d2ee91752e8c5b7de5 has research run40405537349,
job121908899835. Closed artifact12698121032 outer SHA256
bd87b584e3067a0a5b8008fcb2d4750dde652dd2e3692893a93390e6b481ae7b
was checked for bounded canonical unique ordinary ZIP members, CRC and exact
source/tree. The first configured-a-skills probe runs the existing exact
skills list --all command, exits0 with explicit null capture error, stdout1214
bytes and stderr149 bytes. Existing source parsing is blocked by the stderr
guard before the actual configured listing is validated.

The exact stderr bytes (SHA256627ecbbc677b2ca1e37a8b6894cd23619ddcf0ab7e1b1c16bdc460229c434234)
are the already admitted 52-byte ripgrep fallback followed by exactly these
ASCII bytes (each displayed line ends with one newline):

```text
Scheduling MCP context refresh...
Executing MCP context refresh...
MCP context refresh complete.
```

No later configured or untrusted
native research probe was observed in this failed run.

## Finite amendment after two independent plan GOs

In scripts/project_native_collectors/gemini.py retain the whole existing
exit/error/stream type checks first. Admit one additional exact stderr byte
string: the measured 149-byte concatenation, only for the exact
('skills', 'list', '--all') operation. Do not admit it for mcp, version/help,
another argv, partial/reordered/duplicated lines, other bytes or a prefix plus
extra diagnostics. Keep the existing empty-MCP sentinel and fallback-only
tuples unchanged. Preserve stdout bytes and existing strict skills parsing,
exact selected source/path/enablement checks, trust/exclusion controls and
post-command authored-file checks. No stderr stripping, broad pattern,
diagnostic normalization or auth/error acceptance is allowed.

In scripts/test_project_native_gemini.py add an independently stated literal
positive and mutated/truncated/CRLF/reordered/duplicated/appended/foreign-command
negative cases, plus nonzero exit, bool exit, missing/non-null error, wrong
stream types and authored-file mutation refusal. Exercise the actual full
inert collector with this tuple at configured-a-skills while retaining its
existing complete call inventory and false runtime/MCP/auth qualification.
Test that malformed/cross-project stdout still fails after the exact stderr
tuple is admitted; diagnostics alone cannot qualify source selection.

Scope is these two existing Python files and this plan. Do not change source
pins, host/profile, native invocation, 30-second command limit, collector
budget, trust/excluded MCP configuration, process cleanup, artifact admission,
case meanings or qualification requirements. No native binary runs locally.
This plan is separate from the provider call-part/wire-ID grammar correction.

## Validation, review and remaining proof

Run only the affected existing/new inert Gemini collector contracts and shared
research contracts needed to verify unchanged envelope admission, plus syntax
and whitespace checks. No Go/lint slot is needed. Independent code review
precedes root-owned publication to existing PR526; no new PR or release.

The future genuine run must actually execute all remaining A/B/user/untrusted
operations and preserve exact source-bound raw evidence before their cases can
pass. Inert success or this old failed capture does not establish new native
observations, a runtime lifecycle, MCP exchange, vendor authentication or model
behavior. All unproved qualification flags remain false. VS Code is excluded.


## Actual validation

Both independent plan reviews (root and CLI) approved SHA256
71a06dd903b535c632412a1f1a2c7301d8ade1783899c9486b8f9141d4f7994f
before implementation. The actual affected Gemini collector and shared research
inert modules pass all31 tests in0.388 seconds. Python AST and whitespace checks
pass. The positive literal independently matches the captured149 bytes and
SHA256; hostile command/byte/capture/source-path/authored-mutation cases remain
refused. No vendor or Go binary executed. Fresh genuine hosted observation is
still required for every remaining native probe.
