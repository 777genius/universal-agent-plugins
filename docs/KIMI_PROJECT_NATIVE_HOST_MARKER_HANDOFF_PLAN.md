# Kimi single-tool observer: forward only genuine held host markers

## Actual next failure and source diagnosis

Current published PR524 head55f5d848fb61899f2c84bd7cccc7592549718b5f,
tree2f20a8e83fe52710eda7ead5494e7821e873ddce, is identical to local
c989277bca17d6205635187fae7a52b349ec3879. Runtime40456546065 call job
122031436268 fails after public installation/receipt proof. Closed native
12712668290 and frontend12712892996 have exact committed source/tree,
bounded canonical unique ordinary ZIP/CRC closure and all five qualification
flags false. Native outer SHA256
38ceaded9a2b4b9bb28dcfbf6b77bb637012921eef8ea2b7761f6d9e572ec406;
frontend outer SHA256
3fdc6fc1e63494f32ae666aa7fb933fd094c816f58facee84bbe0e33bba9bd67.

The actual failure diagnostic (stderr SHA256
c6a8c370bf0a80ae17a38bcb2e45774944bd4cf9547d8efabb843b688643fbcf)
reports single-tool/host-admission/enter/other, observer_entered=false,
service_recorded=false, cleanup_reported=false and all cleanup values null.
The Python command exits1, stdout0. No nested native API/service was reached.

Python's unchanged require_hosted in scripts/run-native-client-matrix.py
requires exactly GITHUB_ACTIONS=true, RUNNER_ENVIRONMENT=github-hosted and
AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED=1, then independently checks actual
platform.system()/platform.machine() against linux-amd64. The Go fixture's
constructor independently checks those same observed markers, actual GOOS/
GOARCH and ordinary UID/GID before construction. Its closed f.env() intentionally
inherits no ambient host/profile/credential environment, and the single-tool
Python invocation currently uses only f.env(), omitting all three required
host markers. This is a source-proven admission handoff mismatch, not evidence
that the hosted runtime was unavailable or that a native API grammar failed.

The component observer already forwards the exact three real os.Getenv values
explicitly at its existing Python invocation. Preserve that implementation
unchanged as the established finite reference; its pending native installed
result remains a separate observed gate. Do not speculate about its API failure.

## Finite handoff after two independent plan GOs

Scope is repotests/agentplugins_kimi_project_native_attempt_test.go,
repotests/agentplugins_kimi_project_native_attempt_contract_test.go and this
plan. Preserve every previously published external Go/Python change.

Add a small private single-tool observer environment helper which accepts the
already closed base environment plus exactly three held observed strings:
actions, runner-environment and explicit disposable opt-in. Require their exact
eligible values as validation only; output each key using the held argument,
never a synthesized eligibility literal. Reject the entire request on a missing,
changed, whitespace/control-bearing or otherwise invalid marker. Reject any
duplicate occurrence of these three keys already present in the closed base.
Return a copied environment without mutating the original slice.

At only the existing private Python single-tool invocation, read exactly the
three named os.Getenv values into this helper, validate those held values and
pass its returned environment to the unchanged command. This revalidates the
same values used in the handoff, so an environment change since construction
cannot silently manufacture eligibility. Keep original HOME/XDG/PATH, profile,
all other closed environment entries and original native working directory.
Do not use os.Environ, bulk maps, credentials, tokens, proxy/provider/loader
settings, guessed runner labels or any broader inherited variables.

Do not forward a platform override: require_hosted determines the real platform
itself, and the existing outer actual GOOS/GOARCH/UID/GID guards remain unchanged.
Do not alter or bypass either independent hosted guard, set host markers for
local native execution, weaken require_hosted or add fallback credentials.
The Python Context's existing closed vendor-child environment remains unchanged;
this finite metadata handoff applies only to the real Python observer process.

## Meaningful inert controls and remaining hosted proof

Add a positive pure helper contract using inert argument values, without setting
the local process's hosted markers or invoking either native guard/client. Assert
the exact closed output contains the original base entries and exactly three
held values, with source slice/capacity unchanged and no aliasing. Negative
controls cover missing/wrong/self-hosted/whitespace/control values for each
marker and duplicate/conflicting base keys; every refusal returns no partial
environment. Plant unrelated ambient credential/proxy/private canaries and
verify none enter the output. Other version/public/native commands must retain
their existing base environment and command tables.

Run the affected existing Kimi single-tool and new environment contracts, Go1.22
compatible formatting/whitespace checks and both actual affected all-enabled
and whole-file size/architecture lint only after the shared exclusive Go/lint
slot is granted. No vendor, model, installer, credential session or native guard
is executed/spoofed locally. The existing host-guard negative tests remain.

Independent code review precedes root-owned publication to existing PR524.
No source pin, runtime request, trust/manual approval, permission, native API/
provider grammar, identity/receipt proof, budget, supervision/cleanup, generic
failure diagnostic or qualification flag changes. VS Code remains excluded;
no new PR or release. Fresh genuine source-bound hosted output must show the
observer passes its unchanged host admission and reveal the next actual native
boundary. The old failed capture is not relabelled and missing cleanup is not
turned into success.

## Completed local validation

Both independent plan and code reviews approved the exact two-file handoff.
On the reviewed bytes, the focused existing single-tool contracts, new pure
environment controls and original closed-environment contract passed (0.012s).
Actual affected all-enabled lint and unfiltered whole-file size/complexity/
architecture lint each reported zero issues. Formatting and diff whitespace
checks passed. Checks used the granted exclusive slot, pinned Go1.26.10 with
Go1.22 source compatibility, private shared-memory temporary/lint directories,
and the existing bounded module/build caches. No native client, local hosted
guard, installer or model was executed. The genuine hosted follow-up remains
required; the distinct installed component context-stage failure is unchanged.
