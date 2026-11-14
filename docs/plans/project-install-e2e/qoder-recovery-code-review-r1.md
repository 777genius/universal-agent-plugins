# Fresh recovered capture implementation R1

Reviewer finish_sources_vscode_cline. **Final GO** atf3c0d3f927d3bf10b62d875ccb7088ee3681a358.
Full production review at55d57fc7 followed two fresh pre-code GOs atfce7217b.
Nonblocking cap-plus-one reads/at-most-cap writes, separate logs, draining
deadline, leader-exit/error/interruption group cleanup/five-second reap and
descriptor closure match the plan. Only RLIMIT_CORE remains. Literal stderr
byte refusal covers all six probes including whitespace. Pins/acquisition/env/
six argv/real guard are unchanged; baseline function AST comparison verified.

Author's fresh capture6 PASS2.284s/Qoder7 PASS0.938s initially supported source.
R2 then observed the preserved0.2s inert deadline expire before actual startup
under load. Final narrow delta changes only test timing to1s/2s/2.1s plus an
honest failure record; startup/no-survival assertions and production30s/caps
remain. Author's fresh Qoder7 PASS3.410s/capture6 PASS2.449s close it. No blocker,
native or Go process by reviewer. This approves recovered bytes, never native
runtime; failed authentic phase one and phase-two gate are preserved.
