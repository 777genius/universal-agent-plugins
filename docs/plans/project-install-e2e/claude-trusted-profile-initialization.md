# Claude trusted project fixture: initialize before the byte baseline

## Observed failure and primary evidence

The native run at source `d6809d5438654ccdb47bd8d241b96b44e91515c8`
(`40187422616`) fails the new trusted fixture's whole `.claude.json` hash
guard on Windows (`121151065174`) and macOS (`121151065203`). The original
four native tests pass. The preserved macOS public-add result is already
Active + Installed; the Windows result is Prepared with explicit native
project suppression. Neither artifact contains the post-native profile, so
the actual changed fields are not yet established by those runs.

The original Windows artifact is 416,557 bytes with SHA-256
`dc4bcfa000362add4dbff64cfc7f1b252a8d7fa8ce261ab453097c2763dbabec`;
the macOS artifact is 402,554 bytes with SHA-256
`c0a0f68ff1f68c30e58a78022135abee5a2e98be087af6306bfcd2476fa75e8a`.
Both exact outer archives were independently checked before inspection.

Static inspection uses the official pinned npm package
`@anthropic-ai/claude-code-win32-x64@2.1.263`, without executing it locally.
The archive's SHA-512 is
`P1LnudAWj2ptyE0IZzCZKAe7nU7LxqlkcLdBfp8FsHlH5dp75Og4JJ6TClvfjanHn3shYW4CYN6oI1vSqZlkHw==`;
its native executable has SHA-256
`0b35df94c1307004f07b738390bfef8dfca5e9af29aaf6517f305bf086b95b03`,
matching the matrix pin and measured native artifact. Its embedded source
shows:

- `h$` normalizes paths and replaces Windows backslashes with forward slashes;
  `G1e`, `vde` and parent trust lookup use these normalized project keys.
  The failing Windows fixture authored native backslash keys instead.
- Global configuration lives at `CLAUDE_CONFIG_DIR/.claude.json` in the
  production suffix. The fixture selected the correct original profile.
- `init` invokes `HCn` and `SCe`. `HCn` writes missing `firstStartTime` and
  `firstStartVersion`; `SCe` writes a missing generated `machineID`. Both
  return without an update once their fields exist. Native configuration
  merging preserves the authored `projects` entries.

These pinned write paths support a finite initialization experiment. They do
not prove which fields changed in either existing hosted failure, or that all
future native commands are byte stable. The corrected test must measure both.

## Frozen change

Only the separate trusted Claude fixture and narrow inert helper contracts
change. Production adapters, the existing untrusted fixture, containment,
environment, pins, timeouts and native command arguments remain unchanged.

1. Author both exact canonical A/B project trust entries using
   `filepath.ToSlash`, in the same original disposable `CLAUDE_CONFIG_DIR`.
   Retain the authored raw profile and its hash as evidence. Do not add this
   file to the whole-byte foreign map until native initialization finishes.
   All existing user, A/B, settings, disabled foreign skill and cache byte
   sentinels are guarded throughout initialization.
2. Before any public UAP operation, execute exactly one bounded genuine
   `plugin list --json` in A and one in B through the existing capture helper.
   Save raw streams and command receipts. Require a unique whole JSON array,
   no managed fixture candidate and no suppression sentinel; require all
   existing foreign hashes and both exact authored trust entries after each
   command. A malformed/changed trust entry fails. No retries or new flags.
3. Read the resulting disposable profile as one ordinary nonsymlink regular
   file with a fixed 1 MiB limit. Save the exact post-initialization bytes and
   authored/post-initialization hashes, plus bounded top-level changed-key
   names, as evidence. Never rewrite or normalize the native-produced file.
   Add its current hash to the foreign map exactly once, before public UAP.
4. Execute one fixed second A/B empty-list control using the same strict
   validators. The whole profile baseline and every existing foreign hash
   must now remain byte exact. This is a finite stability assertion, not a
   retry-until-stable loop. If native behavior remains unstable, fail before
   claiming or attempting public installation.
5. Keep the existing public add, exact Active identity, genuine A/B listing,
   details/skill bytes and remove assertions. Every subsequent operation and
   observation retains the complete profile byte guard. Never restamp the
   baseline after an installer or later native command. Preserve failure
   evidence and the existing absence, suppression and scope checks.

No original user profile is copied or exempted. Only the new test's authored
disposable profile undergoes native initialization before its single byte
baseline; this does not authorize production trust writes or change the
untrusted-project admission contract.

## Validation and review

Use inert contracts for exact authored trust-key/type/value admission,
malformed/duplicate/trailing JSON and wrong native empty-list identities.
Compile the existing opt-in native suites without executing vendor code.
Run configured changed-source lint and diff checks. Two independent plan
rounds and frozen code review precede publication. The next genuine hosted
native matrix must establish actual Windows/macOS initialization deltas,
stable second controls and complete public add/remove behavior; source-only
checks cannot qualify native discovery.
