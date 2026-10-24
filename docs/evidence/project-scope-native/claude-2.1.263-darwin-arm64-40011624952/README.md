# Claude project installation failure evidence

The genuine pinned Claude Code 2.1.263 macOS arm64 project subtest failed at
production UAP add/activation. The three other user/runtime subtests in the
same job passed, but do not qualify this new project lane.

See [provenance](provenance.json) for the exact commit/tree, official executable
pin, ZIP digest and run/job links. The three original project sub-artifacts
were verified against their summary and preserved byte-for-byte.

The returned error says the plugin list is unrecognized. No raw project list
was captured before the test stopped. This evidence therefore establishes the
failure, not its parsing cause. A separately reviewed diagnostic-only amendment
will capture bounded read-only A/B lists after the first installer error while
retaining this failure, all parser assertions and the disposable-host guard.
Further repair/update/removal stages remain not evaluated.
