# Kimi approval poll refusal diagnostic: validation

Code reviewed: `8311c8e9f571e9fa272abc2daed1a2f367ad160e`.
Tree: `86e78b33b391186b85641fd1838e5e608c85c20b`.
Base: `b2bb2b7f85b5461420f932478b197905e6fc5ad9`.

The approved plan was committed before code as
`f4aa9ad63dd6726d936b808d59dba0ea39cbe31a`. Two independent GPT-6.1
Sol xhigh pre-code reviews accepted the same plan; two independent exact-code
reviews then accepted the implementation without blocking findings.

## Actual validation

- Inert Python Kimi suite: 57 tests passed in5.650 seconds.
- Inert runtime admission suite: 27 tests passed in6.417 seconds.
- Original b2bb single-tool artifact: all23 raw member bodies admitted unchanged;
  its failed result and allfalse qualification claims remained unchanged.
- AST parsing, source hashes and git whitespace checks passed.

The tests exercise the actual polling observer and its ordinary supervisor around
an existing harmless Python child. They check both refusal causes, the unchanged
20poll boundary, no approval/history/snapshot/shutdown action after refusal,
quarantine, cleanup facts, finite scalar projection, privacy, hostile types,
malformed ambiguity, copied facts, mixed first causes and later cleanup errors.
This gate did not run a native vendor binary, installer or Go process.

The optional diagnostic does not turn the existing failure into success. It adds
only a bounded reason for the two original multiple-pending guards and the
original exhaustion guard. Existing manual approval validation, timings,
quarantine, supervision and qualification conditions remain unchanged.

## Exact review and executor records

- `/tmp/uap-additional-kimi-poll-refusal-code-R1.md`: SHA256 `51ee412c539cdb59e0087ca3a2b6fe0759c94b59364cbb9ef1704131e4ca51d3`.
- `/tmp/uap-kimi-poll-refusal-code-r2-review.json`: SHA256 `b07cca2fcb15eaeb7ee8d2339ce878384542a8c7f0a3096d7d96f12b9e3ce992`.
- `/tmp/uap-additional-kimi-poll-validation-442uupq9/report.json`: SHA256 `33f5eedbbe75926aee8c5b6aeb9dd7b026dab29f0a4298021a33de4dd0cc3ad6`.

Local record paths identify this execution's review receipts; the committed
source and this validation description remain the durable repository record.
Native follow-through still requires a genuine disposable hosted run after
publication of the composed Kimi fixes. This validation makes no authentication,
model, complete lifecycle, release or main-merge claim.
