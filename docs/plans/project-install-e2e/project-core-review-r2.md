# Project core plan — independent R2 review

Reviewer: source lane. Reviewed revised `/workspace/scratch/2819fbb48645/research/project-core-plan.md` against implementation-base source at `3ea24e3014ddf6679ca86729c56f1069112b0d3a`. Read-only review; no implementation edits.

## Verdict

The architecture is suitably narrow: one selected runtime, separate mutable namespaces, shared verified store, unchanged portable source graph, explicit project context and no fabricated user profile. Approve after the three small clarifications below are applied and verified. They do not require a new engine, lock format or extra client abstraction.

## Findings to fix in the plan

### R2-1: make namespace physical containment an entry gate

The plan rejects unsafe root aliases and copied `.uap` locators, but does not explicitly say the `.uap` directory itself and each existing ancestor/owned child are checked without following symlinks/junctions before creating/reading mutation authority. `<A>/.uap -> <B>/.uap` defeats logical namespace separation even when all string paths are correctly rooted under A. A copied namespace can also carry another project's live locks/data paths.

Required clarification: canonicalize the explicitly selected project directory; reject `.uap` symlink/junction/reparse aliases and ancestor escapes; bind mutable namespace authority to the selected canonical root. Revalidate existing namespace identity immediately before apply/recovery. No need for a new global registry or persistent path-derived portable ID. Add one negative using a `.uap` symlink/junction to a real other project's mutable directory and assert zero changes to both environments.

### R2-2: distinguish operational facts from portable serialization explicitly

The plan says `TargetContext` is excluded from public JSON/portable formats, while adding it to exported domain structures and adding ProjectRoot to Request/config/plan structures. Default Go JSON encoding would expose exported operational fields unless exact tags/custom projections are specified. Source-derived lock/manifest intent must remain portable across machines; project root must still be available to the process that owns deactivation.

Required clarification: `TargetContext` and operational ProjectRoot fields use explicit `json:"-"` on structures that can be encoded, or are omitted through existing dedicated public projections. Inspection/status may intentionally report selected scope/paths via existing nonportable status fields, but neither manifest digest nor `uap.lock` gets project/native/profile/cache paths. Add a marshaling/portable-pair check proving a literal selected root does not enter the lock or acquisition source identity. No broad output rewrite is needed.

### R2-3: define project transfer when default exchange equals source

`deps export/import` currently derives the active environment directory from App and defaults the exchange directory to `.`. After selecting project scope, invoking from the project root makes these the same directory. The existing global command wording and default are no longer sufficient to explain a safe result.

Required clarification: export/import operate on the explicitly selected environment; if the canonical exchange pair equals the selected pair, refuse/no-op before writes with guidance to pass a different exchange directory. Preserve exact input bytes and current race/regular-file checks. Do not invent a second portable root format or silently read a parent/user manifest. Add one source==destination negative plus project-to-fresh-project locked restore using an explicit exchange directory.

## Additional review observations

* Empty SDK Request.Scope retaining user default while a project-configured Engine rejects a mismatched request is coherent; preserve the request/engine mismatch negative.
* Source lane will add new remote Git/archive variants to the same environment Requirement/LockedSource contracts. Core should continue treating package acquisition opaquely and scope only runtime ownership/selection. Source identity must not acquire ProjectRoot or Context fields.
* Shared store bytes are safe to reuse only with existing digest/executable verification; project namespace metadata must be reconstructed from the current selected lock instead of adopting cached ownership. Existing core plan already states this.
* Optional ProjectPhysicalAuthority uses the existing token/revalidation mechanism and avoids a second universal authority system. Exact native registry/root must be revalidated during deactivation/recovery as the revised plan says.
* Independent plan reviews are complete only after the author applies these clarifications and this reviewer confirms the updated sections. Root remains the implementation authorization gate.

## Applied-fix verification

Verified the revised core plan on 2026-10-23. R2-1, R2-2 and R2-3 are all explicitly fixed in the storage contract, DTO contract, transfer semantics and E2E negatives. **R2 GO; no remaining plan findings.** Root remains the code authorization gate.
