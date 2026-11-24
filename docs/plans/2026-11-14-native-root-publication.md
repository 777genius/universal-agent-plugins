# Original-object publication for native Project roots

## Scope and measured failure

This correction covers the fresh auxiliary Project root published by the existing
transaction kernel. It does not grant authority over mutable runtime contents or
change any client capability, native consent, environment owner, public JSON or
journal schema.

Original Windows transaction job 132341117066 in run 43559930238, source
5ff83587db83f613ebd8c784d2032f24e46b6bdb, fails when the missing-parent layout
renames a staged parent while the selected descendant directory remains open.
The existing-parent case passes. Current source 4cd5b613ae5948742231ed0e7babe67b9c3a1c81
reproduces this in job 132358402052 of run 43565523959. These are failures, not
native acceptance.

The Windows filesystem rename specification requires rejecting a directory
rename when a descendant is open. Go 1.26.11 already shares DELETE for rooted
child opens; changing that share mask is insufficient. References:

- https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fsa/87f86c9b-6c2a-4803-84b7-131a74a434fa
- https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fsa/133840e4-778e-44ca-9b41-da2323615075

## Original authority and handoff

Before releasing any descendant, retain the final marker stat from the original
handle after Write and Sync. Keep that original marker handle live through the
constructor; its post-write FileInfo alone does not prevent inode reuse before
sealing. The pre-write stat cannot describe completed bytes.
Join the original held stage and selected directory to their current held stats;
directory times are capture-stability observations, not persistent rename facts.
Pass the exact closed set (stage, optional selected child, marker) to the neutral
dirswap original-object constructor. Relative names are canonical slash paths.

The immutable proof binds every original object identity as well as bounded
content and modes. Unix UID, GID and link count must match the original held
FileInfo. On Windows, original file attributes are compared, and the owner SID
is observed through a seal handle joined to the still-live original object and
required in later matches; the original FileInfo does not itself encode the
Windows owner.
Only after the proof is complete may the preparer close a distinct selected
descendant. Close the marker handle after sealing and before releasing
that descendant; neither remains open across the Windows parent rename. The
original base and stage handles remain live. A fresh
standalone DirectoryMutation forwards this same typed proof; groups and ordinary
package mutations reject a misplaced nonzero value before effects.

The directory manager must compare the supplied proof before its first journal.
It must not recapture post-close objects and call those originals. The existing
PublishedDigest carries the exact objects-v1 version; every relevant live,
rollback, quarantine and cold role match includes descendant identities.
Unknown or malformed object-proof versions refuse. Legacy unprefixed proofs keep
their existing behavior; there is no migration or automatic authority upgrade.

## Cleanup and recovery

Before the journal, only exact old state and absence of an operation journal
release staging to the preparer. A released selected directory is reopened only
beneath the still-held original stage. Its original FileInfo is compared before
reading or deleting anything and is never replaced by a new capture. Same-byte
copies of either the directory or marker are foreign and must survive.
Cleanup remains nonrecursive. Before sealing, retain the marker handle through
its exact removal, then close it before removing the selected directory; a
missing marker also releases only that original handle. Error cleanup closes
retained handles without adopting any replacement.

Once an operation journal exists, the kernel retains ownership of reconciliation.
A copied child or marker at a pending or active role must retain the exact journal
and foreign objects across fresh-kernel retries. For the new proof, inspecting
only active and quarantine paths would miss a changed unactivated staging tree;
the pending staging role must also be checked before dropping a journal.
The ordinary native-root-v1 digest, parent ancestry, marker bytes and committed
receipt remain mandatory for later reuse of the published runtime root.

## Review and verification status

R1 was independently accepted before implementation. The approved neutral
adapter eight-path change and this transaction eight-path change are separate
review units. Both exact source units received independent R2 acceptance; source
review is not execution evidence. The focused runtime file keeps the handoff
cohesive within the existing file and function limits. No callback, production Fault hook, unsafe
code, broad exception or new public schema is introduced.

The first local application was interrupted by execution-environment failure
before the first source update. The review candidate was subsequently authored
from immutable GitHub 4cd source using the official Git Data API; it is a new
candidate, not a claim that interrupted local operations completed.

The candidate still requires actual formatting/lint/core tests,
all affected transaction rollback/recovery tests, genuine Windows execution and
the affected Hermes native Project lifecycle. Authored tests, successful
cross-compilation and old-source CI do not satisfy those gates.
