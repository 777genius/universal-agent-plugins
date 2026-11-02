# Kimi current repeat-add and approval refusal validation

## Reviewed source and authority

The source for the current genuine failures is PR524 commit
`03d7c243129c98ecdc4cbb99bfe1a47536a556bf`, tree
`de7a5565545f67e26e7e3e101b843f728e3876b1`, runtime
[40553154543](https://github.com/777genius/universal-agent-plugins/actions/runs/40553154543).
The component and single-tool jobs both failed; all five qualification flags
remain false in both native and frontend captures. The complete current API
check set was 44 successful, two native failures and one skipped check.

Both exact proposals received root R1, independent desktop R2 and a separate
native CLI plan review before implementation:

| Proposal | Approved SHA256 |
| --- | --- |
| Repeated-add direct result | `c709b9d3390263eb602ac1781090b1c0b8f48811c10c0c6696ee8aeff2d38e62` |
| Approval refusal finite facts | `e841ca2be7330be91b787f2a4a015c6a0a329b529cf16a55404921d40c7bbcf1` |

The changes remain separate commits:
`4b420105115d66b6b9dfabb54b3d777611d4ddc3` implements the exact repeated-add
result contract; `a58aba727c613a2f2b5f129f0acfecfb20c234c7` implements only
failure diagnostic facts. Native CLI and desktop reviewers independently
approved the first code at tree `ce7d293580faf433844c9b64f6b52456a04b3199`;
the desktop code GO covered the same executable bytes before a comment-only
truthfulness correction. Root, native CLI and desktop reviewers independently
approved the second at tree `bcee7cdf3c5b509676118b6346ee40f7999617e4`.
Final validation notes changed no executable bytes.

The strict approval function's AST is identical to the published function
except for its private name. Its exact turn0, identity, permission and schema
predicate order remains intact. ResponseSchemaRefusal retains its exact
object/class and existing graceful branch. The diagnostic grants no approval
or tool authority and exports no native values, identifiers or hashes.

## Genuine failed captures retained unchanged

All four current artifacts were freshly downloaded again after the interrupted
publication session. Size and SHA256 matched the current GitHub API receipts;
every ZIP byte equaled the previously reviewed copy. Nested CRCs, closed member
inventories, extracted bytes and frontend stream joins remain valid.

| Artifact | Members | Outer ZIP SHA256 |
| --- | ---: | --- |
| Component native 12759050559 | 34 | `01d59c89fe8124ec17d2ab08f5239e746e23d2c0db7876e34ba482885cef4ffb` |
| Component frontend 12759450014 | 25 | `b8163211655fe5215607758975694968dd05bc67c24c3b26a014b599a7ec9aae` |
| Single-tool native 12756894604 | 23 | `80fdf584518a9f2d879c61ce0eb12eb8b4b6adcbaa1fc1028c188399406422ac` |
| Single-tool frontend 12758701279 | 25 | `443be84f49ffa69c4e789d0cd33b7bd27c71d45ae3f6571af887486c69c3cf80` |

The component capture passes its installed observation, then refuses the
successful direct repeat-add result in the fixture's old array-only parser.
The single-tool capture refuses at approvals/validate/validation before any
approved tools/call. Its raw approval values remain quarantined and unobserved;
this correction does not claim which remaining predicate differs.

## Current-base alignment and final inert gates

Commit `29f93abe9739d65336a39a81fc082ce238b5dfa1`, tree
`bbf6199df0628636c81bb719a80005a07c90ab3f`, is a true merge of the reviewed
candidate and current PR520 base `a412ba5e137fdcb7e004393229b323211ed746e1`.
All six Kimi code/test files below remain byte-identical to the reviewed leaf.
The only manual conflict retained both append-only consolidated-plan blocks
verbatim. Independent native CLI review verified the full nonconflicting tree
union, every nonblank parent plan line in order and the automatic detector-test union.
The shared runtime Python source/tests are byte-identical to the Cline-qualified
base. A final documentation-only cleanup removes one inherited extra blank EOF
from KIMI_PROJECT_NATIVE_HOST_MARKER_HANDOFF_PLAN.md.

| Check | Actual result |
| --- | --- |
| Focused Kimi/proof/single-tool/shared native CLI Go, before base merge | PASS 0.392s |
| Actual current repeat-add stdout against separately held first-add identities | PASS 0.009s |
| Complete Kimi collector/single-tool/supervisor Python | 65 tests PASS 17.727s |
| Focused Go after true base merge | PASS 0.620s |
| Kimi/supervisor/shared-runtime Python after true base merge | 92 tests PASS 23.859s |
| All configured root linters on affected lines versus actual a412ba5e | Zero issues |
| Unfiltered root revive/funlen/gocyclo/gocognit/dupl/depguard | Zero issues |
| Strict approval AST comparison, changed Python AST, gofmt and final diff checks | Clean |

The raw replay was a temporary inert Go control; it was removed before final
lint. It also confirms that the unchanged shared array parser still refuses
the actual direct result. No raw capture is committed. Local child processes
were inert test processes; no vendor/client or public installer ran locally.
Go1.26.10 and golangci-lint2.13.2 ran in a coordinated exclusive slot with
GOPROXY/GOSUMDB off, the existing module/cache paths and the unchanged linter
configuration. Python used the existing complete inert suites. Final lint
receipts are `/tmp/uap-kimi-postbase-affected-lint.json` and
`/tmp/uap-kimi-postbase-whole-six-lint.json`.

| Reviewed code/test file | SHA256 |
| --- | --- |
| repotests/agentplugins_kimi_project_native_lifecycle_test.go | `b6cc4cf42b1e425f0ce9b801d5475fdc9810b485daed983466cf24cc63e844ba` |
| repotests/agentplugins_kimi_project_native_repeat_add_test.go | `bb0c1d30455efbccb32fd19290909dc64d275d995ccbfe4258b75547bee5e130` |
| scripts/project_native_collectors/kimi.py | `e772cb0d2c2b7a3bdc290f061d1dab8310a1fe7f9a3e35ef02751e8aa91448e9` |
| scripts/project_native_collectors/kimi_attempt.py | `420509c0101c708ac26fa836f23753441381cbf926e368fa18d8e2c61f442000` |
| scripts/project_native_collectors/kimi_approval_facts.py | `fe829a905c18ee33ee08f6266d89b00b5c852b8a3efe7727860de2d638988ee7` |
| scripts/test_project_native_kimi_attempt.py | `c3b6c38108516f3d4d037c2c8a18e260236c2a953a5644884c190cd4c51e93c6` |

## Remaining native gates

Root alone publishes after a fresh expected-head check. A fresh genuine hosted
component run must establish the later update, repair, restart, removal and
preservation stages. The separate single-tool lane still requires its native
approval/history evidence and a separately reviewed finite history grammar;
its deliberate final failed-capture boundary is unchanged. No account/model,
release or VS Code qualification is added. The original 900s Kimi lifecycle
suite, admission, pins, ownership, provider and cleanup limits are preserved.
Historical failed artifacts are never relabeled as results of this candidate.
