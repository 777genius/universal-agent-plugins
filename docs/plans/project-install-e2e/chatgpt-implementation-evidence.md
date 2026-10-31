# ChatGPT project boundary implementation evidence

The all-client plan's two review rounds approved an explicit unsupported
ChatGPT project leaf. This implementation adds no native project capability,
artifact-only preparation route, project layout, native registry or remote API.
It retains the existing user personal/manual preparation path.

## Implemented boundary

- Production composed CLI tests exercise saved/imperative additions, personal
  app registration input, activation/authentication attestation, positional
  install, mixed Gemini/ChatGPT targets, exact-ID update/repair/removal and the
  invocation-directory project default. Each returns the stable unsupported
  reason before detector, acquirer or security evaluator calls.
- Full user/project-A/project-B byte and mode inventories remain unchanged.
  The tests require no project `.uap`, lock or user state namespace to be
  created. Existing user/project markers remain intact.
- Saved project declarations containing ChatGPT are refused by deps lock,
  deps update, deps sync and environment install. The negative fixture exposed
  lock/update creating `.uap/environment.lock` before target validation; a pure
  project declaration preflight now occurs before that lock. The existing
  locked manifest reread, validation and publication checks remain in place.
- Project target validation reports project scope unsupported before manual
  user-only guidance, so it cannot suggest `--no-save` as a project bypass.
- Direct ChatGPT activation/preflight rejects project scope or a project
  TargetContext even when personal-preparation/attestation flags are supplied.
  Direct deactivation refuses unsupported project completion even with an
  external-uninstall attestation. This protects adapter reuse in addition to
  the shared provider/CLI scope gate.
- Capability tests retain user scope and assert absence of project scope,
  ProjectTargetLayout and NativeRegistryLayout. Direct user manual/prepared
  outcomes remain bounded and never become verified remote installations.

## Verification

Go1.26.10 focused test execution (`-p=2 -count=1 -timeout=3m`) passed production
CLI project ChatGPT contracts **0.576s**, client contracts including existing
exact manual removal **0.009s**, and existing Context7 guided/personal/manual
preparation/lifecycle regressions **0.920s**. Before the correction, the saved
lock/update cases reproduced namespace creation and sync/install exposed
misleading user-only guidance. The final tests retain those precise no-effect
and diagnostic assertions.

After the concrete locked recovery-inspection helper extraction, a final
focused run also included existing staged snapshots, changed-lock rejection
and all-client native recovery: production CLI **0.491s**, client **0.009s**,
and internal CLI **3.653s**, all passing. The extraction retains the prior
installer-nil and recovery-required behavior inside the environment lock.

Exact golangci-lint2.13.2 changed-code and whole-file size/architecture gates
on the affected CLI and ChatGPT client packages: **0 issues** each. No lint
baseline or original assertion was relaxed.
Independent implementation reviews and applicable cross-platform CI follow the
frozen checkpoint. These tests execute no ChatGPT remote API, native client,
MCP server, model, registration or workspace publication.
