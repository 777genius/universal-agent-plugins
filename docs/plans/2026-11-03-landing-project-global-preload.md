# Landing: Project / Global showcase and delayed video preload

## Scope

Implement the owner-approved monitor/laptop layout, a Project / Global segmented switch, distinct copyable command flows, output below commands, lossless TOML highlighting, and Mux cache warming seven seconds after page navigation without playback. Preserve keyboard/focus/fullscreen handling, mobile layout, reduced-motion support, and all nine locales. No new dependencies, CLI semantics, release, security-policy bypass, or VS Code changes.

## Contract review

- `install <source>` delegates to add. A positional `marketing-kit@0.1.0` selector is not supported; exact versions belong in `uap.toml` and `uap.lock`.
- Argumentless `install` detects a project only from a regular `uap.toml` in the current directory. The project example therefore uses explicit project scope on the first command, then plain install on the teammate's machine.
- Global export/import transfers the entire selected user agreement. Keep the replacement confirmation warning and never add `--yes`. Account sign-ins/secrets are not transferred.
- The published Marketing Kit/dependency policy is currently user-only. The UI must identify Project as an example requiring a project-qualified release and must not weaken the actual catalog's existing fail-closed guard. This landing change is not a claim of native project qualification.
- Preload uses a non-autoplay, inert, non-focusable iframe. User activation switches to the existing autoplay=any launch, retaining Mux's sound/muted browser-policy fallback. Cache warming does not guarantee that every mobile browser buffers video bytes.

## Acceptance

Run the existing registry/type/lint/i18n checks, full browser suites and both static base paths. Cover both modes/themes, narrow screens, clipboard success/failure, mode changes during typing, highlighted-text round trips, no central version badge, delayed preload/no early playback/early click/unmount cleanup and live desktop/mobile Mux playback. Retain independent security review and current-head required CI before merge; verify Pages and production afterward.
