# Devin shared host evidence prerequisite — independent R1

Reviewed frozen `64c7ee6d48084da6247a200eb3e77e8e6eb0dc29` after the separately
approved two-GO amendment. **GO for this shared prerequisite.** Reviewer did not
author the implementation. This is not a review or qualification of the later
Devin project adapter.

The production delta is narrow: an optional operational Request.ClientVersion
string with json exclusion, copying it to the reconstructed DetectedClient,
selected.Version propagation from each of the two existing CLI installer request
builders, and permitting an absent original user ConfigRoot only in a configured
project engine. There is no new subprocess, probe, detection ordering, registry
row, native capability, portable schema or group expansion. A request cannot
obtain the operational version through Request JSON. The immutable prepared
request copies the string by value, so a caller changing its original request
cannot replace captured version evidence.

Public Prepare validates exact request/engine scope and project root, declared
project support and the real namespace before profile capture or reconstruction.
The empty-profile exception is therefore confined to an already validated
project engine/request. User scope still requires a profile, every nonempty
profile must remain absolute and clean, and existing ProfileResolver hooks still
run. A project root is never substituted for the missing user profile; later
adapters must retain their exact original-context and native-layout checks.

The saved request builder preserves fresh selected evidence for install/update/
repair, and the no-save facade adds the same selected metadata without widening
its supported-client route. The existing no-save usecase already retains its
DetectedClient directly. Group/known-target DTOs are unchanged under the approved
Claude/Codex-only group boundary. Removal and status retain their existing
recorded authority and do not acquire a new version requirement or process.
Absent evidence stays unknown. Supplying this host field does not establish
binary provenance, native discovery, effective session policy or runtime proof.

Independent verification at an isolated exact-commit checkout:
`go test -p=2 ./installer -run '^TestProject(PreparePreservesSelectedHostVersion|HostEvidence)' -count=1`
passed in **0.010s** using the prescribed existing toolchain/cache. The tests cover
prepared value capture, JSON exclusion/injection refusal, unchanged original
profile, invalid nonempty project profiles, user empty-profile refusal, legitimate
generic project absence and stricter Gemini resolver refusal. An initial attempt
in the author's actively edited worktree overlapped unfinished later adapter
source; it was discarded and no conclusion drawn from that non-frozen build.

The full Devin adapter still needs exact selected executable/version admission,
both public paths and SDK lifecycle coverage, missing/older/wrong-host refusals,
current-only absent-profile behavior and two independent implementation reviews.
No vendor process or native mutation was executed during this review.
