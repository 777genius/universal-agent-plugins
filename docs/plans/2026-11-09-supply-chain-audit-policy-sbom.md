# Plan: UAP supply-chain CI audit, SBOM and local policy

**Date:** 2026-11-09 · **Status:** Draft plan, opt-in additions only  
**PR lane:** supply-chain only · **Base:** `main` @ `00bcdf486dabef2dc2a4e6cb239bae818a451ab0`

## Goal and boundaries

Make UAP installation evidence independently inspectable in CI without weakening existing signed Directory, verified immutable source tree, LintAI / SkillSpector and `uap.lock` contracts. Deliver three **small opt-in capabilities**:

1. **Machine-readable offline audit** of a selected saved User/Project agreement (`agentplugins deps check --format json` is already implemented). Reuse exact existing integrity code; expose a CI-friendly *aggregated* report and stable failure classification rather than a parallel verifier.
2. **SBOM export** from a validated `uap.lock` to CycloneDX JSON / SPDX JSON; include pinned source, content digest, dependency edges and known package metadata. State explicitly that SBOM inventory is **not** SLSA provenance or vulnerability certification.
3. **Local, explicit policy gate** (allow/deny source hosts or coordinates, permitted MCP transports, required pins/hashes) on the *already resolved* dependency closure **before UAP writes client files**. Strict validation and failure-closed behavior. Keep default behavior unchanged when no policy is specified.

**Benchmark against APM:** its `policy` and `apm lock export` modules are architectural/behavior references, not drop-in Python code. APM policy inheritance is experimental and has explicit bypasses. APM itself documents that its SBOM has no signed provenance and packages are not signed. UAP already owns signatures and integrity verification for its own directory/release channels; do not conflate different trust domains.

- https://github.com/microsoft/apm/tree/main/src/apm_cli/policy
- https://github.com/microsoft/apm/blob/main/src/apm_cli/export/sbom.py
- https://microsoft.github.io/apm/enterprise/security/
- https://github.com/777genius/universal-agent-plugins/blob/main/docs/dependency-integrity.md
- https://github.com/777genius/universal-agent-plugins/blob/main/SECURITY.md

## Strict scope (no enterprise platform in this PR)

- Reuse existing `deps check`, immutable snapshots, package digests, signed-index and local scanner. No second source parser, dependency solver, scanner, keyring or registry client.
- Audit JSON uses versioned `schema_version`, `scope`, `outcome`, stable `checks` and optional `findings`; no token/secrets/native absolute paths in portable reports. If cached bytes are missing, return **unavailable** and non-zero; no false pass.
- SBOM includes only successfully checked exact lock coordinates. Deterministic output and stable sorting; avoid guessing licenses, authors or hashes not present. Missing source metadata remains missing, not `NOASSERTION` when format has appropriate representation.
- Local policy file lives in explicitly selected path (or an existing UAP-specific project config only after design review); strict bounded parser (no arbitrary includes, external fetch, interpolation, remote organization discovery or inheritance). Rules: source allow/deny, MCP transports, pinned and hash-required. Noncompliance blocks before filesystem writes.
- Separate **source authenticity**, **byte integrity**, **package scanning**, **license inventory** and **runtime execution** in statuses. Signed release attestations authenticate binaries, not downloaded untrusted third-party agent instructions.
- No automatic network calls from audit/export, no provider API keys and no upload of customer package content.
- Do not change existing default CLI install flow, lock schema, project scoping semantics, or other targets.
- Explicitly exclude full organization policy discovery/inheritance, centralized administrative server, new hosted scanners (Snyk/Socket/Trust Hub), new UI, automatic malware claims, external signature standards and SLSA attestation issuance.

## Implementation plan

1. Inspect `deps check`, lock models, existing `security` review and command wiring. Make a short contract matrix that distinguishes **already present** vs **missing**. Choose whether a new `security audit` subcommand adds value; if not, enrich existing `deps check` rather than duplicate it.
2. Introduce only an output/reporting boundary for combined integrity and scan outcomes. Never re-fetch/resolve content when user asked for offline/CI. Stable non-zero for invalid/unavailable/errors/cancelled, with secret redaction.
3. SBOM: pure transformations from verified locked dependency graph to CycloneDX/SPDX types, sort output deterministically, cover URL/path escaping and double-identities, emit no false provenance. Add CLI option or command without changing install behavior.
4. Policy: bounded configuration DTO/parser and pure decision evaluator. Wire one pre-apply gate after resolution & digest admission, before project/user managed mutations. Use existing security error/result conventions. No `--no-policy` surprise by default; policy opt-in is explicit.
5. Tests: snapshot golden for both SBOM formats and audit JSON, invalid schemes, private creds, changed lock, missing cache, altered bytes, revocation and checksum mismatch, unsupported MCP transport, transitive prohibited source, org-style path confusion, Project A/B isolation, symlink race, no-op audit and no-write on blocked apply.
6. Documentation, CI recipe and review. Re-run baseline source and native project E2E to catch regression in behavior; compare with APM for missing controls but do not copy its versioned policy schema or introduce remote inheritance.

## Acceptance criteria

- `deps check` remains strict and offline; audit never substitutes a signed Directory assertion for byte-level package verification.
- SBOM output validates against published CycloneDX/SPDX JSON schema and is reproducible byte-for-byte for same locked input.
- Local policy sees the **entire transitive closure**, not just direct roots. Explicit Project/User rules do not silently cross scope.
- Installing with policy failure leaves original files, receipts, lock and native configurations untouched; recovery covers interrupted operations.
- Negative/fuzz tests for config parser, path and input size; redacted machine output and zero network when offline.
- CI cross-platform Go tests, architecture guards, dependency/audit integration E2E, code review then green required checks.

## Risk register

- **P0:** duplicate trust/verifier creates conflicting verdicts → use existing integrity/scan engines as the only authorities.
- **P0:** policy gate placed after mutation → enforce before all writes and prove negative no-write tests.
- **P0:** source credentials/paths leak through SBOM → redact and test.
- **P1:** unsupported CycloneDX/SPDX fields fabricated → schema validation and documented omission.
- **P1:** policy scope creeps into organization governance → keep local-only opt-in file; future org governance separate if needed.
- **P1:** scanners unavailable/missing verified data → explicit `unavailable`, not clean.

## Estimated size and definition of done

~2,100–3,800 new lines including docs and adversarial tests, assuming core integrity and scanner APIs are reused. Target is improved **verifiable CI security posture**, not a promise of being unconditionally more secure than APM. Done: fixed CLI contract, validated deterministic output, pre-write denial proof, 3-OS test matrix, reviewed changes and green CI.

### Review round 1 / 6 — native facts

**Finding and concrete plan correction:** Source truth: deps check already has versioned offline JSON and strict integrity. Avoid a duplicate audit command unless existing report cannot meet CI needs; benchmark APM SBOM/policy without treating either as a package signature.

**Decision:** This is part of the acceptance criteria; a contradictory implementation must be rejected rather than treated as a harmless warning.

### Review round 2 / 6 — scope

**Finding and concrete plan correction:** Scope: no org policy discovery, inheritance, remote download, provider integration or new trust service; local opt-in rules only. Security index signatures and release attestations remain separate trust domains.

**Decision:** This is part of the acceptance criteria; a contradictory implementation must be rejected rather than treated as a harmless warning.
