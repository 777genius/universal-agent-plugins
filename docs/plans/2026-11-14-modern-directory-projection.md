# Authenticated modern Directory mirror and build projection

Status: plan for independent review; no production activation.
Base: `bec32861d0a104890f41bd8eb20463e1578a152b` (reviewed UAP #908).

## Problem and boundary

The modern Go reader can retain a well-formed future client as nonselectable,
but the site path still uses the historical reader. The mirror calls strict
`ParseSnapshot`, copies the original signed JSON, and the build then calls
`releaseTargets`, which rejects an unknown client before projecting the catalog.
There is no existing filtering step that makes a modern catalog buildable.

Complete that reader integration in an isolated change. Keep current public
keys, origins, CLI/mirror defaults, production site pin and historical signed
bytes unchanged. This does not publish a feed, create a signed target grant or
qualify an agent's native installation. Registry CAS/publication work is outside
this slice, as are the separate explicit-source UI and Antigravity repair PRs.

## Minimal implementation

1. Add a private, explicitly supplied Directory mirror profile at the existing
   composition seam. Preserve the current historical entrypoint and defaults.
   The prepared modern profile uses the existing `NewModernTrustStore` and a
   distinct reviewed public authority; it is not selected from candidate JSON,
   environment variables, a command-line flag or a query parameter. No real
   modern authority is invented or configured by this change.
2. Verify the complete original snapshot/envelope/pointer before accepting its
   modern structure. Reuse existing limits, path handling, strict parsing and
   signature verification. Any source-commit extraction needed to locate a
   pinned trust document is untrusted identity extraction, not acceptance.
   Preserve the complete authenticated bytes in the staged mirror. A modern
   Directory origin must not redirect Discovery or Security to another issuer.
3. Keep the modern mirrored namespace and rollback floor distinct from the
   historical lineage. Reuse the mirror metadata and monotonicity guard; refuse
   cross-lineage reuse and same-sequence conflicts. Historical output remains
   compatible, including its metadata without a modern lineage identifier.
4. Add a build-only modern projection entrypoint at the existing loader/codec
   seam. Reviewed composition selects it; the snapshot cannot select its own
   trust profile. Validate target identities and complete field semantics
   before projecting only generated, known client IDs. Unknown clients never
   enter `ClientID`, install commands, selection or known-client evidence.
   Preserve strict historical/default and untrusted preview behavior.
5. Keep existing known-client denials, current evidence, schema-1 User scope,
   ChatGPT-only binding, expiry, component and duplicate rules. Reuse current
   parsing rather than introducing another wire validator or authorization
   store. Unknown successful evidence cannot neutralize known failed evidence.

The existing large mirror `main.go` is shrink-only. Move a cohesive Directory
mirror helper if needed; do not grow the exemption or alter lint policy.

## Required evidence

- A real Ed25519 fixture signs a modern schema-1 catalog containing known and
  future client identities. The normal mirror verification and staged-file
  path preserve the snapshot, envelope and pointer bytes exactly. A bounded
  fixture transport supplies immutable input; this is not production evidence.
- Pass that staged snapshot through the actual TypeScript build loader and
  projection, not a second test parser. Known eligible targets remain visible;
  the future client remains nonselectable and known denials remain effective.
- Wrong signatures/keys, malformed or duplicate future IDs, invalid delivery,
  authentication, scope or binding, invalid evidence and mixed lineage/floor
  all refuse before producing usable modern output.
- The historical reader/build entrypoint still rejects the same future-client
  catalog; existing historical mirror paths and monotonicity tests stay green.
- Run focused Go and frontend tests, full frontend unit/type checks, `make lint`
  and relevant `make test-core` verification. Record host limitations honestly;
  required hosted validation runs on the final reviewed commit.

## Size and review estimate

Estimated net production change: 220–360 lines across mirror helpers, metadata
guard and build/codec seams; tests: 300–500 lines. Cohesive moves from the large
mirror file do not count as new behavior. Expected complexity 6/10, reliability
9/10 after the signed round trip and negative controls, confidence 8/10 before
implementation. Stop and review any expansion into publication, a generic
workflow framework, runtime profile selection or a new trust service.

## Concrete activation still required

After the actual issuer has an approved feed ID, separate HTTPS origin, key ID
and public key, and a genuine signed initial bootstrap, a separately reviewed
activation patch supplies those public facts and connects production
composition. It also binds the Registry's protected publication/ref/hosting
controls and qualifies real signed publication, CLI consumption and deployed
site provenance. Test keys and this prepared reader never establish that
activation or native support. Current production remains historical throughout
this change.
