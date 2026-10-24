# Cline hosted pre-code research plan independent review R1

Reviewer: codex_claude, 2026-10-23. Reviewed the initial a4cf690f plan and the
frozen corrective `8b99e838f8a11645b2ffc2254f41b555f6253ad8` in
work/uap-project-cline. No runner or adapter code exists in this supplement.

## Decision: GO at corrected 8b99e838

The automatic-updater finding below is closed before source. The plan approves
only bounded genuine read-only configuration/skill observations in the actual
disposable hosted environment, never ordinary project MCP startup or runtime
qualification. Original independently approved client plans remain in force.

## R1 finding: startup updater must be disabled explicitly

Independently fetched official cli-v3.0.73 tagged main.ts, commands/config.ts,
MCP config-loader.ts and commands/update.ts through the read-only GitHub
connector after public raw-page retrieval failed. runCli calls
`autoUpdateOnStartup()` before argument parsing. That function may check a new
version and arrange a deferred detached shell updater even for short commands;
it explicitly returns before that work when CLINE_NO_AUTO_UPDATE equals1.
Fresh profile and an explicit ordinary environment alone cannot guarantee the
plan's no-updater boundary.

Closed in8b99e838: every Cline process must receive literal
CLINE_NO_AUTO_UPDATE=1 from the runner's explicit environment, including
version/help and both config branches. Tests must reject omission, removal or
an observation-specific override. The plan records the exact tagged source
basis and no longer relies on incidental installation classification to avoid
an updater. No updater/installer execution is approved.

## Other reviewed boundaries

- Exact official npm version, archive SHA512 and measured binary count/hash are
  bound before execution. Compressed/expanded bytes, tar members, portable names
  and types are bounded; only the exact regular package/bin/cline is written
  to a fixed executable path. Other assets are metadata, never extracted or
  executed. Earlier genuine task-host abort134 logs remain failure evidence.
- Actual unchanged require_hosted runs before output/temp/acquisition/process
  effects. Guard refusal creates no output parent/summary. Hosted read-only
  permissions, exact source checkout and credential-free literal environments
  preserve account boundaries; tests cannot forge or patch hosted admission.
- Reusing the reviewed read/process, total-budget and descriptor-relative
  inventory helpers avoids another native guard. Eight-minute collection plus
  ten-minute job, live child file caps/deadlines/group cleanup, bounded no-follow
  inventories and post-admission failure summaries are explicit requirements.
- Text-only skills and disabled remote loopback-port9 markers contain no stdio,
  hooks, executable/plugin agent or credentials. Native command selection is
  finite: version/help, explicit config mcp JSON and config skills JSON. Fixed
  no-auto-approval and cwd arguments prevent accidental implicit task intent.
  Tagged config code has explicit inspection branches before the default task;
  config tools/interactive/default session paths remain excluded.
- Explicit MCP-override cases observe process-selected configuration, including
  a later default process and B with A's selected file; they cannot prove
  automatic workspace discovery. Populated JSON is recorded before interpreting
  fields; exact version/strict JSON and missing/error/overflow/deadline failures
  are explicit, while source/runtime qualification remain false pending human
  review. No model, MCP connection/tool, trust, native registry mutation or
  main/release operation is approved.

No additional Cline native process ran during this plan review. The reviewer's
earlier assigned version/help probes produced separately retained Bun failures;
those failures and static tagged source are not relabeled as genuine grammar,
project discovery or runtime success. Commit both independent GO reports before
adding source, then obtain the required independent implementation reviews.
