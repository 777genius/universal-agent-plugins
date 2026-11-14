# Supported hosted Linux bootstrap runtime result

> **Historical snapshot.** The observations and qualifications below belong to
> their recorded source. The [original repository directory](https://github.com/777genius/universal-agent-plugins/tree/3b93148c356fc5db6fe101d81d943160a5176b6d/docs/evidence/process-containment/linux-proc-children-unavailable/hosted-bootstrap-40143431447) preserves the
> files present before raw-output cleanup. Adjacent metadata and source remain;
> original manifests describe that snapshot, not the current directory inventory.
> Retrieve removed raw files there; see the [project evidence index](../../../PROJECT_INSTALLATION.md).

[Authoring native offline40143431447, Linux amd64 job121019719395](https://github.com/777genius/universal-agent-plugins/actions/runs/40143431447/job/121019719395) succeeded on committed source1bc5b898d8085e707fb405ed0c8f84ca2454157f, treee6f411497b048f54b8bd2d8ac4d3603710a2e57a. The artifact's head.txt and original test records match this source. This source contains the independently reviewed ordinary-containment prerequisite and helper-dispatch correction.

The three reviewed real bootstrap runtime tests each **ran and passed**, with no bootstrap package skip/failure: captured staging plus detached setsid containment0.08s, locked command plus sanitized environment0.07s, and descendant cancellation0.04s. The full bootstrap package passed0.318s. These are actual supported-host process assertions, distinct from the local unsupported prerequisite SKIPs preserved in the parent directory. No deadline or assertion was relaxed.

The original archive is12579492503 (109006bytes, SHA256078da3d0930e82647a0ed78139f423932621edc6eff5eaad2a5ba0036af6795e). Its length, hash and CRC were independently verified after read-only download. [bootstrap-go-test.jsonl](https://github.com/777genius/universal-agent-plugins/blob/3b93148c356fc5db6fe101d81d943160a5176b6d/docs/evidence/process-containment/linux-proc-children-unavailable/hosted-bootstrap-40143431447/bootstrap-go-test.jsonl) contains every original byte line for the exact bootstrap package, preserving order; `provenance.json` records the extraction predicate, full source JSONL hash/length and counts. `archive-members.json` identifies every original archive member. Original metadata/summary/build descriptions are preserved in the immutable snapshot; selected summaries remain adjacent. Auxiliary full-package JSONL and discovery remain in the identified original artifact.

The workflow's exact package inventory excludes install/integrationctl/adapters/process, so this result does **not** qualify the separate nineteen process runtime tests. The unchanged Required/test workflow for main-base PRs invokes make test-required; Makefile runs ./cli/... (bootstrap) and ./install/integrationctl/... (process) without a test-name filter. The actual current-source full lane must still demonstrate its own supported-host result. No new workflow, selector or retry was added for this evidence. Other authoring summary facts, including one unrelated optional socket-fixture skip, are preserved rather than promoted to broader qualification.

No native vendor command ran locally during this evidence retrieval. This result does not establish native-client discovery, MCP/model/auth/trust operation or release qualification.
