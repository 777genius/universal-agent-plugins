# Enterprise policy retained-evidence follow-up

Tracking: [#1148](https://github.com/777genius/universal-agent-plugins/issues/1148),
following merged PRs [#1130](https://github.com/777genius/universal-agent-plugins/pull/1130)
and [#1142](https://github.com/777genius/universal-agent-plugins/pull/1142).
This is a source/evidence record, not release authorization or binary qualification.

## Fixed auditor boundary

The original independent auditor at
`05c8051785ea3f5f8af0a29a8881997f0ccbd5a1:.github/r39-native-audit/native_audit.py`
(Git blob `95e43720d4a8a2344a2242cc62adb8ae69b72c89`) expected
`pluginId == ""` for selected local Codex Skills. The pinned Codex 0.162.0
responses instead contain an explicit JSON `null`. The existing native Go
decoder reads that value into a string's zero value; the independent raw JSON
auditor must preserve the wire representation.

`scripts/native_client_collection_evidence.py` preserves that offline auditor
with only its provenance and this predicate corrected. It requires explicit
null for this pinned historical protocol. Missing, empty, foreign or non-string
non-null IDs are refused. Enabled status, repository scope, exact project paths,
unique inventories, A/B separation, repeat identity, and absence after removal
are still checked. It neither imports product code nor starts an installer,
client, network connection, or archive extraction.

The regression failed on the original auditor on Linux, Windows and macOS and
passes after the correction. Negative controls exercise foreign/missing/empty
IDs, disabled Skills, incorrect scope/path/cwd, duplicate or missing inventory,
discovery errors, project B contamination and residual Skills after removal.

## Reproduce the six-lane audit

The exact historical source is
`fccd72199a1eedc12e3828cf2fdc2f10a2f418f7`, tree
`41d82c5eb527f34d16ec2e458194496012bc1c22`. Its native workflow is
[46139633140](https://github.com/777genius/universal-agent-plugins/actions/runs/46139633140).
It is not the later merged-main commit
`ee6d714f9f684a5baf4b6740e4e8cb0481918be5`.

From a checkout containing the fixed tools and the historical Git object:

```sh
git fetch origin fccd72199a1eedc12e3828cf2fdc2f10a2f418f7
python3 -B -m unittest discover -s scripts -p 'test_native_client*.py'
python3 -B scripts/audit_enterprise_native_evidence.py \
  --source-repo "$PWD" \
  --evidence-dir /absolute/path/to/a-new-evidence-directory
```

The evidence directory's parent must exist. The tool creates a new private
directory and refuses to overwrite an existing one. It uses the authenticated
`gh` session to read the original run, jobs, logs, metadata and six ZIPs. It does
not read or copy credentials. Source texts come from immutable Git objects,
not the caller's working tree. It checks source blob pins, exact run attempt,
all six distinct native jobs, mandatory successful steps, original checkout
logs and authenticated artifact digests before granting per-lane qualification.

The directory retains original bytes and per-lane receipts; `receipt.json` is
written only after all six lanes pass. A failed acquisition or check remains
a failure. Fix its cause and re-use already retained evidence where appropriate;
do not rebuild native workloads merely to correct an offline reader.

The receipts retain the original auditor's limits: binary/source Skill bytes,
full filesystem images and complete installer argv/exit captures are not uploaded.
They do not establish vendor authentication, real model calls, complete cleanup
custody, or released-binary support. Codex project B proves a `skills/list`
query coordinate, not the app-server process cwd.

## Classify broad qualification correctly

The following source contexts must stay separate:

| Evidence | Actual executable / harness scope |
| --- | --- |
| Current-source native collection run `46139633140` | Exact source `fccd7219`; pinned Codex 0.162.0 and Claude; six host/client lanes |
| Full 30-source live runs `46139632929` / `46139633123` | Source-built UAP with a `0.1.115` version label for Directory eligibility; synthetic PR checkout must be bound separately |
| Marketing run `46139632998` | Published npm UAP 0.1.116, with the reviewed current-source lifecycle harness |
| Historical npm 0.1.95 / abort-cleanup lanes | Separate historical compatibility observations, not current-source installation proof |
| Merged-main workflows | Exact `ee6d714f`; PR-head evidence does not silently become merged-main evidence |

Windsurf's expected 30-plugin result is **21 Prepared lifecycles + four retained
native Skill lifecycles + five verified refusals**. It is not 30 native passes.
The positive installer observations do not qualify model/authentication/MCP calls.
A refusal is verified only when the source-specific negative assertions pass;
a nonzero exit alone is insufficient.

For full Codex cohorts, require all 30 source slots in the exact fixture order,
source/run/checkout binding, and per-case add/repeat/remove and foreign-state
assertions. For Marketing, require `status=passed`,
`two_scope_lifecycle_qualified=true`, both user/project scope results, and the
target-specific evidence. A green `report even if blocked` job does not grant
native qualification. Queued, cancelled, skipped, blocked or incomplete cases
remain explicitly unqualified.

The existing broad workflows remain in place. This follow-up does not rerun
successful matrices, alter their acceptance or treat a snapshot of pending jobs
as completed qualification.

## Downstream boundary

Agent Notifications [#694](https://github.com/777genius/agent-notifications/pull/694)
still owns actual Cursor lifecycle requalification against the merged #1142 fix.
Neither the representation regression nor the six-lane collection audit proves
that downstream product's full E2E. No release, tag or package publication is
part of this follow-up.
