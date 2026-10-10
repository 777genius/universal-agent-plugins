# Plain ExactFile

`Kernel.BeginPlainExactFile` opts into a private Darwin arm64 guard under the
existing writer lock. Ordinary `BeginExactFile` and `FileIO` retain their behavior.
Custom IO and unsupported metadata/platforms permit locked planning and unchanged
noops, but refuse changed mutation. No client adapter or platform tuple is enabled.

Changed writes require owned writable single-link regular files and an invariant
held parent on writable local APFS, null ACL/zero security GUIDs, flags zero,
ordinary mode bits, and empty xattrs or sole bounded opaque `com.apple.provenance`.
Actual staging and restoration must naturally match original security/provenance;
the backend never sets, copies, strips or repairs metadata. Absent creation binds
its natural metadata. Rollback requires the verified produced identity and epoch.
Drift, query errors or ambiguous durability retain output with `FileUncertain`.

Repeated descriptor/name/security checks leave a final external syscall race;
this is not linearizable byte CAS. Admission assumes well-formed native APFS and
complete access to documented metadata channels. Native qualification is pending;
Linux compilation and source handoff do not grant native or release availability.

The fixture-dependent Darwin operator tests use `-tags=nativequalification`.
They require the independent TEST witness and control roots; ordinary package tests do not supply these inputs.
