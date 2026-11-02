# Kimi source identity expectation checks

The approved plan is `KIMI_PROJECT_NATIVE_SOURCE_IDENTITY_EXPECTATIONS_PLAN.md`,
SHA256 `0248a4f6a26077468b1b2c7f6ece24898c932d784d0c0909ed485d00709d95be`.
CLI and desktop independently reviewed the complete plan and full pinned native
sources before code. Both also gave independent exact-code GOs on the same
five source hashes below. Their conditional actual-check requirements are now
closed. Root remains the only external publisher.

Local plan commit a4aa9f5c is followed by two separate corrections:
93758271 authors only the exact expected plugin-qualified user skill name;
c73a696e requires the exact fresh first-turn integer0. Their code tree is
`4d9e6e76d1bdc6f25c9f7ba2cc8ee3babd13044d`.

| Exact source | SHA256 |
| --- | --- |
| repotests/agentplugins_kimi_project_native_lifecycle_test.go |74540783f24afc434f7851bc147e4122f6e841f08133530610c5ffdbefafa9b1|
| repotests/agentplugins_kimi_project_native_skill_name_test.go |5469b6b4d7d7c0aead27924d347eb0dfe668016e8331ad96472097c6fb5a9bfa|
| scripts/test_project_native_kimi.py |976c85ac1817c2bd0d869c4eaf7f55443c0d6324d6d5c44fb48eccf63093ebb4|
| scripts/project_native_collectors/kimi_attempt.py |bd49f1777c44bf39aca68c1f391e422ddbe4bcf3a21f6c6ed77dc8cb0e6c365a|
| scripts/test_project_native_kimi_attempt.py |eaae849851de8415ea1bc5db0f348df3d63d22761ef3a0ed65f55440765f2f9a|

Actual offline checks on those final bytes, 2026-10-27:

- Focused Go `^TestKimiProject|^TestKimiSingleTool`: PASS0.255s, including all
  four actual Loader/Planner/Stager/adapter/proof user/A/B/C request-expectation
  contracts. These execute inert owned files, not a Kimi/native installer.
- All configured enabled linters on affected lines from plan a4aa9f5c:
  zero issues.
- Unfiltered whole-file revive/funlen/gocyclo/gocognit/dupl/depguard on the
  complete repotests package: zero total issues, not merely zero selected-file
  issues. Report `/tmp/uap-shell-kimi-expectations-whole.json`.
- Complete Kimi collector, single-tool and shared supervisor Python suite:
  59 tests PASS17.884s. The actual observer seam accepts firstturn0, sends
  exactly one selected decision with no remembered scope, and keeps native
  capture/history unqualified; laterturn1 stops before that action and raw
  HTTP remains quarantined. Existing schema/identity/privacy controls pass.
- Python AST, gofmt and git diff checks are clean.

Go1.26.10 and golangci-lint2.13.2 used the recovered official exact assets,
GOPROXY/GOSUMDB off, approved private shared-memory caches, GOMAXPROCS2,
GOFLAGS -p=1 and GOMEMLIMIT500MiB, in the explicitly coordinated exclusive
slot. The slot was released after checks; no process remains.

Current hosted source91a remains failed in both installed lanes at the
observed skills-a/approval validation boundaries. Its response values were
quarantined, so these corrections remain source-proven observer fixes, not
assertions about uncaptured response bodies. Fresh genuine hosted component
and single-tool runs are mandatory after publication. The single-tool native
history review gate and all account/model qualification limits remain intact.
The alternative finite-refusal diagnostic plan is retained outside the code
checkout and is not implemented or included in this candidate.
