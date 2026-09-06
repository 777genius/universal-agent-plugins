# ROOT's packed-generated project → injected installer bridge

This is a separate, opt-in **test boundary**, following a successful terminal
`private-npm-native.test.js` run at the final integrated commit. It never builds
or executes candidate binaries, regenerates native projects, installs packages,
invokes an external scanner, reads real client profiles or starts providers.
The existing `add --dry-run` command constructs its real planner; only detection,
security assessment and lifecycle/state interfaces are injected. Fake Cursor,
Codex and Claude directories exist only in Go test temporary directories.

The accepted input is ROOT's actual `native-completion.json`, independently pinned
along with its native config. Failed64b2 has no completion and is not input.
Partial invocations, synthetic test records and source-generated harness results
are never packed-native acceptance. Integrate the native oracle fix and these
four new files before the final native run. This worker did not rebuild a candidate.

## Run after the final native gate

Use a clean checkout at ROOT's intended exact 40-character commit. Keep the
candidate, npm pack output, native evidence, native disposable fixture, bridge
config and bridge result outside the checkout. Retain all native temporary
projects, installed prefixes and caches unchanged until bridge verification ends.
Do not run cleanup concurrently. This is the existing quiescent, same-user local
namespace contract, not protection against a hostile concurrent same-UID writer.

1. ROOT runs its final exact-commit native gate with the trusted completed pair:

   ```sh
   UAP_PRIVATE_NPM_NATIVE_CONFIG="$NATIVE_CONFIG" \
     "$NODE" --test npm/agentplugins/test/private-npm-native.test.js
   ```

   Require exit 0 and the terminal completion from that same run. Record SHA256
   of the native config and terminal completion independently. The native config
   must be canonical `JSON.stringify(value, null, 2) + "\n"` JSON, with exactly
   `stage`, `completionDigest`, `evidenceOutput`; stage fields are exactly
   `candidate`, `repo`, `root`, `identity`, `manifestDigest`, `assetScope`,
   `authoringMode`, `go`, `workParent`, `output`, `node`, `npm`.
   Preserve the original config; do not repin a changed or partial run.

2. In a **dedicated external config directory**, write canonical `request.json`:

   ```json
   {
     "expectedCommit": "FINAL_INTEGRATED_40_HEX_COMMIT",
     "nativeConfig": "/absolute/native-config.json",
     "nativeConfigSha256": "INDEPENDENT_NATIVE_CONFIG_SHA256",
     "nativeCompletionSha256": "INDEPENDENT_TERMINAL_COMPLETION_SHA256",
     "fixtureRoot": "/absolute/work-parent/dual-authoring-NATIVE_SUFFIX",
     "disposableEvidence": true
   }
   ```

   `fixtureRoot` is the exact parent of the two `projects` paths in native
   completion, explicitly acknowledged as disposable evidence. It must be a
   direct `dual-authoring-*` child of the configured workParent. Project paths
   are derived exclusively from completion: `<fixtureRoot>/<product> disposable
   projects/<lane>`. No arbitrary project argument is accepted. Both products
   and all five lanes must exist. Every lane must have its successful packed
   `init` invocation; the completed native journey also added `extra-skill`.

3. Seal after completion, immediately before planning:

   ```sh
   "$NODE" npm/agentplugins/scripts/packed-installer-bridge.js \
     seal /absolute/bridge-config/request.json /absolute/bridge-config/sealed.json
   ```

   Save the printed SHA256 independently as `BRIDGE_SHA256`. Publication is
   exclusive: do not overwrite an old config. Sealing reads candidate bytes and
   pins, pack SHA256/size/SRI, completion/identity/mode/scope, invocation records,
   and complete directory/file inventories (including empty entries and modes).
   It does not verify Go build info or rerun native journeys: ROOT's preceding
   trusted native gate supplies that evidence. Config pins both verifier and its
   existing candidate helper. It snapshots the entire native fixture, candidate,
   pack output, native evidence and generated parents. Generated trees reject
   every symlink; the containing npm fixture permits only contained symlinks
   such as installed `.bin` links. All ordinary files must have one hard link.

4. Run only the packed bridge test, using installed Go **1.25.13**, offline cache,
   private HOME/TMP/config/GOCACHE and `GOMAXPROCS=2`. Keep result in a different
   external directory from config and all observed inputs. Example from checkout:

   ```sh
   env -i PATH=/var/data/uap-authoring-test-20260906/toolchain/go/bin:/usr/bin:/bin \
     HOME="$PRIVATE_HOME" USERPROFILE="$PRIVATE_HOME" APPDATA="$PRIVATE_HOME" \
     LOCALAPPDATA="$PRIVATE_HOME" XDG_CONFIG_HOME="$PRIVATE_HOME" \
     XDG_DATA_HOME="$PRIVATE_HOME" XDG_STATE_HOME="$PRIVATE_HOME" \
     XDG_CACHE_HOME="$PRIVATE_CACHE" TMPDIR="$PRIVATE_TMP" TMP="$PRIVATE_TMP" TEMP="$PRIVATE_TMP" \
     GOCACHE="$PRIVATE_CACHE/go-build" \
     GOMODCACHE=/tmp/uap-authoring-native-integration-old-20260906-artifacts/windows-modules \
     GOPROXY=off GOSUMDB=off GOENV=off GOTOOLCHAIN=local GOMAXPROCS=2 \
     UAP_PACKED_INSTALLER_NODE="$NODE" \
     UAP_PACKED_INSTALLER_CONFIG=/absolute/bridge-config/sealed.json \
     UAP_PACKED_INSTALLER_CONFIG_SHA256="$BRIDGE_SHA256" \
     UAP_PACKED_INSTALLER_COMMIT="$FINAL_COMMIT" \
     UAP_PACKED_INSTALLER_OUTPUT=/absolute/bridge-results/completion.json \
     go test -p=2 ./cli/plugin-kit-ai/internal/authoring/commands \
       -run '^TestPackedGeneratedPackagesReachExistingInstallerPlanner$' -count=1 -v
   ```

   Create the private directories and result parent first. `NODE` is an absolute
   trusted installed Node 22/24 path. Go checks HEAD and clean tracked/untracked
   state; don't use `-trimpath`, `-overlay`, alternate build flags or source copies.
   Do not set private directories inside the native fixture. No opt-in config
   skips the test; a partial opt-in fails. A skip is not acceptance.

Require exit 0, **ten passing project subtests**, and the exclusive result JSON
with **30 reports** (two products × five templates × three fake targets). Each
report checks successful structured dry-run, exact target, user scope, expected
status and all component names/kinds/support. The scanner compares the real
acquired snapshot to the exact source bytes/entries inside the private acquisition
root. All detector/scanner calls must occur. State writes, staging, discard,
activation and deactivation fail closed through the existing no-effect fixtures.
Source/client trees and acquisition cleanup are checked after each plan; the
entire sealed input is reverified after all subtests. Failed subtests do not
publish a result. Archive the successful Go output with the result and input pins.

Every bridge result keeps `release_eligible`, `platform_acceptance`, `attested`
**false**. This boundary proves injected planner integration only. It does not
prove real installation, profile discovery, external scanning, runtime behavior,
provider access, non-Linux execution, public release eligibility or attestation.

## Disposable harnesses (not native acceptance)

`node --test npm/agentplugins/scripts/packed-installer-bridge.test.js` tests seal
consistency, negative identity/pin/completion/mode/entry/link mutations and CLI
rejection. Its explicitly synthetic records use an in-process `frozenCandidate`
stub only for intake consistency; restoring the real helper and a separate CLI
process both reject them. No stub is reachable from the production verifier CLI.
Fixtures are retained under private TMPDIR for diagnosis.

`go test -p=2 ./cli/plugin-kit-ai/internal/authoring/commands -run
'^TestPackedInstallerSourceHarness$' -count=1 -v` generates five labelled source
fixtures using the existing public authoring test helper, including extra-skill,
and exercises the same planner seam for 15 plans. It writes no packed result.
Use the same offline/private environment above, without any packed opt-in vars.
