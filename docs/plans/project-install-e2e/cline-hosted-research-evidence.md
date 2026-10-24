# Cline hosted research collector — source checkpoint

2026-10-23. This checkpoint implements only the separately reviewed
`cline-hosted-research-plan.md`. Both final independent plan GO reports were
committed in `99013fbc` before collector source. The merge at `5e240cd3` reuses
the reviewed Grok/Qoder utilities from `514cca44`; the native matrix guard,
shared process/read utility and either existing collector remain byte-identical
to that reviewed checkpoint.

## Implemented boundary

The separate Ubuntu 24.04 workflow checks out the exact PR head with persisted
credentials disabled, contents-read permission, pinned actions and a ten-minute
deadline. The collector calls the actual hosted admission guard before any
requested output path, acquisition or native process. Only the admitted job
can acquire the exact pinned official Cline 3.0.73 archive, verify its registry
SHA512 and the exact binary size/SHA256, and write the one fixed executable.
Archive members are inspected under byte/count/path/type bounds; no asset,
installer or package script is materialized or run.

The eight-minute total budget, bounded no-follow inventory and unchanged
DEVNULL/no-shell/process-group/file-limit helper apply throughout. Every Cline
process gets a fresh explicit profile environment and literal
`CLINE_NO_AUTO_UPDATE=1`, including help/version. The fixed twelve probes use
only help/version or read-only config MCP/skills inspection, literal
`--auto-approve false`, selected cwd and known disposable overrides. Fixtures
contain only plain skill text and disabled remote loopback registrations.
Original fixture bytes must survive each probe, and native-created state is
reported separately. Failed, missing, ambiguous or over-limit output cannot
become complete collection evidence.

## Local verification

- All 23 inert research tests passed in 1.108s: eight Cline tests plus the
  existing eight Grok and seven Qoder tests. The Cline suite exercises finite
  argv/cwd/override selection, mandatory updater suppression, pin/member/output
  admission, text-only fixtures, changed-baseline refusal, strict result
  classification and the actual zero-output hosted-guard refusal.
- All 54 existing native-client safety tests passed in 0.439s without changing
  their source or the actual native guard.
- Python compilation, workflow YAML parsing and `git diff --check` passed.
- No Cline binary, installer, session, trust/auth action, model, MCP connection
  or payload was executed during this verification. Mocked process calls in
  the collector tests are expressly inert and do not qualify native behavior.

## Evidence still required

The earlier genuine local Cline attempts reached Bun 1.4.2 and aborted 134;
their provenance and failure logs remain the record. No successful local
grammar, runtime schema or project discovery is claimed. A genuine admitted
hosted run and independent interpretation of its populated marker/path and
process-override output are still required before the Cline project adapter.
The collector always leaves `project_source_qualified` and
`project_runtime_qualified` false. Even successful collection would not prove
ordinary project MCP startup, UAP project installation/ownership, trust,
enabled skill execution, MCP initialization/tool output or native restart.

Both independent implementation reviews are now **GO** for source checkpoint
`b4946a86d51e47f1d9326a8ff4431ee90c95e5a1`; their complete reports are committed
beside this evidence note. R1 independently passed 23 research and 54 unchanged
native safety tests; R2 independently passed the same suites and rechecked the
exact tagged updater/config dispatch. Neither reviewer executed or acquired
Cline. Root may publish this separate stacked research checkpoint to collect
the missing actual hosted observations. Publication and all remote merges
remain root-owned; the production adapter and genuine native outcomes remain
pending the artifact interpretation above.

## Historical hosted observation recovered on 2026-10-24

A genuine successful earlier run at exact source `bb8c89bf62542096a0af3aff26f21a16fee4d595` was recovered and independently interpreted by the implementing worker and root. The raw text artifact, verified provenance and precise source-closure differences are preserved in [the hosted observation record](../../evidence/project-scope-research/cline-3.0.73-linux-amd64-40017958131/README.md). Positive exact skill paths and explicit MCP override attribution close the missing pre-code observation for a [concrete post-observation adapter plan](cline-post-observation-plan.md). This does not replace a current-source UAP native gate, establish automatic project MCP startup or alter the original artifact's false qualification flags.
