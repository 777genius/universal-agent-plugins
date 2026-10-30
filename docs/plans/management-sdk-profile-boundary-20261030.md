# Captured-profile creation policy correction

Date: 2026-10-30. Continuation of #566 at `ff39443ca46f0f77037ca3324a7854be40b5b12c`.

## Observed failure

Required job 123706365629 completed the CLI and client/usecase tests, then failed
`TestClientIDBudgetDoesNotGrow`: `usecase` newly branches on a client identity.
The offending branch in `stagedClientBinding` captures the selected Copilot user
profile only when creating a new binding. This behavior is needed; its placement
violates the established declarative client-policy boundary. The architecture
budget must not be increased, suppressed or bypassed.

## Minimal correction

1. Declare one internal, non-serialized client trait for recording the selected
   user profile when a binding is first created. Enable it only on the Copilot
   definition. Do not repurpose `BindsNativeProfileRoot`, whose existing behavior
   can fill an empty profile on an existing binding.
2. Replace only the client-ID condition in `stagedClientBinding` with the declared
   trait, keeping the new-binding and user-scope conditions. Existing profile
   authority, native-config/project logic and selected-delivery behavior remain.
3. Preserve the actual profile isolation fixes, including native `COPILOT_HOME`
   binding and contradictory-root refusal. Do not adopt existing blank-proof,
   mismatched or coalesced sibling bindings. VS Code receives no new trait.
4. Retain the creation/existing/sibling/project regression and add a focused
   declaration test proving only the intended user client opts in. No persistence
   field, public capability, new interface, migration or generic policy engine.
5. Integrate current main without losing independent fixes, verify the narrow
   diff and run the existing architecture, domain, Copilot and usecase tests,
   then the genuine SDK/native workflow and ordinary Required gates.
6. Review the actual source and fix concrete findings before merge. Prior native
   receipts remain tied to their original source; new results bind the new head.

## Review questions

Can the trait accidentally inherit into a sibling or project route? Does existing
binding authority remain unchanged? Does the test still exercise original user
A versus ambient B? Is a boolean trait the smallest extension of the already
established client table rather than a new abstraction?

The available session cannot select independent GPT-6.1 Sol/xhigh workers.
Repository reviewer results, self-review and executed tests retain their actual
provenance. No new native client program, VS Code work, release or deployment.
