# Qoder hosted pre-code research plan independent review R2

Reviewer: cursor_clients, 2026-10-23. Reviewed
`work/uap-project-qoder/docs/plans/project-install-e2e/qoder-hosted-research-plan.md`
before runner or adapter implementation. Inspected the unchanged imported
require_hosted/profile_environment helpers, official manifest/provenance
snapshots and current official plugin/reference documentation.

Final plan refresh: `683eaea8fb6aa370d0382b528e3e3b423212d050` adds explicit
DEVNULL stdin and clarifies that failure summaries are written only after
hosted admission. Guard rejection creates no output/probe directory, acquires
no artifact and launches no process. Those clarifications preserve this GO.

## Decision: GO for the bounded research plan

No plan correctness blocker found. This approves implementing the separately
guarded version/help/empty-list research workflow only. It does not approve
Qoder registry mutations or qualify a project installation/runtime.

- Exact versioned HTTPS acquisition is bound to the vendor manifest digest,
  compressed read limit, tar member/type/path limits, declared binary size and
  the already measured binary digest before execution. It materializes one
  fixed binary path without vendor installation scripts or archive-wide
  extraction. The supplied manifest/provenance agree on version 1.1.78 and
  the pinned Linux x64 archive/binary hashes.
- The script imports and calls the actual unchanged hosted guard before
  directories, acquisition or execution. The workflow is disposable
  GitHub-hosted Linux amd64 only, with fixed source, read-only permissions and
  no persisted checkout credentials. Tests do not spoof hosted variables or
  patch/weaken the guard to run native code on the task host.
- A literal fresh profile/environment and fixed empty project cwd isolate
  read-only probes from user/native configuration. No workflow token,
  credential, proxy, model key or permission/trust flag reaches Qoder. Runtime
  egress isolation is explicitly unclaimed. Existing task-host private-proc
  launcher failures and empty-proc Bun aborts remain failure evidence.
- The six allowlisted commands are only version, plugin/help forms and an
  empty registry list. Probe deadlines, separate bounded stdout/stderr files,
  whole-process-group termination, total workflow timeout and per-probe logs,
  hashes and failure summaries make the result reviewable. Implementation
  review must verify those output bounds remain enforced while a process is
  running, including descendant termination on timeout/limit breach; successful
  post-exit truncation alone would not satisfy this plan.
- Version validation requires the exact token, and empty-list validation
  accepts exactly a JSON empty array. Nonzero exits, signals, malformed JSON,
  unexpected/nonempty shapes, timeout, output-limit failure and skipped probes
  cannot be relabeled as successful empty evidence. Artifact upload runs on
  failure as well as success.
- Current official reference independently confirms that list JSON reports
  recorded scopes, project/local rows carry projectPath, and effective enabled
  status belongs to command cwd. The plan correctly does not infer a populated
  schema or project ownership from an empty list. Genuine sufficient schema
  evidence remains a later precondition for adapter mutation code.

Meaningful inert implementation tests and a direct real-guard refusal check
remain mandatory before publication. Commit both independent plan/fix GO
records before runner code. The eventual hosted evidence must be interpreted
on its actual measured outputs, and it cannot replace the separate native
project E2E qualification.
