# Qoder hosted pre-code research implementation checkpoint

The research-only plan at `683eaea8` received independent R1/R2 GO reviews,
committed in `adcfffb2` before runner code. The original approved Qoder project
client plan and its native schema gate remain unchanged.

The new runner imports the existing matrix's require_hosted helper unchanged
and invokes it before output/probe paths, network acquisition or execution.
The fixed official 1.1.78 vendor archive hash and measured binary hash are
checked before execution. Actual compressed and expanded reads are bounded;
tar member count, names, types and exact single binary admission are checked
without archive-wide extraction. Native runtime receives only the existing
fresh-profile helper's variables plus literal PATH/TMPDIR/LANG values.

Every process receives DEVNULL stdin, fixed empty-project cwd, separate bounded
stdout/stderr files, a 30s deadline, child-enforced file size limits and process
group cleanup. Only the six plan-approved version/help/empty-list arguments
exist. The final summary records each exit, elapsed time, byte counts, hashes
and accepted/failed result. Any omitted/failed/unrecognized observation fails
the overall research. Guard refusal produces only a bounded diagnostic and
nonzero exit; it creates no requested output directory or summary. Failure
summaries apply only after hosted admission.

The separate workflow selects an actual GitHub-hosted ubuntu-24.04 runner,
read-only repository permissions, an exact source checkout with persist-credentials:false, no token passed to the native runtime, and artifact upload on
success/failure. Runtime egress isolation is not claimed. No installation
script, plugin install/uninstall, trust, authentication, model, hook or MCP
initialization command is present.

## Author verification

- Inert research contracts: **7 tests PASS 0.983s**. These include live child
  output limits, DEVNULL stdin, deadline termination of an actual inert child
  and descendant group, archive admission, pin-before-parse, fresh explicit
  runtime authority and exact version/empty-array rules.
- The direct guard-refusal test invokes the real imported guard without a
  hosted opt-in. No forged hosted variables or patched guard is used and no
  native client runs. The requested output parent remains absent.
- Existing matrix/native safety suite at this leaf's `c96`  base: **54 tests
  PASS 0.458s**. Existing matrix runner, guards and tests remain unchanged.
- Python syntax, workflow YAML parse/admission and git diff whitespace: PASS.

These results qualify only the research plumbing contracts. The genuine hosted
research has not run at this checkpoint. The retained task-host Bun abort134
logs do not establish successful native help. Even a future successful empty
list will not establish populated project record schema, exact ownership,
project installation, runtime discovery, skill execution, MCP initialization,
native restart or all-client E2E completion. Qoder registry mutation adapter
code remains on hold pending sufficient genuine native schema observations.

## Independent implementation closure

Both independent reviewers returned **GO** at the frozen runner checkpoint
`dfb262a80b37952a7156e37f5e53b787a90a3fd1`. Their committed reports record
independent seven-test inert safety passes (R1 0.934s, R2 0.923s); R2 also reran
the unchanged existing 54-test safety selection (0.421s). Neither reviewer
executed Qoder, acquired a native artifact, spoofed the hosted guard or claimed
native project qualification. The early research PR can now obtain genuine
hosted version/help/empty-list outputs for inspection. Project registry mutation
code and its stronger native schema/installation/runtime gates remain pending.
