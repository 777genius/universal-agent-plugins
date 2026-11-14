# Shared project core implementation independent review R2

**Current status: R2 GO after `caa27b4f` corrective verification.** Both
findings below are closed; their initial rationale remains as review history.

Reviewer: `client_plans`, 2026-10-23. Reviewed frozen shared core through
`49ef94ac614a204caaa9a59478df907a24cfecef`, its original plan and R1 review,
and the subsequent root-native authority correction `08b1604a`. Gemini adapter
implementation has separate independent reviews; uncommitted cohesive CLI
fixture files are excluded from this frozen core review.

## Decision

Two narrow corrections were identified and independently verified closed in
`08b1604a` and `caa27b4f`. No additional architecture or broad implementation
change is requested. Parent owns combined source/project and native client
qualification after the core/adapters are integrated.

## Core-R2-1 — root-native registry accepted by planning could not persist

The planner deliberately accepts `NativeRegistryRoot == ProjectRoot`, required
for native configuration such as workspace-root `opencode.json`. State's
`validateProjectBinding` applied strict-child containment to both package target
and NativeProfileRoot, rejecting the same registry root that planning admitted.
This would block project clients whose native configuration lives directly at
the repository root.

Author confirmed the mismatch and corrected it in `08b1604a`: only the exact
NativeProfileRoot equality case uses the selected root's parent as its
no-follow containment base, after absolute/clean/non-root validation. Package,
data and native object locators remain strict project children. Tests accept
Save/Load of exact root-native authority and reject other project roots,
relative roots, project-root artifacts and symlink authority. **Closed** by
independent code and focused test verification.

## Core-R2-2 — copied namespace mutex must retain shared physical authority

Parent's actual CI vet identified the new embedded projectNamespaceGuard mutex
being copied by existing `projected := *e` preview code. Beyond vet, copied
guard state would split the session's pinned project/namespace identity when a
derived preview observes a new directory identity. The required correction is
one shared guard pointer initialized by New and retained by both preview copies
and WithTemporaryRoot, without a suppression or independent replacement guard.
Author corrected this in `caa27b4f`: New allocates the guard; shallow projected
views share it; WithTemporaryRoot retains the original pointer and immediately
revalidates its pins. Missing/uninitialized project guards fail closed. New
tests prove that a namespace first observed by either derived view remains
pinned for both observers, and that an already-replaced namespace cannot be
adopted by creating a temporary view. **Closed** by independent frozen code,
focused test and vet verification.

## Component ledger

| Core boundary | Review result |
| --- | --- |
| Context and capability admission | GO: absent SDK scope remains user; request/engine scope and physical project roots agree before package reads. Project admission requires explicit catalog qualification plus project and native layout ports. Unsupported/remote/manual-only requests fail before acquisition. Operational context is JSON-private and excluded from portable source identity. |
| User versus project profile | GO: detected ConfigRoot remains original user authority. Actual project NativeRegistryRoot is separately selected and threaded through plan, staging, observer, activation, exact binding checks, deactivation, repair and grouped lifecycle. Contextual physical capture returns existing authority tokens instead of substituting user ConfigRoot. |
| Scoped lifecycle and backend policy | GO: one opt-in declarative project policy supplies effective traits. Existing user traits/backend aliases remain unchanged; project Copilot/VSCode families are independent. Native-config project branches acquire attempt/recovery and launcher policy through scoped helpers. |
| Physical namespace | GO: fixed selected `.uap`, contained mutable paths, shared pinned project/namespace identities and rechecks at apply/recovery/mutation boundaries. Context/namespace construction does not create directories. |
| State and ownership | GO after Core-R2-1: project state refuses copied outside locators, user bindings/data, wrong profile namespaces and receipt paths; selected project and user environment IDs are disjoint. Exact root-native registry is permitted without broadening managed child locators. |
| Native attempt and recovery | GO: effect tracking uses plan/binding scope and actual NativeRegistryRoot; persisted attempts retain their binding scope. Namespace/physical profile revalidation remains inside existing mutation/recovery boundaries. |
| CLI composition | GO: each entry point selects the App before routing; ForScope composes independent project mutable roots and root portable intent/lock while retaining user detection and shared read-only/content caches. Unsupported binding/migration surfaces fail explicitly. Derived previews must share namespace authority as above. |
| Saved lifecycle and transfer | GO: selected environment ID and scope reach observation, coordinator, authorities, Directory eligibility, repair/update/remove/recover and public status. Export/import transfers only intent/lock and rejects same physical project-directory transfer. Source integration must preserve both selected App and identity-from-lock. |
| User compatibility and public metadata | GO by structure and existing independent R1 evidence. Zero project policy enables nothing; user wrappers and user-default ownership remain exact. Newly qualified adapter catalog scopes require explicit compatibility golden changes, handled separately in the Gemini stack. |

## Independent execution and limits

Exact Go1.26.10 focused context/project tests on the root-native correction
passed: domain0.005s; environment0.008s; statev2 0.014s; installer0.276s.
Planner/usecase packages reported no matching tests for that narrow regex and
are not represented as independent project execution proof. The clean Gemini
branch's post-49ef real merge independently passed provider project composition
0.084s, nativeconfig0.010s, statev2 0.008s and selected production CLI composition
0.031s. Author/root CI owns broader lint, full suite, platform and combined
lifecycle gates.

No native agent process/model/MCP runtime was executed in this review. No core
production file was edited by the reviewer. The original user owner request
authorizes narrowly scoped project work; unrelated frozen behavior remains
outside this review.

## Final frozen verification

Reviewed `caa27b4fdd8f017f174b03224bf0f2d9aa0f05fb` read-only, cumulatively
including `08b1604a` and shared core `49ef94ac`. Independently ran focused
project namespace, preview and cross-environment SDK admission tests:
**PASS0.010s**. Independently ran installer `go vet`: **PASS**, no copied-lock
diagnostic. The earlier root-native state/context tests remain passing as
recorded above. Author's exact changed-code and full-file size/architecture
gates report zero issues for the frozen corrections.

**R2 GO.** No confirmed core correctness, ownership or review finding remains
open. This does not certify uncommitted cohesive CLI fixtures, future adapter
stacks, hosted native clients or an executable release. Parent retains those
integration and applicable CI gates.
