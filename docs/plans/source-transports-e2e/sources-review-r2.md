# Independent sources plan review — round 2

Reviewer: client-plans lane; 2026-10-23. Reviewed the post-R1 source plan,
`sources-review-r1.md`, current acquisition/runner, source identity/binding,
saved-add and dependency staging code. Official Git environment/configuration
references were rechecked. No implementation performed.

Verdict: **two small contract fixes required; architecture and R1 fixes GO**.
These do not justify new abstractions, a second engine, or broader test matrices.

## P2 — freeze archive stream termination before assigning the archive lane

The plan requires validation of trailing/concatenated content but still calls
it a policy rather than defining one. Go gzip normally permits concatenated
members, and TAR readers can report end-of-archive before consuming a second
stream. Without a frozen rule, a worker can legitimately checksum bytes that
the extractor never assessed, while another worker assumes every member was
validated. This is an input-contract gap, not a demand for more formats.

**Fix:** select one strict first-delivery policy: one ZIP archive; one TAR stream
with only defined zero block padding after its end marker; one gzip member
containing that TAR stream, rejecting an additional member or non-padding
trailing payload. Define any ZIP preamble/trailer handling explicitly rather
than accept self-extracting/polyglot containers by accident. Bound all retained
padding by the existing byte limits. The archive lane tests exactly these end
conditions, CRC/truncation, valid conventional padding, concatenated gzip/TAR,
and payload trailing an otherwise valid archive. Equivalent equally strict
rules are acceptable if frozen before implementation.

## P2 — define source-recognition and auto-classification precedence

The grammar table includes provider browser URLs, Git `.git`/fragments,
recognized archive suffixes, and HTTPS sniffing. It does not state recognition
order or when classification becomes irrevocable. A provider-hosted release
archive must not be parsed as a browser repository; a positively recognized
but malformed archive must not be retried as Git. Conversely a private
extensionless Git endpoint's 401 must reach native Git credential handling
without leaking authentication to the preliminary downloader.

**Fix:** freeze parser precedence and the auto-classification decision tree:
explicit source scheme/selector first; recognized complete archive shape before
provider repository/tree shape; all other HTTPS is bounded auto-classification.
After positive archive identification, corruption/manifest/selection errors
stay archive errors with no Git fallback. For archive auto-detection, preserve
the acquired bytes/snapshot across candidate selection so no second download
can change the selected revision. Fallback native Git only after a non-archive
response/classification, with the existing sanitized errors and credential-free
HTTPS downloader. The exact ordering can differ if it has the same unambiguous
outcomes and is documented.

Required focused regressions: GitHub/GitLab-hosted release archive;
extensionless valid and corrupt archive; private extensionless clone endpoint;
HTML rejection then native Git failure; multi-candidate archive selection with
one HTTP request; and no helper/auth/header forwarding during the HTTP probe.

## Accepted components

| Component | Assessment |
| --- | --- |
| One pure source grammar + transient browser suffix | GO; parsing remains offline and final selector/path comes from one network-layer resolver. |
| Generic URL/archive typed source variants | GO; three-way validation explicit, legacy GitHub bytes and signed Directory boundary preserved. |
| Frozen versioned tuple-hash source keys | GO; stable ownership separates endpoint/package from commit/checksum. |
| Lock and state source-copy seam | GO; all immutable source facts must survive repair/remove/replay. Additive version remains contingent on the stated old-reader fail-closed test. |
| R1 cache provenance correction | GO; current source identity always comes from the selected validated lock, while one tree entry stays shared. |
| R1 wrapper-independent archive paths | GO; logical paths remain stable when physical version wrapper changes. |
| Native credentials for fetch only | GO; preserve trusted native authentication, isolate checkout, no package filters/hooks/remote helpers beyond HTTPS/SSH protocols. |
| Archive limits/paths/executables/cleanup | GO after termination/classification fixes above; byte pin precedes extraction and tree pin follows standard sealing. |
| Local archive behavior | GO; explicit current local-source imperative semantics, no false portable saved URL. |
| User/project selected App integration | GO; no reconstruction of user environment during replay; one project lifecycle seam is sufficient. |
| Native transport tests | GO; real Git smart HTTP/private helper/SSH and actual archive formats prove transports, with fixtures clearly separate from real-client/native runtime proof. |

No unrelated native-client matrix expansion, scanner work, release, or provider
endorsement is needed. Send the revised contract for fix verification; parent
retains final implementation authorization.

## Fix verification — 2026-10-23

Rechecked the revised source plan. Classification precedence is explicit;
positive archive recognition is irreversible; one credential-free download is
retained across selection. The archive policy now freezes one non-ZIP64 ZIP
with exact end/comment bounds, TAR end markers plus bounded aligned zero
padding, and one gzip member with validated trailer and no extra payload.
`ResolvedGit` belongs to the acquisition layer, preserving pure grammar.
Both findings are addressed. **R2 GO.** Parent authorization is still required
before implementation; no code was written during either review.
