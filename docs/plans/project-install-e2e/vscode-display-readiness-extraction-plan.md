# VS Code display readiness complexity correction

Final assembled source `1a19373e024ba4c2ac0748fda1306ce0ffd95429`
failed the configured Required lint gate because `displayReady` has cognitive
complexity 28, above the limit of 25. This amendment is a behavior-preserving
extraction inside the existing reviewed collector. It adds no evidence or
native qualification.

## Exact change

Replace only the body of the already-entered `se == nil` branch in
`displayReady` with a call to `c.retainReadyDisplay(lock, lockInfo, info, proof)`.
Move that exact body into the existing `display_authority_linux.go` file:

1. Call `validateDisplayReady` with the same lock bytes, captured lock/socket
   metadata and isolation proof.
2. Repeat the same socket `Lstat` and `sameFile` comparison with the captured
   socket identity; preserve the same refusal text.
3. Call `holdDisplayAssets` with the same two fixed paths and metadata. Preserve
   its O_PATH/no-follow/CLOEXEC, held/named/pair verification and partial-close
   behavior without modification.
4. Assign `c.displayAssets` only after that call succeeds, then return nil.

Every existing branch and error return outside that branch remains unchanged.
The ten-second timer, 25ms ticker, context cancellation, missing-file retry,
probe order, syscall order and teardown remain unchanged. No runtime flags,
privilege, namespace, sandbox, qualification, test assertion or lint exclusion
changes. The helper is called only after the original lock/socket presence
conditions have succeeded.

## Verification and review

Run the complete affected package's existing inert readiness, descriptor,
replacement, teardown and cleanup suite in a private Unix-socket-capable
executor, plus vet. Run the configured full-package correctness, size and
architecture lint without a changed-line filter. A mirror test for a pure
extraction is unnecessary; preserve the existing meaningful tests verbatim.

Two independent reviewers must approve this exact plan in both review rounds
before source editing, then review the frozen source diff. The final hosted
native metadata replay remains independently required at the next source.
