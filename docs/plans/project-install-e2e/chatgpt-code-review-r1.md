# ChatGPT project boundary independent implementation review R1

Reviewer: codex_claude, 2026-10-23. Reviewed frozen leaf
`74608d3b4340815d9603bed7030411b15818be2e` in
`work/uap-project-chatgpt`. Reviewer did not author this implementation.

## Decision: GO

No blocker found in the explicit unsupported project boundary. The leaf adds no
native project capability, filesystem installation layout, fake profile,
remote-registration attestation or project preparation artifact. It preserves
the existing user personal/manual preparation contract.

## Production composition and no-effects assessment

- Production scope selection uses the selected existing project and core's
  namespace guard. Explicit ChatGPT and mixed target requests are refused by
  the shared project capability gate before detector, acquisition, scanning or
  lifecycle dispatch. No ProjectTargetLayout, NativeRegistryLayout or project
  catalog declaration is added for ChatGPT.
- The composed fixtures rebuild the real production App through ForScope and
  reapply only detector/acquirer/security spies. They exercise saved and
  imperative add, no-save, personal app IDs, completion flags, positional
  install, update, repair, remove, mixed targets and invocation-cwd default.
  The error remains `chatgpt does not support project scope`; success output,
  host-boundary calls and filesystem changes are rejected by assertions.
- The project dependency preflight reads and validates declared manifest
  targets before the environment lock can create `.uap`. This is an early
  refusal check only. The authoritative manifest is still reread and validated
  after locking, recovery admission remains inside that lock, and existing
  manifest-change checks still govern publication. User scope returns before
  this new project-only preflight.
- Saved lock/update/sync/environment-install fixtures require no lock,
  `.uap`, user-state namespace or acquired source. Existing manifest and
  user/project-A/project-B byte-and-mode inventories are preserved. Adjacent
  transfer validation also validates target eligibility before publication;
  no alternative import/install route is added by this leaf.
- Direct ChatGPT activation/preflight rejects either project scope or a
  nonempty project root, including inconsistent legacy labels and positive
  attestation/personal preparation flags. Deactivation refuses the same
  unsupported context even with external-uninstall attestation. The provider
  invokes preflight before its generic attestation shortcut.
- Existing user activation still returns manual or personal-prepared outcomes
  under its existing requirements. It does not become installed verification
  or remotely observed authentication. Existing user external-uninstall
  behavior is unchanged.

## Independent verification

Ran with Go 1.26.10, GOMAXPROCS=3, `-p=2 -count=1 -timeout=3m`:

```sh
go test ./install/integrationctl/agentplugins/clients/chatgpt \
  ./cli/internal/agentpluginscli ./cli/cmd/agentplugins \
  -run '^Test(ProjectChatGPT|ChatGPT)'
```

Results: client PASS **0.009s**, internal CLI PASS **0.074s**, production CLI
PASS **0.522s**. `git diff --check` on the frozen leaf: PASS. Reviewed the
changed production functions and surrounding scope, saved-target, locked
manifest, sync and transfer validation paths.

These are inert production-composition and refusal contracts. No ChatGPT
remote API, native client, MCP server, model, app registration or account/
workspace mutation was executed. There is no native project proof to infer
from an unsupported leaf. Independent R2 and applicable CI remain separate.
