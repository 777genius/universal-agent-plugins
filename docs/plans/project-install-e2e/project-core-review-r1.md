# Independent project core plan review — round 1

Reviewer: client-plans lane. Reviewed plan `project-core-plan.md` and current
`domain/clients.go`, `domain/state.go`, `domain/planning.go`, planner dispatch,
client identity/lifecycle implementations, and command port. Review date:
2026-10-23. Verdict: **changes required before implementation**, with the
existing engine/storage direction accepted.

## Required fixes

### P1 — project context stops before deactivation

The proposed context reaches `DeliveryPlan`/activation but not
`domain.DeactivationRequest`. That existing request has neither scope nor a
project root/plan. Claude/Qoder/Grok removal must run the native command from
the recorded project and use the correct scope; Gemini/OpenCode/Kiro direct
config removal must distinguish the project registry from the unchanged user
`DetectedClient.ConfigRoot`. Reconstructing context from the managed artifact
path would violate the plan's own invariant.

**Fix:** carry validated `TargetContext` through deactivation and any read-only
binding/target-facts path used to construct it. Reconstruct from the configured
namespace plus exact recorded binding, validate scope/root agreement before
dispatch, and add focused removal-from-unrelated-cwd tests. The command port
already has `Dir`; no new runner abstraction is required.

### P1 — native registry authority has no complete project contract

The planner currently invokes `setNativeRegistry` before target resolution and
defaults it to the detected user config root. Several native adapters use the
plan registry for identity checks but `request.Client.ConfigRoot` for mutation.
Adding `ProjectTargetLayout` alone does not establish which exact native root
is authoritative, and treating a project's `.gemini` or `.kiro` root as the
user profile would break the explicit no-profile-substitution contract.

**Fix:** specify that project adapters implement the existing
`NativeRegistryLayout` with `PlanInput.TargetContext`, selecting their actual
project registry/config root (or explicit unsupported/manual remote state).
Core validates the resulting plan context and supplies it consistently to
activation/deactivation/verification. Per-client implementations must branch
on context rather than overwrite detected user config. Add a generic contract
fixture where package target, native project root, and user profile are three
different paths, covering plan/apply/inspect/repair/remove.

### P2 — the before-acquisition unsupported gate is not assigned

The plan says absence of `ProjectTargetLayout` rejects a project client before
source acquisition, but the current planner runs after the envelope is loaded.
The new interface by itself cannot deliver that ordering. Client definitions
also still declare only user scope.

**Fix:** identify a reusable read-only selected-scope capability check at the
installer/CLI entry boundary before acquisition, backed by registered adapter
support. Client stacks own adding `ScopeProject` only after qualification.
Keep package/component-specific admission in the existing planner after load.
Test an unsupported project's add/deps-add using a counting acquirer: zero
acquisition, state, journal, and native effects. Do not pre-advertise all clients
in the base PR.

## Accepted components

| Component | Assessment |
| --- | --- |
| Explicit cwd default and `--project-root` | GO; deterministic root, no hidden Git-root search. Document nested cwd semantics in CLI help. |
| `uap.toml` / `uap.lock` reuse | GO; no scope/path additions to portable identity. |
| `.uap/` mutable state and project-default receipt | GO; namespace separation avoids a global project registry. Existing foreign/symlink path guards must apply at namespace creation. |
| Shared digest store | GO; no cache removal or cache-as-ownership confusion. |
| Existing user APIs and defaults | GO; preserve empty-scope SDK compatibility and user-default output/paths. |
| `TargetContext` and optional layout hook | GO after P1 fixes; adequate for native registry, component projection, explicit session-config, and manual remote clients without a larger framework. |
| Scoped App reconstruction | GO; correctly identifies Cobra App-copy closure risk. Cover production composition, not only an injected test App. |
| Coordinator/observation parameterization | GO; retain Observe wrapper; inspect every receipt lookup and internally generated authority. |
| Exact recovery, copy/move refusal | GO; compare reconstructed paths plus current authority before effects. Portable restore must start from manifest/lock only. |
| Focused E2E and evidence labels | GO; fixture command success is not genuine client discovery/runtime proof. |

No style-only changes requested. No implementation performed during review.

## Fix verification

The revised plan was checked on 2026-10-23. All three required findings are
addressed: deactivation/binding facts receive context and the exact native root;
project registry selection is mandatory and preserves the original detected
user profile; a shared entry-boundary capability gate prevents acquisition for
unsupported scopes. Optional contextual physical-authority capture reuses the
existing token/revalidation contract rather than manufacturing a profile.
**Round 1: GO to independent round 2.** This does not authorize implementation.
