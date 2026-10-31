# Landing browser CI: isolate viewport cases and await rendered setup

2026-10-25. This narrow test amendment addresses the two genuine failures at
integration `c7bad1b0ee074c70e45070c706502c7f928b74b2`, tree
`be8bd8c0a93985a363cb851282fc23a9f87369d3`. It changes no landing product source,
registry/catalog/authentication behavior, workflow timeouts, retries or sleeps.

## Exact observed evidence

Landing run `40185973996`, job `121146717043`, reported 238 passing tests, two
failures and one existing skip. Artifact `12596570336` is 15,906,743 bytes,
SHA-256 `953a2a38dca97cc620802caeeb09e4e0722c904344ab1dabcde5fa93e3183f3c`.
The downloaded archive matches both size and digest. Its original bytes and
trace ZIPs remain unchanged.

- `hero-layout.spec.ts:28` puts four full navigations and independent geometry
  checks into one 30-second test. The trace completes the 1440, 1024 and 390
  viewport assertions, enters the 320 viewport at approximately 24.9 seconds,
  and reaches the test deadline before its geometry checks. It records no
  failed layout assertion for the completed cases.
- `landing.spec.ts:174` reads initially rendered sections without synchronizing
  hydration/render-frame setup. The product intentionally applies
  `scroll-reveal-ready` in one animation frame and `scroll-reveal-active` in the
  next. The class expectation ends at trace time 361245.970 ms; the retained
  after-snapshot at 361779.034 ms already contains both classes. This observation
  does not prove why the browser scheduled that frame late or excuse the failed
  assertion. It establishes the missing rendering precondition in this test.

## Exact change

1. Generate four independent named hero viewport tests by moving the existing
   width loop outside the test declaration. Preserve the default 30-second
   budget, setup, all assertions and optional screenshot name for every width.
2. In the existing below-fold reveal test, await the existing `hydrated(page)`
   helper and then two real `requestAnimationFrame` callbacks before checking
   the initial public classes and opacity. Preserve the five-second class
   assertion and the one-second opacity completion after actual scrolling.
3. Keep all other tests and product code unchanged, including reduced-motion
   and JavaScript-disabled coverage. Do not edit the separate catalog or auth
   indicator PRs.

Two independent plan rounds precede implementation. Review the frozen delta,
collect the affected browser tests and run them against the production artifact
when the local browser/dependencies are available. The next assembled hosted
Landing run remains the full gate; trace analysis and test collection alone do
not establish that the revised browser assertions pass.
