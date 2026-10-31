# Combined project client integration independent review R2

Reviewer: cursor_clients, 2026-10-23. Frozen integration:
`daa705adbfb1681d52f8d6259db7cdf716b75195` in `work/uap-project`.
This is a bounded integration/merge review, supplementing the separate client
implementation reviews. No implementation changes were made during review.

## Decision: GO

The merge retains the independently reviewed Codex, ChatGPT, Cursor, OpenCode
and Claude client implementations and their fixtures. No blocker was found in
the two substantive conflict resolutions.

### Client and fixture preservation

Compared each final client adapter directory against the corresponding second
parent of its integration merge. Every adapter directory is byte-identical:

| Client | Reviewed leaf integrated |
| --- | --- |
| Codex | `e38612444534df661adac2b864a6e9bcc05c5080` |
| ChatGPT | `0a7f6442a50c14dc0b6f556904d4692a70e1cdf0` |
| Cursor | `1f7315e9cb7dd786de92857f5d7e2188ce88311a` |
| OpenCode | `72254cd3ae2c2ed96103d78334a8316a3ad14ad5` |
| Claude | `ca4de139dfcb3d19b3869385504eb2a1d7fadc00` |

Also compared all tracked client-named test/fixture files from each leaf;
none changed or disappeared. The integration deletes no tracked path relative
to neutral-component base `c96fd096`. Existing Gemini/source/core work remains
in the ancestry. The shared client definition changes retain all existing
user traits and add project scope only for the approved client lanes; ChatGPT
remains an explicit unsupported project target. No legacy capability removal
or VS Code modification is introduced by these merges.

### Native source selection conflict

- PROJECT_TESTS contains both the Codex project component lifecycle and Claude
  project plugin lifecycle. The workflow passes --project-components for
  either client, while OpenCode retains its original selection.
- Distinct Codex and Claude exact-selection tests survive; neither silently
  overrides the other Python method. Both compare the anchored selection with
  actual Go declarations and the union of each client's legacy and project
  required suites. Neither project test enters released CLIENT_TESTS.
- Existing PINS, PATTERNS and REQUIRED_TESTS are structurally unchanged.
  Among existing runner functions, only main changes; require_hosted,
  provisioning/attestation, profile creation and release preparation helpers
  are unchanged. The project selector rejects released, draft, prepared-input
  and prepare-only combinations before hosted admission or effects.
- Failure, omitted required tests and any skipped native test remain failures.
  The current-source project flags do not qualify historical/released proof.
  Workflow permissions, platforms, exact source checkout, credentials handling,
  timeouts and evidence upload remain unchanged by the integration.

### Dependency lock conflict

The ChatGPT project's read-only target preflight runs before environment lock
creation. The locked operation still checks recovery, rereads and validates the
authoritative manifest, and resolves against that snapshot. The source lane's
publication helper remains intact: reread manifest equality is required before
publishing the dependency store and lock. Extracting the recovery inspection
into a helper preserves its error and recovery-required behavior. Thus the
merge retains unsupported-target refusal and source-cache publication safety.

## Independent bounded verification

- `python3 -m unittest discover -s scripts -p test_native_client_matrix.py`:
  **18 tests PASS**.
- `python3 -m unittest discover -s scripts -p test_released_native_evidence.py`:
  **42 tests PASS**.
- Read the combined merge diffs, complete runner selection changes, workflow
  condition and dependency lock implementation; compared adapter and fixture
  Git objects against all five integrated leaves.

No native client payload was executed, no hosted environment was fabricated,
and no broad compile was repeated. This GO covers merge correctness and
preservation. It does not substitute for the pending genuine hosted native,
cross-platform or release qualification runs. The separately reviewed CI
routing amendment is still required to trigger applicable jobs on stack bases.
