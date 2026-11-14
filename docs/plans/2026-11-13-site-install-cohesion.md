# Site installation coherence after CLI 0.1.108

## Goal and current evidence

Make the public website's agent discovery, plugin selection, installation commands,
and explanatory text agree with the released CLI and the authenticated catalog.
The owner authorized implementation, PR review, merge, deployment, and production
verification on 2026-11-13. CLI and npm latest are 0.1.108. Starting main is
9a67633b3ca4305e247c5f5becc9aa437c73f4be. No overlapping frontend PR is open.

The published Directory is sequence 47. The existing Registry publication run
43496820010 owns promotion of the merged Context7 0.2.5 repair. Observe that run;
do not duplicate its signing or publication. Pi modern-feed work remains owned by
Registry #451 and UAP #900/#902/#903/#904.

## Scope and design

1. Reuse the signed resolver, generated adapter capabilities, and existing exact
   Pi Project source helper. Add one small presentation availability helper that
   distinguishes signed installation, guarded Context7 Web preparation, and the
   existing pinned Pi Project source. No synthetic signed grants.
2. Share that policy between Hero, card, detail, filters, agent projections and
   counts. Preserve peer-target compatibility, Desktop/Codex exclusivity, Web
   trust and source identity checks, Directory expiry, and Pi source exclusions.
3. Keep legacy query `client=chatgpt` as Web and label it ChatGPT Web. Introduce
   `client=chatgpt-desktop` only as a UI filter identity. CLI target spellings stay
   `chatgpt` for Desktop and `chatgptweb` for Web. The existing /agents/chatgpt/
   page aggregates Desktop and Web, matching its existing public description.
4. Derive usable agent-page examples from eligible catalog packages. Clearly
   identify own-package prerequisites when no catalog example exists. Explain
   supported Skills/source paths without implying unavailable catalog grants.
5. Make final-choice and terminal-selection help accurate in all nine locales.
   Preserve the no-auto-option requirement that one choice remains selected.
6. Deploy the accepted tested site source using the repository's existing
   production pin and Pages workflow. Confirm the live source and public routes.

No new installer, resolver, feed, CLI release, generalized Git fallback, dependency
upgrade, frozen VS Code behavior, or broad redesign is part of this frontend fix.
The site must communicate current capability honestly; an absent published grant
must not be presented as a global lack of agent support.

## Estimated handwritten changes

| Work | Production lines | Test lines |
| --- | ---: | ---: |
| Surface-aware discovery and consistent route presentation | 45–95 | 55–120 |
| Shared guarded Context7 Web preparation | 35–75 | 50–100 |
| Selection help and fallback behavior | 4–10 | 15–30 |
| Agent-page examples and availability explanation | 25–70 | 30–80 |

Allow 30–90 locale lines. Shared policy extraction may move existing code; count
unique meaningful changes separately from moves, fixtures and generated output.
Complexity 4/10; expected reliability 9/10; estimate confidence 8/10.

## Critical reviews

- Round 1: independent frontend and installer reviews confirmed the existing
  resolver/helper boundaries and rejected a generalized Git or new-feed expansion.
- Round 2: fixed query compatibility, aggregate ChatGPT projection/count semantics,
  scoped Pi discovery, mixed-target Web guards and source-kind presentation.
- Round 3: independent implementation review before merge; correct findings and
  inspect the exact final commit. Do not treat queued/skipped checks as success.
- Late review confirmed mixed guided Web setup has no Web receipt until the
  separate registration step. Preserve its unsaved add command and explicitly
  restrict maintenance to local peers; signed Web lifecycle remains unchanged.
  Verify exact command strings in unit and desktop/mobile browser coverage.

## Verification and completion

- Targeted helper/filter/projection regression cases and the existing registry suite.
- Locale validation and generated locale inventory; lint and Vue type checks.
- Browser journeys for Hero → card → detail, Desktop/Web filters and flags,
  Pi User/Project distinction, saved choices, final-choice deselection, agent
  examples, mobile and RTL. Reuse existing browser harness and CI artifacts.
- Successful applicable PR checks and independent review before merge.
- Advance only the tested immutable production source; wait for Pages and its
  public readback verification. Verify live routes and actual deployment identity.
- Follow existing Context7 publication through its genuine gates and confirm the
  published immutable release, superseded old release and unchanged upstream default.
- Report remaining external protected review requirements only if an actual gate
  requires an owner action; queueing or runner delays are not a blocker.

## Development workspace containment

During final qualification, Dependabot reported that locked `@nuxt/schema` 3.21.11
is affected by [GHSA-mjrq-cffv-q4qc](https://github.com/nuxt/nuxt/security/advisories/GHSA-mjrq-cffv-q4qc).
Nuxt's automatic workspace detection can make a development server's file access
too broad when a parent directory contains a stray workspace or lock file. The
advisory explicitly excludes production builds.

Apply the vendor's workaround, `workspaceDir: '.'`, in the landing configuration.
Nuxt resolves this relative to the project root, independent of the shell's current
directory. Regress the actual locked resolver against an unrelated parent workspace.
This mitigates the configuration exposure; it does not replace the affected package
with a patched version or imply that the dependency alert is closed. A Nuxt major
version migration remains a separate change with its own toolchain qualification.
