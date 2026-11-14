# SDK documentation expansion

## Goal

Give integrators a public, navigable SDK section under `/docs/<locale>/sdk/` that explains **which SDK to choose**, rather than relying on unpublished/generated API references.

## Scope

- Publish an SDK overview and focused integration guides for Go hook runtime, hook host detection, Go installer facade, and TypeScript management client.
- Ground versions, public imports, examples and limitations in actual `sdk/`, `install/integrationctl/agentplugins/installer/`, and `sdks/typescript/` source.
- Publish all new routes in nine maintained documentation locales, with navigation, cross-links and correct publication boundary.
- Distinguish published module/package versions from source-only capabilities; never conflate install success with native activation.
- Check generated nav, links, locale parity, code snippets and E2E docs CI; fix review findings before merge.

## Out of scope

No changes to installer/SDK/runtime behavior, generated API reference by hand, release tags, package publication, registry, or non-doc product UI. Do not reopen the VS Code implementation freeze.

## Review checklist

- Every public API/example resolves to checked source.
- CLI vs Go runtime vs local installer vs TypeScript management boundaries are explicit.
- Exact version/availability constraints and recovery/confirmation precautions survive translation.
- Nine locales and sidebar paths are complete; links go only to published docs or honest repository source.
- GitHub Actions `docs` workflow passes; an independent self-review finds no blocking issues.
