# Hosted fixture portability correction evidence

Actual46b4678 hosted Core fast run40145167029 exposed fixture setup defects in
macOS job121024834276 and Windows job121024834278; terminal jobs121024833733
and121024833729 repeated Claude/Grok fixture failures. Production canonical path,
strict native parsing and bounded filesystem discovery guards remain unchanged.

## Plans and independent code review

APM and Skillshare independently gave R1 architecture GOs on
b24201c4d33e6720d6212180863a122168f35c8d. Their R2 GOs covered fdd3f5e:
six finite fixture files, exact changed JSON field, literal traversal negatives,
129 physically distinct ASCII entries and the specific unchanged scanner limit,
one platform-absolute inert executable, and an open-handle Windows alias recheck
with independent SameFile proof and canonical positives before and after.

Both independently gave CODE GO on28ec7ba7d8ba5d73fe4e8e853cf47987cd7bb599,
treecba91337b34675fc241dda3245b8538eec25c95b, after reading all six changed
files and the Windows anchored opener. No production source, parser, sharing
flags, budget, platform skip, workflow selector or assertion was weakened.
The existing before-leaf deterministic ancestor replacement regression remains.

Root and APM independently gave CODE GO on the separate Claude three-file
canonical seed correction1a93c8d481b9009902f3985a70270b76cb40b33d.
The positive existing project/other/SDK/CLI fixture roots are resolved before
constructing authority. The authored alias/../ traversal and physical escape
proof remain literal, and exact foreign/user/B and result assertions remain.

## Observed checks

At exact fixture source28ec7ba7, verified Go1.26.10:

- Codex provider project contracts:PASS0.720s.
- Kimi/Kiro public SDK stdio/data/repair/restart/removal contracts:PASS1.082s.
- Grok strict native shape and finite collision/budget contracts:PASS0.151s.
- golangci-lint2.13.2 changed-source gate from fdd3f5e for provider/installer/Grok
  and public CLI packages:0 issues.
- Authored whitespace check:PASS.
- Windows amd64 Grok test binary compiled successfully,7,588,864bytes,
  SHA2564c454aea3923d26ab63da25924b18e7e306d2e43e163810d76f596597f0145a3.
  It was not executed here; this is compile evidence only.

The exact public TLS Git Grok lifecycle was attempted once and failed before
source acquisition with 'initialize private Git transport workspace failed'.
This executor lacks /proc/self/task/<live-thread>/children. The existing actual
unsupported-preflight/no-launch regression was then run, rather than modifying
the runner: TestOrdinaryActualUnsupportedPreflightCannotLaunch PASS0.011s.
The local TLS Git journey is not counted as passed. Actual hosted Linux/macOS/
Windows lifecycle results remain required; there is no local skip or bypass.

At exact Claude fixture source1a93c8d, its author observed client traversal
PASS0.041s, two public SDK lifecycle casesPASS1.879s, and CLI grouped update
PASS0.546s. Independent critics performed static reviews, not duplicate runs.

These results qualify the named fixture revisions. The later complete assembly,
including Qoder and remaining client work, requires its own integration checks.
No native vendor, model, MCP server, authentication or trust operation was
performed by these fixture tests.
