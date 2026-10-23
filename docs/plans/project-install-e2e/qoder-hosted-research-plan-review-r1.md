# Qoder hosted research plan — independent R1

Reviewed frozen plan `683eaea8fb6aa370d0382b528e3e3b423212d050` in `work/uap-project-qoder` on 2026-10-23. Reviewer did not author the plan or runner. **GO for the bounded research implementation.** No runner source or adapter code was present for this review.

The plan preserves the imported, unchanged `require_hosted('linux-amd64')` as the first admission step before acquisition, output/probe directories, or native execution. Its explicit GitHub-hosted Linux admission is retained; no local hosted-variable spoofing or weakened guard is allowed. The separate read-only workflow grants only contents read and does not pass checkout credentials or workflow/model tokens into the fresh native environment.

Acquisition is restricted to the exact vendor HTTPS archive and both recorded archive/binary SHA256 values, with bounded compressed reads, tar member/type/path/size checks, no general extraction, and a fixed private binary path. The native command allowlist contains only version, help, and an empty-profile JSON list. There is no plugin install/uninstall, payload execution, trust/auth/model/MCP action, or claim of runtime network isolation. An empty list cannot establish a nonempty schema or qualify project mutation/native installation.

Two R1 clarifications were requested and are now explicit in the frozen plan:

- Every child has `stdin=DEVNULL`, bounded output files and deadline/process-group cleanup, so read-only probes cannot hang on interactive input.
- Failure summaries are written only after hosted admission. Guard rejection emits a bounded diagnostic/nonzero exit and creates no output/probe directory, acquires no bytes, and launches no native process.

The stated tests use inert injected operations and the real guard refusal; they prohibit monkeypatching/weaking the guard or executing Qoder on the task host. The author's additional expanded-gzip bound and live child file-size enforcement strengthen the recorded resource limits without broadening runtime authority. Both independent plan GO reports must be committed before runner code, and implementation review and genuine hosted evidence remain separate gates. This review executed no Qoder process.
