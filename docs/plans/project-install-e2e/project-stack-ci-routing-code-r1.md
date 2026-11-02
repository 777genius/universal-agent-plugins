# Project stack CI routing — independent implementation R1

Reviewed frozen implementation `7a3513673451c93a3d0e0e332e6719886c9d1f4c` against its parent on 2026-10-23, after the separately reviewed routing plan. **GO.**

The entire commit changes exactly two branch-filter lines: native client pull requests gain `feat/project-*` and `feat/source-transports-e2e`; core replaces the exact project feature base with the superset `feat/project-*`, retaining main, source and `refactor/**`. This implements the approved additive routing and admits the task-owned flat client/checkpoint bases.

No path filter, event, job, matrix, action pin, permission, timeout, checkout/source selection, credential policy, native guard, release restriction or assertion changes. The real disposable-host/native-platform guard remains untouched. The implementation introduces no executable/native logic that needs additional local execution; parent independently reports the existing native matrix contracts passing (18 tests). Actual exact-head hosted runs remain required evidence and are not implied by this routing GO.
