# Windsurf / Devin — finish automatic installation E2E

Date: 2026-11-18. Base: `main` at `75dc17fde7b0f4ddcc1d2c12dfcd56be015723b0`. Draft only until exact-head proof.

## Baseline and missing slice

The existing `clients/windsurf` adapter is the authority for the shared Windsurf / Devin client identity. It already projects managed Skills and MCP into verified Devin Local User/Global or Project roots and retains a separate legacy Cascade path. #710 and #817 are merged. #931 owns the wider genuine native-client qualification, and #957 owns general native Skill readiness. Do not fork or duplicate those implementations.

Current version admission is intentionally asymmetric: User/Global allows exact observed `devin` 3000.11.205 or 3000.12.57, while Project allows only 3000.11.205. The newer version must not silently be treated as Project-compatible without genuine native discovery. Determine and implement the smallest justified completion of the Project automatic install path.

## Scope

- Reuse existing `ClientWindsurf`, client adapter, project component kernel, ownership receipts, CLI/SDK and native runner. No second installer, additional client ID, flags, new source of truth or speculative readiness.
- Select only the original host's primary `devin` executable and observed exact compatible version; legacy `windsurf`/Cascade and unselected sibling executable must not be reinterpreted as Devin.
- Preserve original XDG/APPDATA, project A/B isolation, unmanaged files, same-name collision refusal, symlink containment, rollback, saved repair and offline removal.
- Maintain honest distinctions among file installation, native inventory discovery, trust, authenticated runtime/model execution and public release support. Do not auto-authorize trust.
- Leave frozen VS Code, unrelated clients, generated data, Registry and publication untouched. Cooperate with active #931/#957 rather than altering their files concurrently.

## Proof and delivery

1. Verify current official Devin configuration/version and native install contracts.
2. Extend only the necessary client-specific version/admission path if genuinely qualified, with positive and negative Go tests that cover User vs Project, exact primary vs sibling, legacy preservation and package/receipt integrity.
3. Reuse the existing pinned Devin native E2E workflow and independent reader to prove public CLI add → unchanged replay/repair → remove, original Skill + stdio/HTTP MCP visibility in A/B, preservation of foreign/Cascade, on the exact final commit.
4. Run relevant Go tests/lint, whole affected E2E and applicable cross-platform CI; review at least twice for architecture/security and correctness. Never count skipped/queued/evidence-only cells as pass.
5. Commit each coherent slice directly to this Draft PR branch (each GitHub commit immediately updates remote); merge only after exact-head qualification and final review.

## Review checklist

- R1: domain purity; adapter-only client branching; existing protocol/component abstractions reused; no new duplication; exact version admission and safe refusal.
- R2: trusted-host provenance; user/project/Cascade isolation; ownership and rollback; truthful native evidence; complete final-source status.

Initial estimate: production 30–100 LOC, tests/contract/workflow 90–250 LOC, documentation 30–60 LOC. Complexity 4/10; reliability target 9/10; confidence 7/10 pending genuine native E2E.
