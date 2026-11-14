# Trusted VS Code artifact handoff

Owner authorized a narrow correction to the trusted research artifact boundary.
The previous workflow scanned two pathname-based ZIP locations with `always()`;
a publisher exception or timeout could leave a ZIP there, and an output-root
replacement could defeat an inner marker or quarantine. No native success or
private-view authority follows from this correction.

The frontend retains original output, frontend and native directory descriptors
through an ExitStack. A closed publication helper clears its ZIP binding BEFORE
any publisher attempt or quarantine. Only a completed publisher or an already
negative capture with literal `child_reaped is True` and
`archive_failure_allowed is True` can acquire a binding. The initial prepared
publisher attempt is recorded before calling it; finally does not retry that
attempt after an incomplete result or exception. Invalid or changed authority
emits no handoff, even when quarantine cannot safely act.

The only successful stdout record is a <=16KiB closed receipt with source
commit/tree, semantic collection status, original directory identities and
nullable frontend/native ZIP identities, lengths and SHA256. The frontend
holds directory authority, opens upload/ZIP descriptor-relative with NOFOLLOW,
bounds the read, and rechecks held and linked identities after reading. No
receipt-provided pathname is interpreted. A missing receipt grants no upload.

The existing Go wrapper forwards a unique fixed-prefix receipt only after its
ordinary process runner proves cleanup, its context remains valid and complete
stdout capture is available. Python semantic exit 2 may carry an admitted
negative artifact; cleanup, capture, cancellation and other exit errors cannot.
Closed JSON/source/type validation precedes forwarding. It still fails the
native test for exit 2 or invalid native evidence.

The trusted workflow parent keeps run_probe stdout in memory, rejects timeout,
missing or duplicate handoffs, verifies the independently expected source and
opens the original output/ZIP locations without aliases. It checks held/path
identity and SHA while copying admitted bytes into a fresh private sibling
staging directory. Both files must complete within one five-second copy budget
before any step output is written. Uploads require the collection step's success
and an explicit nonempty staged pathname; they never scan the original root.
A separate assertion AFTER uploads requires actual native test success, so a
successfully collected negative diagnostic keeps the job failed. Partially
written GITHUB_OUTPUT cannot authorize upload after a failed collection step.

Owned edits: scripts/research-vscode-project.py,
scripts/test_vscode_project_research.py,
repotests/agentplugins_vscode_project_native_gateway_test.go,
.github/workflows/vscode-project-native.yml and this plan. Existing native
840/870/880/900/930/980 budgets and publisher work/reap budgets are preserved.
The separate private-view author supplies only the authorized trusted launch
seam: fixed unshare --mount --propagation private before the same controller,
and original_mount_namespace in its sealed request. This plan does not claim
that private-view execution or native qualification has passed.

Inert checks cover actual fork/ZIP publication, late errors after ZIP creation,
literal cleanup flags, quarantine failure, original-root replacement, changed
ZIPs, closed receipt source/types, full Go capture/cleanup gate and copying to
fresh staging. Actual exact-source CI remains required before merge; local
checks cannot qualify native editor stages.
