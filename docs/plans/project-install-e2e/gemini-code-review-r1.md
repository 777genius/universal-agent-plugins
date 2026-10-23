# Gemini project implementation independent review R1

Reviewer: project_core, 2026-10-23. Initial read-only checkpoint `d76f29ae..a96b4aff07dedfb99297d3fab564b1c778d6968b` in the Gemini child worktree. Review scope is the Gemini adapter and the owner-authorized precise CodecGemini comment parsing delta. Shared core was reviewed separately.

## Decision

**R1 GO at final frozen `65b8f17a951e80f06f9ed9c2677dd16a3ccd3d0a`.** Scoped root selection, conservative project readiness, exact native ownership and the precise Gemini comment codec changes have no confirmed blocker. Initial checkpoint observations below are retained as review history. Native Gemini session discovery and cohesive source/project CLI execution remain separate qualification/integration gates.

## Initial checkpoint assessment

**No confirmed blocker in the scoped native paths or codec delta. Final GO is pending the author's catalog qualification, conservative project readiness, disabled/shadowed settings, alias/preflight and composed stager tests.** The initial checkpoint alone is not yet project admission/product proof.

- Project native settings/skills root is explicitly `<TargetContext.ProjectRoot>/.gemini`. Detected `Client.ConfigRoot` remains the original user profile. Project/root mismatch and changed original user context are rejected before native operations.
- Native mutation, verification, removal and registry inspection select the same explicit root; removal is supplied the recorded actual native registry and TargetContext. Exact existing ownership codecs and native skill transactions are reused.
- The managed artifact layout stays under the selected project, with physical profile canonicalization and no-follow containment reused. User home variables are not replaced and host trust is not granted.
- `CodecGemini` settings.json accepts comments while preserving the existing AST and exact entry receipts. Duplicate keys, malformed input and trailing commas remain errors. Other strict JSON codecs stay strict.
- Independently inspected hujson's parser representation: a final child Value.AfterExtra is cleared for a closing delimiter without a trailing comma and remains present when a trailing comma exists; the new sentinel check therefore does distinguish this format boundary while retaining comments.
- Remaining direct parseDocument calls in transition*.go are guarded closed OpenCode dialect transitions. They do not admit CodecGemini, so no unrelated transition broadening is needed.

## Final review checklist

Review the final scoped policy/catalog and readiness delta, run the full Gemini and nativeconfig packages independently on the frozen tree, and verify user behavior plus scoped isolation/disabled/foreign-edit/collision/alias/attestation boundaries. A cohesive composed source/project lifecycle follows integration; adapter fixture proof must not be labeled native Gemini process certification.

## Final frozen checkpoint verification

Reviewed the exact final tree read-only, including the real core merge, Gemini catalog qualification and project policy, all moved composed contracts in the external providers test package, and the final `document.go` / `gemini_document.go` last-member deletion correction.

- The catalog qualifies only Gemini project scope through its declared native-config lifecycle. It does not select a test constructor or alter the user lifecycle traits.
- Project filesystem success consistently remains ActivationPrepared plus VerificationInstalled, without activation attestation. Neither repeated automatic reconciliation, verify-only, a restart, ambient trust variables nor `--activation-complete` can claim native session discovery. The existing ActivationPreflighter rejects the shortcut before mutation and direct adapter reuse invokes the same guard.
- Preflight reads user/project settings through the exact comment-aware native kernel, preserves disabled skills and MCP restrictions, and reports possible `.agents/skills` shadowing. Workspace trust, system policy and MCP initialization remain explicitly unobserved; there is no new trust/settings engine.
- The composed loader/planner/renderer/stager/activator contract renders portable commands as Gemini skills and installs/removes the exact skill/command/MCP objects under the project root. Foreign entries/comments, user profile and project B remain unchanged. Malformed settings, object collisions, writer-lock failure, scope/root mismatch and changed original user context fail closed.
- The final codec fix removes only trailing commas created by deleting a last managed member, moves surviving foreign trivia to the enclosing delimiter, and revalidates the comment-only document. Native-invalid input trailing commas still fail; unrelated strict JSON codecs remain unchanged. The foreign-trivia regression exercises the problematic preceding member directly.
- Architecture was preserved by moving composed journey tests to `providers/project_gemini_*_test.go`, package providers_test; no upward-import suppression was added.

Independent final execution (`-count=1 -p=2 -timeout=2m`): full Gemini **PASS 0.046s**, full nativeconfig **PASS 3.730s**, full domain **PASS 0.034s**, and providers `^TestProjectGemini` **PASS 0.061s**. The authored CI lint evidence records both exact changed-code and full-file size/architecture gates at zero issues. Shared core CI now discovers TestProject contracts in nativeconfig, providers and all client packages on Linux, macOS and Windows.

No confirmed review finding remains open. These are inert fixture/contract checks, not hosted native Gemini product certification.
