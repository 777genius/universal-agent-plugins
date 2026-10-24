# Selected Devin facts are not sibling facts

Pre-code narrow follow-up, 2026-10-23. The two reviewed host-evidence/absent-profile
contracts and original client plan remain in force. No implementation of this
specific correction exists at this checkpoint.

The new real TLS Git/ZIP public CLI regression succeeds on first saved Devin
project install with no legacy Cascade profile, then fails during saved repair:
`target facts are unavailable or conflict with owned state: ConfigRoot must be
an explicit absolute clean path for windsurf`. The source is
`dependencyInstaller.populateDependencyTargets`: it appends every binding,
including the selected request client, into KnownTargets. The SDK field contract
says KnownTargets is for installed sibling bindings not selected by the request.
Its existing stricter sibling root validation is not the right place to repeat
primary request admission.

Implement only a selected-client exclusion in that CLI collector:
`if binding.ClientID == request.ClientID { continue }`, before looking up and
appending the sibling fact. Keep the existing knownTargetIndex validation,
BindingID/executable/root requirements and all unselected sibling facts unchanged.
Do not introduce an empty-root exception or version field for sibling facts.
The primary selected Request already carries its fresh observed executable/
version and optional original root under the reviewed project-only admission.

Independent tracing of compatibilityChecks shows that selected single-target
bindings already use req.ClientConfigRoot/ClientExecutable directly; only
binding.ClientID != req.ClientID consults KnownTargets and checks its BindingID.
The selected target's prepared DetectedClient replaces its observational
reconstruction in compatibilityBindingCheck, retaining the fresh version.
The selected binding remains bound by core installation/binding/environment
receipts, namespace, target/profile/native object and immutable package guards.
No duplicate selected KnownTargets value supplies additional authority there.

Verification: retain the failing current-only no-profile real TLS saved
install/locked-sync/repair/update/cold-import/remove journey until it passes;
add an affected-builder assertion that selected facts are omitted while exact
unselected sibling facts remain. Existing knownTargetIndex user/sibling missing/
invalid root and binding identity tests stay unchanged and must pass. No native
process, new public schema, group expansion, detection order, user capability,
profile fallback or version-dependent cleanup is added.

Obtain both independent plan GO reports before applying this correction. Freeze
it with the final Devin adapter for both independent code reviews; root owns
publication. This remains prepared file-delivery evidence, not native discovery,
trust, MCP runtime, Desktop harness or release qualification.
