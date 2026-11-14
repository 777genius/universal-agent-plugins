# Qoder exact auth-membership correction: validation

## Reviewed source and chronology

The membership addendum was approved before its implementation by independent
GPT-6.1 Sol xhigh reviewers `kimi_current_review` (R1) and
`native_current_review` (R2), at exact plan SHA256
`ff0d090da140c389e0690990ab6cd7440b8c1f2c92c008f2f28bb64fb0ff2b3f`.
The plan and review record were committed as `2b63d444f9cbb25be9fa23a475e80e50e0a5b22a`.

The correction was committed as `b42ff698deb486b69b611b9d2adb6ead6accebb9`.
Both independent reviewers read the complete helper, contracts and wiring and
returned exact-source CODE GO. Actual lint then identified one staticcheck QF1001
finding. Commit `83dd06d0bc3e932baf4f8de3d59333f167898e08` changes only the two
predicate lines using De Morgan's law. Both reviewers read that exact delta and
extended their CODE GO. Final code tree:
`c25f4a711379822b835b3e1e18f0bd6624cbcea4`.

The original metadata author's complete `39f1267d` history and tests remain
preserved. This correction's plan approvals do not backdate reviews of that
original implementation.

## Actual inert validation

The new direct regression was run against the original helper through a Go
source overlay. It failed as expected for all six unknown auth siblings and for
initial capture, demonstrating the early-baseline gap. It ran in 0.013 seconds
of package test time. The old helper was never installed over the working source.

On the final corrected source, the following gates actually passed:

- `go test -p=1 -count=1 -timeout=120s ./repotests -run '^(TestQoder|TestProjectNativeCLI)'`:
  PASS, 0.049 seconds of package test time.
- All enabled lint on the affected delta from `39f1267d`: zero issues.
- Unfiltered whole-repotests `revive,funlen,gocyclo,gocognit,dupl,depguard`:
  zero issues.
- Formatting and whitespace checks: clean.

Go 1.26.10 and golangci-lint 2.13.2 ran with offline dependency resolution. No
vendor executable, hosted environment simulation or native qualification ran
locally. The reviewers' static code approvals are separate from these root-run
tests.

| Actual log | SHA256 |
| --- | --- |
| Original helper counterexample | `564c195467b4282a497b9c9a4aa0c39c01cae30f4f9d68c4a5f721642dac7d5d` |
| Final focused Go | `0df67033922854846ec5ae3c86b30b637afb41defd26348a039098ec250430c9` |
| Final affected lint | `e92606b0bf483111dff0a120c315ea165821348f31365020e2468a0059095c47` |
| Final unfiltered six-linter gate | `e92606b0bf483111dff0a120c315ea165821348f31365020e2468a0059095c47` |

## Preserved evidence limits

The fixture now rejects any direct `.auth` entry other than `machine_id` and
`.credential-transaction`, including before the first full-profile snapshot.
It enumerates bounded names through held directory authority and never reads
unknown bodies. Existing file ownership, byte, mode, inode, link, directory and
full-profile checks remain required.

This is a fixture acceptance correction. All five qualification flags remain
false until genuine, current-source hosted evidence establishes applicable
claims. The earlier negative captures, their omissions and the absence of account
or model qualification remain unchanged. No new MCP support, VS Code work, merge
of an unqualified leaf, release or vendor-binary/source equivalence is claimed.
