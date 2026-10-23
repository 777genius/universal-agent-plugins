# Archive implementation independent review R2

Reviewed 2026-10-23 by sources_lead. Scope: cumulative archive component through `b99a15a497b1d0fb916a62965c0a33568ab9988f`, including the two reviewed compatibility amendments and R1 suffix fix. The review reads production extraction, framing, transport, package selection and cleanup code independently; it does not reuse the archive author's conclusions.

## Decision

**R2 GO.** No remaining correctness, containment, provenance or format-policy blocker found. R1's initial suffix/magic mismatch finding is closed by the narrow original-endpoint check before stream publication; extensionless and signed redirect handling retain their intended behavior.

## Reviewed boundaries

- The retained stream is bounded, hashed exactly once before extraction, and checked against requested/locked original-byte SHA256. A recognized archive never falls back to Git after corruption, unsupported framing, selection or pin failures. HTTP status/nonarchive classification is separate from explicit archive intent.
- Initial intent is query-free HTTPS; transient redirect targets preserve bounded signed query encoding while rejecting userinfo, fragments, controls and downgrade. Every hop clears headers, client cookies/callbacks are excluded, errors omit transient URLs and bodies, and only the original endpoint is persisted. The default transport and injected ordinary TLS transport both enforce certificate verification.
- ZIP central/local/descriptor records, counts, method/flags, CRC/size checks and contiguous regions reject preambles, undeclared gaps, ZIP64, overlapping records and trailers. TAR audits raw record framing before standard decoding, including effective PAX size, metadata limits, two terminal zero blocks and aligned zero padding; gzip validates one member with no outer tail.
- Extraction rejects links/special files and uses exclusive rooted writes. Exactly one conventional `./` prefix is removed before NFC/portable path checks and collision tracking. Implicit directory, Unicode/case and file-versus-directory collisions share one policy. Entries, depth, file size, expanded bytes and raw/gzip stream expansion are bounded independently.
- A provider wrapper is transient. Logical selected paths exclude that wrapper, root manifest presence has precedence, explicit subpaths remain logical, and candidate diagnostics are bounded without another HTTP acquisition. The unchanged standard loader remains manifest authority; package code is never executed.
- Context, bounded timeout, private stream/extraction cleanup, snapshot cleanup on pin mismatch and caller ownership of the sealed snapshot remain explicit.

## Verification

The source integration additionally passed a real HTTPS CLI lifecycle matrix for public Git, credential-helper private Git, ZIP, TAR, TAR.GZ, TGZ and extensionless archives: saved add, stable-owner update across a changing archive wrapper, repair, cold locked restore and repeat sync. Native agent process responses are inert fixtures; this is source/lifecycle proof, not real agent certification. Full independent archive and native acquisition package tests are recorded below when completed.

Independent complete source acquisition package tests passed in **6.107s**, Go 1.26.10/GOMAXPROCS=3, with native loopback execution. This includes the full hostile archive/format/stream-limit suite and actual Git HTTPS/private/SSH policy fixtures. R2 GO remains confirmed.

## Repeated cancellation correction

A later repeated run exposed a P2 cancellation return-shape race: a canceled HTTP body could end with clean EOF, after which framing validation reported truncated ZIP instead of context cancellation. R2 was narrowly reopened for this finding. Corrective archive commit `09131147` checks context unconditionally after retained/gzip copy and close, before unpacking, and during gzip expansion. The existing assertion is unchanged. Independent `TestArchiveCancellationAndDeclaredDownloadLimit -count=100` passes; **R2 GO restored** after this fix.
