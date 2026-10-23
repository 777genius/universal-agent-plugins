# Direct source implementation review R1

**Current status: R1 GO after corrective verification at `236c3319`**, limited
to the original S1/S2/S4 findings. The original findings below are retained as
the review history; cumulative S5 integration has its own independent review.

Reviewer: `client_plans`, independent of source implementation. Date: 2026-10-23.
Reviewed committed S1/S2/S4 changes from `1a9b25de` through `ec7cc19b` in
`work/uap-sources`, using `git show` to exclude the concurrently edited CLI
integration. Current working files were also checked only to reproduce the
unchanged native transport policy and identify already pending parser fixes.
The separate archive implementation is excluded from this self-review; its R1
is `archive-code-review-r1.md` by `project_core`.

**Verdict: CHANGES REQUIRED.** Two native transport restrictions are bypassable
through ordinary user Git configuration. Three smaller offline grammar gaps
also require correction. Source lead received the concrete findings and has
started the native policy fixes. No source implementation files were changed
by this reviewer.

## R1-1 — P1: URL-scoped Git configuration bypasses TLS verification

`adapters/sourceacquisition/git_transport.go:nativeGitOptions` supplies generic
`-c http.sslVerify=true`. Git's URL-specific configuration can override that
setting. A disposable HOME containing:

```gitconfig
[http "https://127.0.0.1:<fixture-port>"]
    sslVerify = false
```

allowed `OSRunner.Run` with `NativeCredentials:true` and native `git ls-remote`
to send an HTTP request to an untrusted local TLS fixture. The fixture returned
404, so Git then failed; the request counter was **1**, proving that certificate
verification had already been bypassed. No fixture CA was provisioned. Errors
remained sanitized, which is good but does not enforce verified transport.

**Required fix:** use an authoritative per-command TLS verification override
that takes precedence over URL-scoped settings while preserving legitimate CA
configuration. Source lead proposes `GIT_SSL_NO_VERIFY=false`; verify it with a
real native Git regression against the same URL-specific false setting. Keep
signed anonymous acquisition isolated and do not inherit unsafe TLS bypasses.

## R1-2 — P1: a specific protocol allowance bypasses the transport restriction

The same helper supplies `protocol.allow=never` plus allowances for HTTPS and
SSH. A user's more specific `protocol.file.allow=always` remains effective. A
disposable user configuration containing a `url.file://...insteadOf` rewrite
from `https://fixture.invalid/` to a disposable local repository plus that file
allowance made the following native request succeed:

```text
OSRunner.Run(NativeCredentials:true,
    git -C <private-fixture-repo> fetch --quiet --depth=1
        https://fixture.invalid/repo.git HEAD)
```

Observed result: **success=true, err=nil**. The public input passed HTTPS
normalization, but the effective transport was forbidden local file access.
Only inert fixture Git content was used; no package commands ran.

**Required fix:** hard-limit actual direct transport protocols per command,
independently of user Git configuration (`GIT_ALLOW_PROTOCOL=https:ssh` is the
proposed route). Preserve the existing explicit `URLForRepo` fixture seam only
as a narrow internal injection, not as input-derived permission. Test the
`insteadOf` + specific protocol allowance with the actual Git binary. Also
verify that an HTTPS remote's redirect cannot downgrade to HTTP; libcurl
redirects need their own concrete proof rather than assuming the initial Git
protocol allowlist covers them.

## R1-3 — P2: explicit local archive names containing @ are misclassified

`sourcespec/parse.go:Parse` checks shorthand `@` before explicit local archive
classification. Reproduced:

```text
Parse("./package@1.0.zip")
  kind="git", err="GitHub source requires owner/repo"
```

This is an ordinary explicit relative archive filename. The in-progress CLI's
`localArchiveInput` also relies on Parse, so the shared grammar prevents the
promised local archive acquisition rather than leaving this as a harmless
parser-only limitation.

**Required fix:** honor explicit local path/archive precedence before shorthand
selection, preserving genuine `owner/repo@ref` and native SCP endpoints. Add
relative/absolute local archive names with `@` and shorthand/SCP regressions.
No filesystem probing is necessary inside the pure parser.

## R1-4 — P2: new URL Git intent and acquisition use different path grammars

`environment/model.go:Requirement.Validate` still applies legacy
`validateSubpath` to new generic URL Git coordinates. Actual probes showed:

```text
GitSource{URL:"https://example.test/repo.git", Path:"CON"}:
    Requirement.Validate() = nil
    sourcespec.NormalizeSubpath() = error
GitSource{URL:"https://example.test/repo.git", Path:"a\\x01b"}:
    Requirement.Validate() = nil
    sourcespec.NormalizeSubpath() = error
```

Acquisition correctly refuses these inputs later, but new manifests and locks
can be admitted with paths the acquisition contract cannot consume. This
violates the accepted single normalized grammar for the new source variants.

**Required fix:** validate new URL Git package paths with the shared strict
normalizer at intent/lock admission. Preserve already accepted historical
Repository/GitHub bytes and behavior rather than broadly tightening old locks.
Add matching manifest/model/acquisition validation regressions.

## R1-5 — P2: decoded SSH usernames can contain controls

`sourcespec/git.go:NormalizeGitURL` validates raw input and decoded path, but
its decoded username check omits control characters. Reproduced:

```text
Parse("ssh://user%0Aname@example.test/repo")
  kind="git", normalized URL retained, err=nil
```

The accepted endpoint policy rejects controls; percent encoding must not bypass
that check in the login component. This is a validation gap, without an observed
command injection claim.

**Required fix:** reject decoded username controls (and equivalent unsafe
userinfo text) before any native command or persistent identity. Keep valid SSH
user routing and the narrow Azure username-only HTTPS normalization unchanged.

## Component ledger

| Component | Assessment |
| --- | --- |
| S1 pure typed grammar and stable tuple keys | Structure GO; R1-3/R1-5 require fixes. Userless SCP dispatch absent in frozen ec7cc19b is already fixed in the observed pending parser; retain its regression in the next commit. |
| Provider repository/tree parsing | GO within reviewed provider grammar: transient BrowserTree, slash-ref resolution, explicit ambiguity refusal, known query consumption, archive-before-provider classification. |
| S2 separate native fetch and isolated checkout | Separation GO; real native fixtures exercise HTTPS credentials and SSH host verification without calling fixtures native-client proof. R1-1/R1-2 block transport qualification. |
| Exact revisions, tag peeling and browser ambiguity | GO: immutable commit validation, native ref enumeration, tag fetch/peeling, final coordinate persistence. |
| Signed anonymous GitHub boundary | Existing anonymous methods and isolated environment remain separate; do not broaden Directory/Discovery credentials. |
| S4 additive intent/lock variants | GO after R1-4: explicit three-way source counting, archive pins, moving update rule, requested selectors, graph edges and immutable revisions. |
| Old lock/key compatibility | GO by inspection: optional fields omit for old values, canonical GitHub remains Repository in environment intent, old source-key and manifest digest formulas remain intact, old strict readers reject new fields. |
| Dependency manifest/editing | GO after shared grammar fixes: source mixtures and inappropriate fields fail, no credential-bearing URL text is echoed in tested semantic failures, source location reporting preserved, archive values cloned. |
| Domain provenance fields | Additive fields GO; complete persistence/copy/cache reconstruction is S5 and requires the later integration review. |
| S5 CLI/store/lifecycle integration | Outside this frozen review; concurrently edited files were not judged as finished implementation. Actual end-to-end source journeys and both source-cache population orders remain required. |

## Verification and boundaries

The two P1 observations used the actual installed Git binary through the
production OSRunner, disposable HOME/config/repositories and a real untrusted
loopback TLS server. No network service outside loopback was contacted, no
external repository or package code was executed, and no user configuration was
modified. Probe programs were temporary review-only files outside the worktree.
Pure grammar/model probes used the shared current definitions; the reviewed
policies/functions were compared with ec7cc19b before attributing findings.

Focused source model/parser tests and the new native regressions must pass on
the corrective commit before R1 GO. Parent owns independent R2 routing,
publication and the later cumulative S5 review. Do not interpret this review as
an executable release or native agent-client runtime qualification.

## Corrective verification — 2026-10-23

Reviewed fixes `c1f8d232` and `16d58ac7` in frozen cumulative source tree
`236c331983d9356345a6654776e1b30a995aabc5`. All five findings are closed:

| Finding | Verified correction |
| --- | --- |
| R1-1 TLS verification | Native commands enumerate configured `http.*.sslVerify` key names in the same private repository context and override each at its own specificity, plus the generic key. Values are never read or logged. Actual regressions cover both direct and `insteadOf`-rewritten untrusted TLS endpoints with no HTTP request received. `GIT_SSL_NO_VERIFY=false` was correctly **not** adopted: Git treats a nonempty value as a bypass. |
| R1-2 actual protocol restriction | `GIT_ALLOW_PROTOCOL=https:ssh` independently fences user `protocol.file.allow` and URL rewrites. Local file access is confined to the explicit private `URLForRepo` fixture mapping. Actual native tests reject the configured file rewrite and an HTTPS-to-HTTP redirect. |
| R1-3 local archive precedence | Explicit relative/absolute/Windows archive paths are classified before shorthand `@`; the follow-up preserves `owner/repo@release.zip` as Git and native backslash archive paths as local. |
| R1-4 matching path grammar | New URL Git intent/locked variants validate through shared `NormalizeSubpath`; historical Repository coordinates retain their previous admission grammar. |
| R1-5 decoded SSH controls | Decoded user names reject Unicode controls before endpoint normalization. Valid SSH routing remains separate from credentials. |

Independent exact-toolchain verification on the frozen tree passed:
`sourcespec` full suite 0.003s; `environment` full suite 0.016s; the four actual
native Git TLS/rewrite/protocol/downgrade regressions 0.547s. Tests used
disposable native configuration/repositories and loopback TLS only. No package
code or external service was executed. The existing native credential/config
stability assumption during an individual command remains unchanged.
