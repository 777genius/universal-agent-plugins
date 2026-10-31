# Qualified project preparation and planner regression amendment

2026-10-24. Root's actual assembled a9dd4402 domain/codec/publicProjectCLI gates
passed, but full planner exposed two old cases. Two independent plan GOs BEFORE
source/test edits; fresh focused tests and two independent code GOs afterwards.
This amendment does not treat all failures as obsolete assertions.

## Actual diagnosis

TestPreparationRequiresSupportedNativeUserTarget still expects every Kiro project
prepare request to fail. Kiro now explicitly qualifies project scope and inherits
its supported prepare intent. However the actual direct synthetic plan has only
ScopeProject + relative NativeRegistryRoot="native", with no target context,
physical artifact or active layout. ApplyInstallIntent currently checks only the
scope's allowed intent then Kiro.RefinePreparation checks nonempty native root;
that accepts a malformed context. Preserve this case as a real negative.

The second old planner case uses Cursor project scope with no ProjectRoot and
expects PlanUnsupported/scope_not_supported. Cursor project is qualified now;
missing canonical context must fail validation. Preserve the undetected-user
case; use actually unsupported ChatGPT project scope with a real valid root for
the unsupported-scope negative, plus an explicit qualified-Cursor missing-root
error/no-resolved-artifact case. Do not remove unsupported or malformed tests.

## Narrow source correction

For prepare intent only, if either scope field is project OR TargetContext has a
nonempty ProjectRoot, ApplyInstallIntent validates matching project scope and
NewTargetContext BEFORE changing InstallIntent/refining readiness. This refuses
missing/relative/nonclean roots and user-labeled project context. Continue the
existing scope trait/compatibility checks; do not invent generic target layouts
or add a client-ID switch. Unsupported plans remain unsupported and user-only
legacy preparation semantics remain unchanged for genuine user contexts.

Kiro.RefinePreparation, including direct calls, routes every project candidate
through its existing kiroProjectPlanRoot with the plan's original UserConfigRoot.
Require exact project/native .kiro root, physical ID and active artifact, preserving
user profile context; project preparation uses existing KiroProjectAction and
prepared/package-valid. It never writes native config, grants trust or attests
loading. This reuses the already reviewed project authority helper rather than
loosening its root/path contract. Keep ordinary user PrepareAction unchanged.

## Meaningful regressions and gates

Retain the malformed relative-root case and add missing/wrong native/root/scope/
artifact/active variants, including direct refiner calls and no readiness change
on refusal. Add a valid qualified Kiro project prepare positive from real absolute
canonical fixture roots/exact layout, asserting prepared/package-valid and no
filesystem effects/original-user preservation. Keep unsupported ChatGPT project
and missing-root Cursor cases separate from undetected-user admission. Preserve
all existing compatibility/unsupported-package tests, user tests and goldens.

Run complete planner plus targeted Kiro/intent/provider contracts and relevant
lint. Do not rewrite unrelated native guard or dependency snapshots; root's
separate Codex parser extraction resolves inherited closure drift. Checkpoint
small approved source increments to root immediately; root alone publishes and
merges only project feature stacks, never main/release.
