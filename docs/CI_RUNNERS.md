# Linux CI runners

Trusted Linux CI jobs use the repository variable `CI_LINUX_RUNNER`, configured
as `nscloud-ubuntu-24.04-amd64-4x8` (Ubuntu 24.04, x86-64, 4 vCPU, 8 GB RAM).
Namespace GitHub Actions is already enabled for the account repositories.
The workspace shares 32 Linux vCPU / 64 GB RAM, allowing eight such jobs at once.

Matrix OS names remain unchanged: only Linux execution is routed. Native macOS
and Windows jobs keep their existing runners. Pull requests from forks keep
GitHub-hosted runners rather than running untrusted code on the managed pool.
Unset variables fall back to the original GitHub Ubuntu labels.

Where required, `CI_LINUX_ARM_RUNNER` selects
`nscloud-ubuntu-24.04-arm64-4x8`, and `CI_LINUX_2204_RUNNER` selects
`nscloud-ubuntu-22.04-amd64-4x8`. Do not route an ARM matrix leg to x86-64.
Native research using the explicit `RUNNER_ENVIRONMENT=github-hosted` admission
contract stays on GitHub-hosted runners; the migration does not spoof or weaken
that physical-host guarantee. Public qualification cells also retain the exact
runner IDs admitted by their provisioning manifest, including ARM64; generic
CI routing must not override those identities. Dedicated private-network runners and deployment
procedures are outside this
migration. OIDC/attestation signing jobs retain their original GitHub-hosted
runners. Existing release workflows that already use `CI_LINUX_RUNNER`
continue following that repository setting. Frozen VS Code workflows are unchanged.

To verify execution, inspect the job's labels and runner name in GitHub Actions
and the corresponding job in Namespace. A queued job alone is not proof of a
working runner. Validate an actual CI job to completion before declaring rollout
complete. Existing queued jobs retain their old routing; rerun only the current
required work after the workflow change reaches its branch. Preserve completed
checks and avoid duplicating active runs.

## Opt in one pull request to GitHub-hosted Linux

Maintainers can apply the `ci:github-hosted` label to a pull request before a
fresh synchronization push. The covered ordinary Linux amd64 checks then use
their original GitHub-hosted Ubuntu labels instead of `CI_LINUX_RUNNER`.
Jobs originally pinned to `ubuntu-24.04` keep that label; other covered jobs use
their original `ubuntu-latest` fallback. Job steps, check names, matrix values,
tool versions, permissions and required gates are unchanged. Inspect the actual
job allocation's runner labels and runner name to confirm where it executed;
a matrix value preserved for check-name compatibility is not provider evidence.

This opt-in does not change repository variables or route another pull request.
Fork behavior is unchanged. ARM and architecture-specific overrides, native
research physical-host requirements, public qualification runner identities,
frozen VS Code workflows and signing jobs keep their existing routing.

Apply the label **before** the push that starts the desired checks. Adding a
label does not reroute an existing queued job, and these workflows deliberately
have no `labeled` pull-request trigger. In particular, the packed/native
workflow keeps its unfiltered PR and dispatch triggers and its always-running
aggregate; only its lanes are gated (see below). A new
head must complete its actual checks; earlier successful results remain history.

Changing the shared lint workflow also triggers Core fast under its existing
path policy. Keep those additional checks and their dependent jobs; the same
per-PR opt-in covers their ordinary Linux amd64 execution.

## Pull request path policy

Pull requests skip lanes whose inputs did not change, so documentation and
landing work no longer queues Govulncheck, Polyglot Smoke, Terminal UI,
Coverage, Skills picker, lint, the Management SDK matrix or the authoring
native, packed and Milestone A lanes. Pushes to `main`, schedules and manual
dispatches keep their unfiltered triggers, so every merge commit still
produces the push-triggered checks the release workflow requires. Terminal UI
and both authoring workflows have no push trigger; Milestone A release
evidence comes from a manual dispatch. `repotests/ci_path_policy_test.go`
guards these rules.

The documentation and web surface is root Markdown, `docs/`, `website/`,
`landing/` and `assets/`. Every filter below that skips documentation names
exactly this list.

- `pr-changes.yml` is the one pull request classifier. It reads the pull
  request file list once and reports `lint`, `management_sdk` and
  `authoring`. Callers skip a job only on an explicit `false`. An unavailable
  or empty files API response for a non-empty diff, a 3,000+ file diff or a
  path with a control character marks every class true, and a classifier
  failure runs every gated job. Gated jobs wait for it, so it follows the
  pull request's own routing above (`CI_LINUX_RUNNER`, or GitHub-hosted for
  `ci:github-hosted` and forks) rather than adding a wait in a second queue.
  It only calls the GitHub API and never checks out pull request code, so the
  `CI_LIGHTWEIGHT_RUNNER` variable may move it to any pool; unset it to
  return to the default routing.
- `Required / test` runs on every ready pull request: repotests read
  `README.md`, `docs/`, `website/`, `landing/` and workflow files. `lint`
  skips without a Go, module, `.golangci.yml` or lint baseline change, and
  `management-sdk` without a `sdks/typescript/`, `contracts/management/`,
  generated DTO or root module change.
- Authoring native offline keeps its unfiltered pull request trigger and its
  always-running `Authoring / required native and packed` aggregate. Its
  native and packed lanes skip only when the classifier reports a
  documentation-only diff (`authoring=false`), and the aggregate then requires
  both lanes to be skipped and says so in a notice and the job summary;
  otherwise it requires both to succeed, as `scripts/check-packed-workflow.py`
  enforces. A green aggregate is native and packed evidence only when the lanes
  ran. A path that a Windows checkout
  rejects still runs the lanes, because they hold the remaining Windows pull
  request checkout for such a diff.
- Authoring Milestone A E2E ignores the documentation surface at workflow
  level. A job-level gate would add jobs to its dispatch runs, and Milestone A
  release admission binds a dispatch run with exactly its four jobs.
- Govulncheck runs only for Go sources, `go.mod`, `go.sum` and `go.work*`.
  CodeQL ignores Markdown and `docs/`. Docs ignores `landing/`.
- Polyglot Smoke, Terminal UI, Coverage and Skills picker ignore the
  documentation surface.
- Dependency Review stays unfiltered: the release workflow needs its success
  on the merged pull request head.

GitHub evaluates workflow path filters on the first 3,000 changed files only,
so a larger pull request can skip a filtered workflow. Workflows with a push
trigger still run every lane on the merge commit; Terminal UI and Milestone A
then run again on the next pull request that touches their inputs, or by
manual dispatch.

No branch protection or ruleset requires these checks today. If one is added,
require only checks from unfiltered workflows: a check from a skipped workflow
stays pending. If a newer merge cancelled the `main` push run of a release
candidate that came from a documentation-only pull request, its pull request
head has no Go checks to fall back on. Tag the newest `main` commit whose push
checks completed, or re-run the cancelled workflows only while no newer `main`
push run is in progress, because a re-run joins the same concurrency group.

## Run Pages on hosted runners at an exact source commit

The existing Pages workflow accepts a manual boolean `github_hosted` input,
defaulting to `false`, and an optional `expected_sha` string. Hosted manual runs
require a full lowercase 40-character `expected_sha`. Any supplied SHA must
match the workflow's immutable `GITHUB_SHA` before checkout or building; it is
checked again before deployment. A moved ref therefore fails closed instead of
silently publishing another source commit. Automatic pushes and ordinary manual
dispatches retain their existing runner selection and behavior.

The SHA comparison binds the selected source snapshot; it does not establish
merge status, replace successful CI, or bypass deployment-environment approval.

After merging, set `merge_sha` to the exact merged commit and dispatch from the
publication branch, for example:

```bash
gh workflow run docs-pages.yml --ref main \
  -F github_hosted=true -f expected_sha="$merge_sha"
```

Keep the existing `github-pages` concurrency group with
`cancel-in-progress: false`, the same build/artifact steps, permissions and
`github-pages` deployment environment. If a redundant automatic Pages run for
that **same merge commit** is queued ahead of the explicitly chosen hosted
dispatch, coordinate a handoff of only that queued run. Do not cancel active or
unrelated publication work, change repository-wide runner variables, or replace
the workflow's freshly built artifact with a local artifact.

Rollback: set `CI_LINUX_RUNNER` to the prior provider (`ubuntu-latest` for
review-router-ai, `ubicloud-standard-4` for universal-agent-plugins), or unset it
to restore each workflow's original GitHub-hosted label. Restore any architecture
specific overrides separately. No source or test logic changes are needed.
