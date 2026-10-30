# Report prospective identity resolution errors

## Problem and reproduction

`Engine.ReserveIdentity` can return success after a prospective binding target
cannot be resolved. For a registered Cursor adapter, an explicit installation
ID and declared name with `ClientConfigRoot: "relative"` currently return that
installation ID, empty binding/path fields, and a nil error. The resolver rejects
the relative root, but the private `reserveNewBinding` helper drops its error.

This is a public SDK reporting defect. No production caller of `ReserveIdentity`
was found in this repository, and `Prepare` independently validates its target;
this is not a claim of a runtime mutation-authority or P1 defect.

## Bounded change

Return the existing target resolution error from `reserveNewBinding` and propagate
it from `ReserveIdentity`. Preserve the public signature, existing-binding lookup,
and successful partial reservations when an installation/client/declared name is
intentionally omitted. Successful complete reservations retain their current
binding/path computation. Do not add a context API, planner abstraction, new
layer, persistence format, or compatibility changes.

## Verification

- Reproduce refusal for an invalid prospective target and confirm no installer
  state, staging directory, or client files are written.
- Exercise a registered adapter whose target-layout method returns a sentinel
  error and confirm callers can recognize it with `errors.Is`, if the existing
  adapter capability permits this without a new injection seam.
- Preserve omitted-input partial reservations and existing-binding lookup.
- Run the existing valid reservation/Prepare correspondence and collision
  regression tests, and focused package checks available in this environment.
- Keep Linux containment limitations explicit if they prevent an existing
  integration test from executing; do not weaken its assertions.

## Risks

A consumer that supplied all prospective binding inputs but relied on a nil error
after an invalid target will now receive the resolution error. Such a result never
provided the requested binding/path. Intentionally partial requests and recorded
binding lookup continue to succeed under the same conditions.
