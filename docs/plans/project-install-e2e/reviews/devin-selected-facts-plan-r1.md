# Selected-client facts follow-up — independent plan R1

Reviewed frozen documentation-only `c48bbc58` in `work/uap-project-windsurf` on
2026-10-23. **GO** for the narrow selected-client exclusion, after both plan GO
reports are committed. No correction source or native operation was performed
by this reviewer. The unfinished later adapter is outside this plan review.

Independently traced dependencyInstaller.Prepare: it selects the primary client
from the detected map and builds Request.ClientID/ClientConfigRoot/executable/
ClientVersion before populateDependencyTargets. The collector currently loops
every existing binding and duplicates that selected primary into KnownTargets.
The published Request comment explicitly reserves KnownTargets for unselected
installed siblings. Its knownTargetIndex correctly requires each sibling's
clean absolute root and exact BindingID; weakening that separate boundary to
admit a legitimate absent primary profile would be the wrong correction.

The planned early exclusion compares only binding.ClientID to the already
selected Request.ClientID before sibling lookup/collection. All unselected
binding facts, missing-sibling errors, executable selection, binding IDs and
strict knownTargetIndex checks remain unchanged. No new empty-root exception,
version field, group behavior or user capability is added. For the current
single-client dependency request there is no need to duplicate selected facts.

Independently traced SDK detectedClients and compatibilityChecks. The primary
continues through Request reconstruction under the reviewed exact engine/scope/
namespace admission. Compatibility uses the Request's primary root/executable
directly when binding.ClientID equals req.ClientID; only a different binding
looks up KnownTargets and compares its owned ClientBindingID. The subsequent
compatibilityBindingCheck restores the prepared target.Client for the selected
client, retaining its fresh captured version/authority. The duplicate selected
KnownTargets entry supplies no extra selected-binding authority there. Existing
installation/binding/environment selection, package pin, target/native object,
profile and namespace validation still guard the mutation.

The proposed verification covers the actual composition defect: current-only
no-legacy-profile public saved install must continue through locked sync,
repair, update, transfer and removal. The focused builder regression must show
the selected client absent from KnownTargets while an exact unselected sibling
remains; existing missing/invalid sibling profile, stale BindingID and separate
executable compatibility tests must stay intact and pass. The earlier genuine
failure is valid author evidence, while final execution proof belongs to the
frozen correction and adapter implementation review.

No blocking plan issue found. This permits no profile guessing, default HOME,
version-dependent cleanup, native process, trust/model/account/MCP action or
discovery/runtime qualification. Freeze the correction with the final adapter
for both independent implementation reviews; root owns publication.
