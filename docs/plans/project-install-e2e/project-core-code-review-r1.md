# Project core implementation independent review R1

Reviewer: sources_lead, 2026-10-23. Frozen cumulative implementation: `3ea24e30..3f421660c0d5856c9f951759ade176c393394e4d` (context/ownership contracts, scoped lifecycle authority, CLI composition and the final compile/lint corrections). Read-only review; no project implementation edits.

## Verdict

**R1 GO for the shared core implementation.** No remaining confirmed correctness or authority blocker found in this review. This verdict covers the shared context/namespace/policy/CLI seams, and does not certify the forthcoming client adapters or executable release. The representative cohesive project lifecycle and each client's native layout/runtime proof remain required when those stacks integrate.

## Component review ledger

| Component | Independent assessment |
| --- | --- |
| Target context and SDK admission | GO. Missing SDK scope remains user; project request and engine roots must agree before package reads. Operational context fields are JSON-private and do not enter source/lock identity. The original detected user profile remains distinct from the selected project and native registry. |
| Core policy and capability gating | GO. Project policy has an inert zero value; project admission requires declared scope and concrete project/native layout hooks. Effective lifecycle and backend families are explicitly scoped. Existing user traits/backend aliases and constructors remain unchanged. ChatGPT project requests are rejected before acquisition. |
| Physical namespace | GO. `.uap` is fixed beneath the selected physical project, contained mutable children are checked before access, existing project/namespace identities are pinned and rechecked before effect/recovery boundaries, and non-project engines cannot accept project requests. Namespace symlink/identity replacement tests exercise refusal. |
| State and recovery ownership | GO. Project state rejects user-scoped bindings/data and copied absolute locators from another project; project receipts use `project-default` and remain disjoint from user-default ownership. Native registry, package target, profile namespace and data locators are validated under the selected root. Existing recovery paths reuse the same selected engine and physical namespace checks. |
| Lifecycle and backend handling | GO. Scope/root reaches planning, staging, inspection, activation, deactivation, repair, groups and removal. Deactivation receives the persisted native registry and original target context rather than inferring a project from an artifact path. Project config lanes do not inherit user CLI registry/backend sibling behavior. Native identity kernel checks now use `plan.Scope`. |
| Production CLI composition | GO. `App.ForScope` composes independent mutable roots under project `.uap` and project-root intent/lock, while retaining original user detection and shared source/security/content caches. Every selected route preserves the selected App; unsupported migration/binding routes fail explicitly. No project composition/read-only inspection creates `.uap`. |
| Saved dependency and transfer seams | GO. Resolver/coordinator/observation/alias ownership use the selected environment ID. Signed Directory requests carry selected scope. Portable exchange is only manifest/lock; same-directory project transfer refuses before writes. New direct-source helpers must retain this selected App during the planned source/project merge. |
| User compatibility | GO by structure and focused regression evidence. Old `Observe`/user constructors remain wrappers, user-default identity stays exact, and new project policy is opted in only with a qualified adapter. Existing source identity schema is not changed by this core branch. |

## Provisional observation closed without a code change

The reviewer initially questioned ignored second results from `NativeRegistry` in target/authority capture. Signature verification establishes that the existing interface returns `(root, executable string)`, not an error; ignoring the executable in target-only resolution is intentional. Actual error-returning project target, precondition and contextual authority hooks propagate their failures. No interface change is requested.

## Verification

Independent focused tests cover project context JSON/private validation, selected environment ownership, project state-copy refusal, namespace symlink/replacement rejection, SDK cross-environment refusal before package access, production composition isolation, unsupported target refusal and same-directory transfer refusal. These run against the frozen correction checkpoint with Go 1.26.10, GOMAXPROCS=3 and the task-owned shared caches. Results are recorded below on completion. No external services, package payloads or real agent processes are run by this focused review.

## Integration requirements

Preserve source branch `identityFromLocked` on every shared store/cache/replay path, additive source fields in state projections, typed acquired-source validation and exact old GitHub identity. Preserve project branch selected scope/root on each new source lifecycle helper and repair request. The top-level source/project integration needs a cohesive generic Git/archive project run after the first qualified adapter lands; fixture core checks alone cannot establish that whole product journey.

Focused independent test invocation passed all five packages: domain 0.003s; environment 0.012s; statev2 0.010s; installer 0.304s; production CLI composition 0.213s. **R1 GO confirmed on3f421660.**
