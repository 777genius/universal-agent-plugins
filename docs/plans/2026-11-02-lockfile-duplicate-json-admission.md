# Reject ambiguous duplicate JSON object keys in the portable lock (2026-11-02)

## Confirmed problem

The current `cli/internal/agentpluginscli/deps_resolver.go` decodes `uap.lock` using Go `encoding/json.Decoder` with `DisallowUnknownFields`. This rejects unknown fields but **does not reject duplicate object keys**. A syntactically legal lock can have the same `lock_version`, `manifest_digest`, `source_key`, `source`, or nested `commit` field twice. Different tools/reviewers may interpret the first or last value; Go silently adopts the last. This undermines immutable-lock auditability without adding capabilities.

## Minimal architectural change

Before any lock is materialized or reconciled, invoke the existing `conformance.RejectDuplicateJSONKeys` once in the **single CLI decode chokepoint**. This validator is already used by the source index and standard loader; do not add a duplicate JSON parser or change the `uap.lock` schema. Keep Go `DisallowUnknownFields`, exact trailing-value rejection, manifest/lock identity and immutable-tree verification intact.

## Tests and E2E

- Feed the real portable `uap.lock` writer, then tamper root and nested packages/sources/selectors with repeated identical keys as well as differently valued ones. Reject even when values match.
- Test escaped-equivalent key syntax such as `"loc\\u006b_version"` versus `"lock_version"`: JSON string normalization must happen before duplicate detection.
- Run exact public CLI `install --locked` or `deps status` with a tampered lock, assert fail closed with unchanged lock, managed native files/receipts and no source acquisition. Exercise valid clean locks as positive control.
- Require targeted real run/pass in the existing Package Manager E2E catalogue on Linux, Windows and two macOS runners, without skip or adopting a new state format.

## SOLID / Clean Architecture / DRY / no-overengineering

- `conformance` owns JSON syntax; `environment` owns graph semantics; CLI owns file decoding; installer owns filesystem side effects. One new validation call at existing CLI port, no parallel parser or configuration layer.
- No feature flags, new manifest fields, VS Code changes or accepted ADR modifications.

## Exit gates
- [ ] Production fix and parser/unit tests
- [ ] Full portable Package Manager E2E and native target preservation
- [ ] Security/data-loss/architecture critique; fix comments
- [ ] Final-head CI and safe squash merge
