# Private controlled npm pair

These generated packages are private preparation artifacts. They are not public
releases and carry no attestation or supported-platform acceptance. The fixed
`agentplugins` and `plugin-kit-ai` shims invoke their own pinned executable with
the caller's arguments. Both products require exactly `release-cli-contract-v1`.
They neither add `author` nor translate output. Useful v1 source remains intact.

Installation is from independently verified local tarballs only, with scripts
disabled. There are no dependencies or lifecycle scripts. Node minimums remain
22 for universal-agent-plugins and 18 for plugin-kit-ai; execution on Node 18 is
a separate unproved gate. No private package changes default npm selection.

Before invoking a shim, create an owner-only mode-0700 cache directory outside
all package/candidate inputs. Set `UAP_PRIVATE_NPM_CACHE` to its absolute path.
For cold acquisition set `UAP_PRIVATE_NPM_CANDIDATE` to the sealed local candidate
directory. Its final basename has the existing producer's restricted spelling;
ancestor, cache, install and project paths may contain spaces. A valid warm cache
works without the source. There is no download, latest selection or v1 fallback.

The cache contract is the N1 cooperating same-user quiescent namespace contract:
cold and warm users take the same finite non-stealable lock. Metadata, ownership,
size, hash and executable mode are checked. Only ordinary owned corruption is
repairable. Unsafe objects and stale-looking locks are preserved. This does not
promise hostile same-user protection, crash durability or continuous path
availability. Cleanup uncertainty is an error with inspectable evidence.

Ordinary environment and cwd are preserved. All `UAP_PRIVATE_NPM_*`,
`UAP_CANDIDATE_*` and both products' `*_INTERNAL_PROOF_*` variables are removed
from the child environment. No shell is used; stdio is inherited. Numeric status
is returned unchanged. Parent SIGINT/SIGTERM is forwarded once; after 1500 ms a
remaining child receives SIGKILL. The wrapper waits for child and stdio closure,
then removes handlers before reflecting a child signal. Only that child's PID
is signalled. POSIX fixture results do not establish Windows signal semantics.

# Maintainer staging contract

Run the opt-in stager only after root integrates N2 and controls a candidate
from that exact commit. The earlier dependency candidate cannot prove the new
wrapper source. The stager never rebuilds a native product and never executes
candidate bytes to discover identity. Its trusted existing Go verifier checks
both products' embedded main, version, revision, mode and target before packing.

`npm/agentplugins/scripts/stage-dual-authoring-npm.js --candidate /absolute/stage.json`
accepts exactly these fields (paths and pins are root-supplied):

```json
{
  "candidate": true,
  "repo": "/private/source",
  "root": "/private/inputs/candidate",
  "identity": {
    "repository": "777genius/universal-agent-plugins",
    "commit": "<exact final 40 lowercase hex SHA>",
    "engine_revision": "<same exact SHA>",
    "versions": { "agentplugins": "0.1.91", "plugin-kit-ai": "2.0.0" }
  },
  "manifestDigest": "<independently retained candidate SHA256>",
  "assetScope": "linux-amd64-pair",
  "authoringMode": "release-cli-contract-v1",
  "go": "/absolute/trusted/go",
  "node": "/absolute/trusted/node",
  "npm": "/absolute/installed/npm/bin/npm-cli.js",
  "workParent": "/private/scratch",
  "output": "/private/output/absent-pair"
}
```

Source, candidate, scratch and output are disjoint. The final output must be
absent. The stager exclusively reserves it and preserves partial failures and
unrelated collisions. The candidate is never modified. npm uses distinct empty
user/global configs, a private HOME/TMP/cache and offline, scripts-disabled pack.
No dependency/toolchain installation or network access is part of staging.

The Go source archive excludes npm. Every runtime, package template, license
and README is read from the fixed regular-file Git blob allowlist at the same
exact SHA. Executing generator/helper bytes must also match that SHA. Dirty
runtime worktree changes are ignored; dirty executing generators are rejected.
The complete source-relative closure is copied independently into both packs.
Small generated CommonJS scopes in bin/scripts/lib allow bounded root-metadata
errors before Node tries to parse a damaged root package.json itself.

Actual tar entries, regular types, modes and every extracted byte are checked.
`completion.json` appears outside the candidate only after both packs verify.
It records Git blob IDs and SHA256 independently of the Go archive, generated
file hashes, actual tarball SHA256/SRI and actual tool paths/hashes/versions.
No tarball contains its own hash. Partial evidence is not a completed pair.

# Root-owned native acceptance

Set `UAP_PRIVATE_NPM_NATIVE_CONFIG` to an absolute JSON file containing exactly
`stage` (the object above), `completionDigest` (independently pinned SHA256 of
completion.json) and `evidenceOutput` (an absent, disjoint directory). Run:

```sh
node --test npm/agentplugins/test/private-npm-native.test.js
```

The suite verifies identity before execution, installs both exact packs in
separate disposable prefixes, and runs the five template journeys, versions,
help, completion, errors and B's committed retired-command inventory. Shared
policy, engine and digest fields are compared exactly. Partial invocation logs
survive a failed assertion; native-completion.json is written only on success.
No generated dependency, MCP handshake or client/provider action runs. Root
also retains the existing injected installer-planner regression
`TestGeneratedPackagesReachExistingInstallerPlanner`; it is source fixture
proof, never packed native proof. Its separate integration boundary remains
explicit in the native evidence. Public workflows, registry provenance, native
Windows/macOS and all unexecuted supported lanes remain separate release gates.
