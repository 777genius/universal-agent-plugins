# Private N1 acquisition contract

`ensureBinary(product, options)` is an explicit internal API. `product` is the
fixed shim constant `agentplugins` or `plugin-kit-ai`; options supply absolute
`packageRoot`, existing owner-only `cacheRoot`, exact `target`, and a local
`candidateRoot` for cold acquisition. Optional `signal` cancels acquisition;
`lockOptions.timeoutMs/pollMs` bound cooperating waits (defaults 30 s / 50 ms).
There is no environment dispatcher or npm lifecycle hook in N1. N2's fixed
launcher supplies these options; see [private packaging](README.md).

The package contains canonical, bounded `package.json`, `candidate.json` and
`private-release.json`. Package name/version/private flag and the sole product
bin mapping `bin/<product>.js` are checked. The descriptor has exactly:

```json
{
  "schema": "dual-authoring-npm/v1",
  "product": "agentplugins",
  "npm_package": "universal-agent-plugins",
  "identity": {
    "repository": "777genius/universal-agent-plugins",
    "commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "engine_revision": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "versions": { "agentplugins": "0.1.23", "plugin-kit-ai": "2.0.0" }
  },
  "asset_scope": "linux-amd64-pair",
  "authoring_mode": "vertical-slice-v1",
  "candidate_sha256": "<independently retained canonical candidate digest>"
}
```

This example explains fields, not literal canonical fixture bytes. Encoding is
exactly the existing candidate `encode` helper. Identity/schema/archive helpers
remain owned by the candidate producer. Expected mode is mandatory and compared
exactly. Historical callers default to `vertical-slice-v1`; N2 supplies the
explicit `expectedMode: "release-cli-contract-v1"`. Each rejects the other mode.

Trust starts at the controlled builder and independently retained pack/candidate
digest. The descriptor cannot authenticate a malicious replacement package.
N1 never executes assets or parses Go build info. N2 must require the existing
trusted staging verifier and committed wrapper blob closure at the integrated
source SHA. Candidate status remains CANDIDATE and release_eligible remains false.

The selected source file is frozen once, after acquiring the target lock.
Source files/root must be sealed; candidate basename restrictions are retained,
while ancestor/package/cache paths allow spaces. Exact source entries and
manifest bytes are checked. Raw pins agree; archives verify compressed pins,
existing canonical bounded `unpack`, then inner pins. No network or v1 fallback.

Cache keys include schema/mode, candidate digest, product/version, target and
binary digest. Roots cannot overlap package/candidate inputs. Every cache
consumer, including warm hits, takes the same finite, non-stealable lock and
rechecks ownership, directory ancestors, regular type, link count, size, hash
and POSIX 0755 mode. Ordinary owned corruption is repairable; unsafe objects are
preserved. Warm success requires current embedded identity, but no source root.

One core transaction reserves an exclusive stage, closes/reverifies it, uses a
bounded quarantine/rollback and reverifies publication before success. Cleanup
compares held inode identities; failures report rollback/cleanup uncertainty and
retain inspectable debris where cleanup cannot be proven. It is a private,
quiescent same-user namespace with cooperating consumers: no continuous path
availability, crash durability, hostile same-user or native platform claim.

Default agentplugins retains historical metadata/evidence and acquisition-before-
commit-lock behavior. Its warm hash-only policy and exports stay unchanged.
Shared compatible fault fixes reject malformed declared lengths, turn malformed
redirects/premature stream closes into settled errors, close before download
failure, refuse changed cleanup identities, and report commit/rollback cleanup
failures. No defaults, public package/workflow metadata or plugin-kit v1 files
change. N2 owns packs, launchers, signals and native journeys after A/B integration.
