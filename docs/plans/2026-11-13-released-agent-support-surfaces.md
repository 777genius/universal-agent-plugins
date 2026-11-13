# Released agent support presentation — 2026-11-13

## Goal

Align README, public compatibility docs and landing installation feedback with
native CLI 0.1.108, without overstating package eligibility or client activation.

## Verified starting point

- Main: `4544598d0f012e96aaf208331c76c3fc3a9f0c5a`.
- Public native release: [0.1.108](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.108),
  source `8c3cfa28e332c533b0cc0845739c0275730a4285`, published 2026-11-13.
- [Release qualification](https://github.com/777genius/universal-agent-plugins/actions/runs/43337036671)
  and [promotion](https://github.com/777genius/universal-agent-plugins/actions/runs/43454712919)
  succeeded. npm publication must be checked independently.
- Production, release and main use the same client presentation/capability data:
  16 public clients, 17 plugin choices (ChatGPT Desktop/Web), 15 standalone Skills targets.
- Live Context7/Playwright disable Hermes/Antigravity because no eligible signed
  package target resolves. Skills exposes Hermes/Antigravity/Pi and generates
  `--target "hermes,antigravity,pi"`.
- Documentation still calls Hermes/Antigravity unreleased under CLI 0.1.102.
  Some native evidence summaries omit merged Grok/Kimi/Hermes/Kiro results.
- Open #878 six-client automation is unreleased. VS Code remains frozen.

## Changes

1. Correct the current release boundary in README, the project support guide,
   Hermes/Antigravity guides, and all nine published compatibility locales.
   Retain historical evidence anchors and exact OS/client/surface limits.
2. Update the support-table note in all landing locales. Explain that agent
   capability, compatible catalog package release, and post-install activation
   are separate. Retain Automatic/Guided/Prepared rows.
3. Give supported local agents a clear package-specific unavailable explanation
   in hero, plugin cards and details. Preserve existing trust, expiry, Web app
   binding, scope, pinned-Pi and resolver decisions; no eligibility changes.
4. Update current native evidence summaries only for already merged source:
   Grok #883, Hermes #884, Kimi #885, Kiro #686 and OpenCode #655.
   Do not attribute post-tag #899 or draft #878 to the released binary.

## Size and risk

Expected executable changes: 20–60 lines; docs/translations: 150–300 changed
lines. Complexity 3/10, expected reliability 9/10, confidence 9/10.
Reuse existing presentation and resolution contracts. No new capability
registry, release automation, install path, dependency or public CLI changes.

## Review and validation

- Round 1: verify release/source/npm distinction and signed package eligibility.
- Round 2: independent diff review for overclaims, locale parity and unchanged
  command/selection behavior.
- Run existing landing registry tests, i18n checks and lint; regenerate the docs
  locale inventory through its existing generator.
- Exercise hero/card/detail disabled choices, standalone Skills multi-target
  commands and User/Project scope in existing browser suites.
- Build the changed public docs with the existing provisioned toolchain or
  report the exact missing prerequisite; skipped/queued checks are not PASS.
- Publish a Draft PR and keep its validation evidence tied to the final SHA.
  Production publication requires updating the existing immutable source pin
  to the reviewed source commit/tree; a main change alone does not publish it.

## Boundaries

No signed Directory target grants are added or fabricated. No new CLI release,
provider login, trust bypass or vendor-model execution. No unrelated legacy
five-target component cleanup. Public deployment remains a separate final
action from preparing and reviewing this correction.
