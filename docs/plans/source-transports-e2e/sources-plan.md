# Sources PR research and implementation plan

Status: both independent plan review/fix rounds are verified GO; implementation authorized by the owner on 2026-10-23. Publication remains separately coordinated.

Repository: `777genius/universal-agent-plugins`; implementation base `3ea24e3014ddf6679ca86729c56f1069112b0d3a`; assigned worktree `the dedicated source implementation worktree`, branch `feat/source-transports-e2e`.

## 1. Accepted scope and constraints

Support complete standard Agent Plugins packages acquired through arbitrary public HTTPS Git, GitHub/GitLab/Azure browser repository/tree URLs, native SSH/private Git using the user's existing credentials, HTTPS package archives including extensionless downloads, and local ZIP/TAR/TAR.GZ/TGZ archives. This includes saved add, dependency intent and lock, transitive resolution, update, locked install/sync, export/import/fresh-store restore, dry run, repair and remove.

No raw SKILL.md importer, remote-directory traversal protocol, YAML fallback, new plugin manifest, package execution, scanner uploads or edits to frozen VS Code implementation/configuration/protocols/dedicated fixtures. Standard `plugin.json` and component interpretation stay unchanged. HTTPS downloads contain a complete package or a repository-like wrapper around one; HTML or a standalone manifest is not a package.

Directory and Discovery remain separately signed contracts. Their GitHub owner/repository + immutable SHA records, eligibility policy, anonymous acquisition, signature/digest checks and source keys do not broaden merely because direct user sources broaden. Direct user sources gain no Directory endorsement.

Read required: root `AGENTS.md`, `docs/AUTHORING_CAPABILITY_PRESERVATION.md`, owner decisions at top of `docs/STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md`, existing `docs/UAP_DEPENDENCIES.md`, `docs/THREAT_MODEL.md`. User-selected GPT-6.1 high/xhigh supersedes historical worker-model text.

## 2. Evidence from current source

* `cli/internal/agentpluginscli/source.go` accepts explicit local paths, an exact GitHub `owner/repo@FULL_SHA[//path]`, Discovery and Directory. Root package discovery is bounded to 16 candidates and validates candidates with the real standard loader.
* `cli/internal/agentpluginscli/app.go:SourceAcquirer` exposes only local and GitHub operations. Existing fake sources implement this narrow interface throughout tests.
* `install/integrationctl/agentplugins/adapters/sourceacquisition/acquirer.go` already separates a private `acquireGit(url, repository, revision, subpath)` implementation from its GitHub wrapper. It fetches an exact SHA, validates `FETCH_HEAD^{commit}`, tracks executable modes, seals snapshots, and never runs package commands. The implementation currently constructs GitHub-only canonical identity and failures.
* `sourceacquisition/refs.go` already resolves default branches, exact branches/tags, ambiguity and annotated tags. Adapt this neutral mechanism rather than inventing another resolver.
* Its `OSRunner` intentionally strips HOME, user Git config, SSH agent/askpass and all GIT_* variables. This is correct for the signed anonymous path and currently prevents private Git. Its contained process runner preserves cancellation/process-tree cleanup.
* Preserved `install/integrationctl/adapters/source/resolver_parse.go` supports `https://...git#ref`, `ssh://...git#ref` and `git@host:path.git#ref`; `resolver_clone.go` uses ambient clone/checkouts and error output. Reuse/adapt grammar ideas only. Do not route the standard engine through the old YAML resolver, copy its ambient clone flow or delete its code/tests.
* `environment/model.go` hardcodes `GitSource.Repository` and `LockedGitSource.Repository` to GitHub owner/repo, `github:<lowercase owner/repo>:<path>` source keys, two source variants and commit/ref/default selectors. `environment/graph.go`, `resolve_graph.go`, `resolve_expand.go` and coordinator logic assume every non-Git source is Registry.
* `adapters/dependencymanifest/decode.go` turns a `git` string into GitHub owner/repo by trimming a fixed prefix/suffix. `cli/internal/agentpluginscli/environment_session.go` writes GitHub URLs through `manifestedit`.
* `deps_resolver.go`, `deps_staging.go`, `deps_sync.go` and `saved_add.go` reconstruct GitHub identities and fetch from GitHub even during fresh-store restore. These are necessary E2E boundaries.
* `adapters/environmentstore/store.go` is content addressed, seals and rechecks bytes and executable inventory, publishes atomically and does not need another cache layout. Source metadata is provenance, not its content key.
* `domain.SourceIdentity` and `SourceBinding` lack a typed source URL/archive checksum. Multiple explicit copy sites in `usecase/{installation,binding,group_commit}.go` must retain new source facts.
* `directoryv1.ResolveDirectExact` is a strict GitHub/local validator inside the signed adapter; new direct source validation should not weaken that existing contract.
* `cli/internal/app/bundle_install_archive_header.go` already contains robust retained TAR path/type/collision/size policy; `bundle_install_apply_extract*.go` contains private extraction. It expects a legacy export marker and gzip TAR. It cannot be imported by the lower-level installer module without reversing dependency direction. Preserve it and adapt the proven policy into a narrow source-acquisition archive adapter; do not create a legacy command dependency or require `.plugin-kit-ai-export.json`.

## 3. Minimal design decisions for review

### 3.1 One pure normalized source grammar

Add neutral package `install/integrationctl/agentplugins/sourcespec` with no network, filesystem, domain or environment imports. Its immutable values normalize Git endpoints, package paths, selectors, archive locations and safe persistent display forms. It is shared by CLI, environment validation and acquisition; source parsing is not duplicated in three layers.

Proposed value types:

```go
type Kind string // git, archive, https-auto, local-archive
type Git struct { URL, Repository, Path, Ref, Commit string; Browser *BrowserTree }
type BrowserTree struct { Provider, Suffix string } // transient unresolved selector/path
type Archive struct { URL, Path, SHA256 string } // URL is HTTPS or explicit absolute local file
type Spec struct { Kind Kind; Git *Git; Archive *Archive; Requested string }
func Parse(string) (Spec, error)
func NormalizeGitURL(string) (url string, githubRepository string, err error)
func NormalizeArchiveURL(string) (string, error)
func NormalizeSubpath(string) (string, error)
```

`Repository` remains populated only for the existing canonical HTTPS GitHub shorthand identity. Generic URL identity lives in `URL`; do not place a GitLab/Azure/SSH endpoint into a field interpreted by Directory as GitHub owner/repo.

Accept these forms:

| Input | Normalization / intent |
|---|---|
| Existing `[github:]owner/repo@FULL_SHA[//path]` | Preserve current exact-SHA interpretation and canonical/source-key bytes |
| `github:owner/repo[@REF][//path]` | Explicit Git shorthand; missing ref means default branch; bare `owner/repo` remains Directory to avoid changing existing intent |
| `https://host/team/repo.git[#REF[//path]]` | Native Git HTTPS; optional exact commit/ref; no fragment means default branch |
| `git+https://host/path[#REF[//path]]` | Explicit arbitrary HTTPS Git, including clone endpoints without `.git` |
| `ssh://[user@]host[:port]/team/repo[.git][#REF[//path]]` | Native SSH Git; preserve case-sensitive repository path |
| `[user@]host:team/repo[.git][#REF[//path]]` | SCP-like SSH Git; require a real host and nonempty safe path; accept Azure `git@ssh.dev.azure.com:v3/org/project/repo`; canonicalize spelling deterministically |
| `https://github.com/owner/repo[/tree/REF[/path]]` | Strip browser syntax, normalize to repository Git endpoint plus selector/path |
| `https://gitlab.com/group[/subgroups]/repo[/-/tree/REF[/path]]` | Support nested groups; `/-/tree/` separates repository from selector/path; same syntax on self-managed hosts containing that explicit marker |
| `https://dev.azure.com/org/[project/]_git/repo[?version=GBbranch|GTtag|GCsha&path=/package]` | Normalize clone endpoint; map GB/GT to explicit refs and GC to commit; decode path once |
| `https://org.visualstudio.com/[project/]_git/repo` | Azure legacy clone/browser host grammar; recognized query fields as above |
| HTTPS `.zip`, `.tar`, `.tar.gz`, `.tgz` | Complete archive source |
| HTTPS without recognized extension/provider/Git fragment | Bounded HTTPS body acquisition and sniffing; positive ZIP/TAR/gzip-TAR means archive. If the response is not an archive (including 401/404), try native Git for an arbitrary clone endpoint, then report a sanitized combined source error. No credential-bearing HTML/body logging |
| Explicit local archive path | Complete archive source; imperative local source unless represented explicitly in a local project manifest in the later project-scope PR |

**Classification precedence is frozen:** (1) explicit local directory/archive paths and explicit git+https/ssh/SCP/Git shorthand or explicit Git selector; (2) recognized archive suffix or #sha256 archive selector, including GitHub/GitLab release URLs; (3) recognized provider repository/tree URL; (4) remaining HTTPS auto-classification. Conflicting explicit selectors/formats fail rather than overriding each other. An explicit archive suffix or positive ZIP/gzip/TAR magic recognition commits the operation to archive semantics: corrupt/truncated/oversize archive, malformed manifest and candidate-selection failures never fall back to Git. For extensionless HTTPS, fetch once without native credentials, retain the exact bytes/snapshot for package selection, and use native Git fallback only after a non-archive response/classification (including 401/404 or HTML). The probe never consults Git helpers or forwards credentials/headers. Multi-candidate archive diagnostics and any selected candidate use that one acquired byte stream.

Provider tree selectors containing `/` require unambiguous handling. The pure parser retains provider and entire undecided suffix in transient `BrowserTree`; it does not guess Ref or Path. `ResolvedGit` belongs to sourceacquisition (alongside RefResolution), contains `{Git sourcespec.Git; Resolution RefResolution}`, and does not make the pure package depend on an adapter. Add network helper `ResolveGitSource(ctx, sourcespec.Git) (ResolvedGit, error)` in sourceacquisition: enumerate exact remote branch/tag names once, match possible URL suffix boundaries, and return a fully resolved Git with Browser=nil, exact Ref/Commit/Path, plus RefResolution. Accept only one matching selector/path decomposition; fail with `use the clone URL#refs/heads/branch//package` when more than one interpretation is valid. Full SHAs are unambiguous. Only resolved coordinates enter manifest/lock/source identity. File/blob browser URLs are rejected with a tree/package-root hint rather than importing one file.

Use fragment `#sha256=<64 lowercase hex>[&path=<normalized package path>]` for explicit archive checksum/path. Git `#REF//path` retains familiar legacy behavior; exact `refs/heads/...` / `refs/tags/...` avoid branch/tag ambiguity. Do not accept credentials/passwords/tokens in URLs. HTTPS userinfo is rejected (Azure username-only clone prefixes can be safely stripped before persisting/using the standard endpoint). SSH username is transport routing, preserved; passwords are rejected. Reject control characters, backslashes, unsafe encodings, empty/duplicate provider query fields and arbitrary query/fragment aliases. Recognized Azure `path`, `version`, `_a=contents` are consumed then dropped from endpoint identity. Do not lowercase generic repository paths or conflate SSH and HTTPS access principals; GitHub HTTPS aliases only map to existing GitHub identity.

### 3.2 Additive identity, lock and intent

Extend Git source structs with optional `URL` and make `Repository` optional, requiring exactly one except that canonical GitHub HTTPS normalization intentionally chooses the old Repository representation. Existing valid structures, source keys, canonical identity and manifest digests remain byte compatible.

```go
GitSource{ Repository string, URL string, Path string, Selector Selector }
LockedGitSource{ Repository string, URL string, Path string, Requested Selector,
                 Commit string, ResolvedRef string, DefaultBranch string }
ArchiveSource{ URL string, Path string, SHA256 string } // SHA256 optional explicit byte pin
LockedArchiveSource{ URL string, Path string, RequestedSHA256 string, SHA256 string }
Requirement{ Alias string; Git *GitSource; Registry *RegistrySource; Archive *ArchiveSource }
LockedSource{ Git *LockedGitSource; Registry *LockedRegistrySource; Archive *LockedArchiveSource }
```

`ArchiveSource` is HTTPS only in portable saved dependencies. Local archive inputs follow existing local-directory imperative behavior, recorded with a sanitized absolute path and content checksum, and are not silently exported as portable URLs. All remote ready archives save by default and participate in closure resolution. Add `archive = "https://..."`, optional `sha256 = "<64 hex>"`, and `path = "..."` to `uap.toml`; reject mixed `archive/git/registry`, Git selectors on archives and checksum on Git. Git keeps current `git/ref/commit/path` syntax.

Freeze new stable source keys as `git-url-v1:<64 lowercase sha256 hex>` and `archive-url-v1:<64 lowercase sha256 hex>`, where the hash preimage is canonical JSON encoding of the three-string array `[kind, canonical endpoint URL, logical package path]` with no optional elements. Use this one pure helper in sourcespec for Requirement.SourceKey, lock validation and new SourceBindingID inputs. Snapshot-cache keys use the same tuple source key plus immutable commit/archive checksum, digest algorithm and tree digest. Existing GitHub and Directory source keys are untouched. Checksum/commit/selector are version facts, excluded from stable source ownership. Preserve generic path case, explicit SSH user, URL port and decoded portable path meaning; tests cover URL delimiters and case-distinct paths. Add a typed archive selector `SelectorArchive` whose value is the explicit SHA256 or empty for a moving URL; its validator accepts only empty or one lowercase hex SHA256. Root and transitive graph selectors, edge validation, source reconstruction and update policy must handle all three variants explicitly instead of relying on else = Registry.

Keep manifest schema and lock version 1 as additive manager fields: old binaries already reject unknown manifest/lock fields fail closed. Confirm by tests that no old field changes meaning and old locks retain existing normalized output. If review determines a version bump is required, use a new lock version only for new source variants and continue accepting old locks without rewriting identity.

Add optional source provenance fields `SourceKind`, `SourceURL`, `ArchiveSHA256` to both `domain.SourceIdentity` and `SourceBinding`; copy them through every state/store/usecase projection. Source kind is `git`/`archive`; zero values keep historical source data. For generic Git, `ResolvedRevision` remains the exact commit; for archives use a clearly tagged `sha256:<archive bytes hash>` immutable revision. New canonical direct identities are stable endpoint+package identities, with exact version separate. Preserve historical direct GitHub canonical identity bytes.

Compute source binding identity from stable endpoint/package for new types. Old GitHub dependency special-case behavior stays unchanged. Updating a moving URL/ref must not change ownership; switching repository/URL/package still requires explicit switch/rebind. Repair always uses the installed binding's exact commit or archive checksum, never today's moving branch/download.

### 3.3 Acquisition interface and compatibility

Retain the existing `SourceAcquirer` interface and GitHub methods for signed callers and existing test fakes. Introduce a separate optional `DirectSourceAcquirer` capability implemented by `sourceacquisition.Acquirer` and the lazy composition wrapper:

```go
ResolveGitSource(context.Context, sourcespec.Git) (ResolvedGit, error)
ResolveGit(context.Context, sourcespec.Git, selector string, exactRef bool) (RefResolution, error)
DiscoverGitPackages(context.Context, sourcespec.Git, commit string) ([]string, error)
AcquireGit(context.Context, sourcespec.Git, commit, expectedTreeDigest string) (domain.PackageSnapshot, error)
AcquireArchive(context.Context, sourcespec.Archive, expectedArchiveSHA256, expectedTreeDigest string) (domain.PackageSnapshot, error)
```

AcquireGit requires an immutable commit. Selector resolution occurs once per new/update intent before discovery/loading. Signed Directory/Discovery continue calling existing `AcquireGitHubVerified`, without user credential mode or generic source parsing. Use one CLI helper to dispatch GitHub old/new methods and one helper to construct identity from a locked package; every restore path uses them rather than manually rebuilding `githubIdentity`.

Archive acquisition returns byte digest and selected **logical** package path in snapshot source metadata. Physical extraction wrapper names never enter source identity or portable pins. The archive adapter does not parse standard plugin schema or evaluate package code. CLI validation uses the standard loader and the existing security/consent boundary. A missing optional direct capability produces a precise capability error in old fakes, not panic or accidental Directory fallback.

### 3.4 Native Git and credential boundary

Keep current anonymous isolated OSRunner behavior for signed GitHub. Add a per-command transport mode used only by direct user-selected Git `ls-remote`/`fetch` operations. These operations run in a fresh private repository, outside the caller's checkout, using the user's HOME/USERPROFILE/XDG Git credential configuration, SSH config/known-hosts and SSH_AUTH_SOCK, plus trusted authentication environment variables (`GIT_ASKPASS`, `SSH_ASKPASS`, `GIT_SSH`, `GIT_SSH_COMMAND`, `GIT_SSH_VARIANT`, CA/proxy inputs as appropriate). Do not persist credential material or copy it into package snapshots. Preserve process-tree cleanup.

Strip ambient Git repository/object/index/worktree overrides, tracing destinations, arbitrary `GIT_CONFIG_COUNT/KEY/VALUE` injection and unsafe TLS bypass. Override hooks location to an empty private directory, disable fsmonitor/maintenance/submodule recursion and disallow non-HTTPS/SSH protocols. `GIT_TERMINAL_PROMPT=0`, SSH batch mode and `GCM_INTERACTIVE=Never` ensure automated commands do not hang; existing helpers/agent may supply already provisioned credentials. Do not disable host-key checking or TLS verification. Authentication errors explain how to configure native Git credentials without echoing native stderr, URLs with secrets, helper output or command environment.

**Crucial checkout separation:** direct transport fetches full blobs for the shallow immutable commit (omit blob:none). Checkout/materialization then uses the current anonymous isolated environment and no user/system filter/hook configuration. This prevents a fetched `.gitattributes` from triggering a user-installed smudge filter or LFS downloader and avoids a credential-dependent lazy fetch during isolated checkout. Configure safe isolated repo settings before fetch/checkout; package checkout stays detached, sparse paths remain validated and gitlinks/symlinks retain existing source snapshot policy. Do not run a generic ambient `git clone` followed by checkout.

Resolve annotated tags to commits and verify fetched commit. Default/ref update uses exact remote selection with no broad refspec or revision expression. Root discovery shares native Git transport and remains bounded. Any credential-related state stays in the owned temporary repository and is removed on success, error, declined confirmation and cancellation.

### 3.5 Archive adapter policy

Archive implementation can be a separate worker after both plan reviews; its owned files are new `sourceacquisition/archive*.go` only (including tests). Source lead owns sourcespec and domain shapes and supplies the agreed API. No edits to acquirer.go, environment or CLI by the archive worker.

* HTTPS only for remote archives; standard certificate verification, bounded redirects (max 5), HTTPS at every hop, no URL userinfo, timeout/context cancellation, no response-body/error dumping. No supplied authorization header/cookie forwarding. Extensionless downloads use magic/header inspection; content type is advisory, not authority. Query-bearing signed-token URLs are rejected to keep credentials out of persistent intent/locks; stable public endpoints are the supported package URL contract.
* Max compressed/download bytes 256 MiB, max archive entries 10,000, max regular file 64 MiB, max total expanded bytes 256 MiB, max depth 64. Limits apply to declared and actual data before writing/allocating; ZIP central-directory bounds do not replace streamed counters. Bound gzip expansion and consume/validate the complete archive including checksum/truncation errors and trailing/concatenated content policy.
* Support ZIP (store/deflate only, reject encryption/unsupported methods), TAR, gzip TAR. Detect incompatible format/suffix and corrupted streams; never treat HTML, a bare SKILL or plugin.json as an archive. Reject multiple ZIP volumes and malformed/truncated headers.
* **Stream termination is frozen:** accept one archive only. ZIP must begin at byte zero with a local-file header (or an empty EOCD, which subsequently fails package selection), have valid single-disk central-directory/end records and terminate exactly at the EOCD plus its declared bounded ZIP comment. No preamble/self-extracting prefix, appended archive or undeclared trailer is accepted. ZIP64 is unsupported in this first delivery because all accepted entry/count/byte sizes fit ordinary ZIP; reject its records explicitly with a format-limit diagnostic. TAR requires its two zero end blocks, followed only by whole 512-byte zero padding blocks within byte limits; reject a second TAR or nonzero/misaligned tail and missing/truncated end blocks. Gzip TAR is exactly one gzip member containing the accepted TAR stream, with CRC/size/trailer validation; disable concatenated-member mode and reject any extra compressed member or outer trailing bytes. Conventional in-member TAR zero padding remains accepted and bounded. Test these precise boundaries, valid padding/comments, CRC/truncation, concatenated gzip/TAR and prefix/trailer payload.
* Adapt retained TAR policy: normalized NFC portable relative paths; reject traversal/absolute/drive/UNC/backslash/control paths, links, devices/special entries, duplicate names, Unicode/case collisions including implicit ancestors, file/directory conflicts, excessive declared sizes and archive bombs. Ignore benign `./` root directory records only if defined explicitly in tests; do not normalize traversal into acceptance. TAR PAX metadata is validated through resulting headers; hard links and symlinks are rejected.
* Extract only to newly created private `0700` temp root via rooted/no-follow exclusive file operations. Strip special bits; retain only ordinary executable intent as metadata for `SnapshotWithExecutables`, consistent across OSes. No overwrite, chmod of external paths or running extraction utilities.
* Logical archive root is the extraction root when it has root plugin.json. Otherwise, if every archive member lies under exactly one outer directory (no sibling files), strip exactly that one physical wrapper before package-path selection, even when its package is nested. This wrapper is transient and never persisted. Explicit Path is relative to this logical root; empty Path prefers its root plugin.json, then bounded package-shaped manifest candidates. One candidate may be selected; multiple candidates require explicit Path and are listed without a second download. The standard loader validates only the selected package. A malformed logical root manifest fails without nested fallback. Updating a wrapper from repo-old/ to repo-new/ preserves logical path and stable identity. The adapter API returns the logical path it selected, never an extracted absolute path or wrapper name.
* Compute SHA256 of original archive bytes; check requested/locked byte checksum before extracting. Re-seal selected package with packagedigest and check locked tree digest. Lock both byte checksum and tree/manifest digests. If the endpoint changes bytes with identical extracted tree, locked restore still fails byte identity unless the verified store already has the original tree. Update may adopt changed bytes only by the normal explicit update/consent flow.
* Return cleanup-owned sealed snapshot; remove download and extraction roots on all paths. Never publish store/state/manifest before existing consent and commit boundaries.

## 4. Implementation components and exact ownership

| Component | Owned production files | Review focus / outputs |
|---|---|---|
| S1 pure grammar | new `install/integrationctl/agentplugins/sourcespec/{source,git,provider,archive}.go` and focused tests | provider grammar, path/ref ambiguity, redaction, canonicalization and old input preservation |
| S2 native Git | `.../adapters/sourceacquisition/{acquirer,refs}.go`, new `git_transport.go`, `git_direct.go`, relevant focused tests | separate anonymous/authenticated transport; isolated checkout; exact revisions, limits, cleanup |
| S3 archive | new `.../adapters/sourceacquisition/archive*.go` only | safe downloader/parser/extractor, original byte digest, root selection, real ZIP/TAR/gzip fixtures |
| S4 environment contract | `.../environment/{model,graph,resolve_graph,resolve_expand,coordinator}.go` as necessary; new `sources.go`; `.../adapters/dependencymanifest/decode.go`; `cli/internal/agentpluginscli/manifestedit/edit.go`; associated tests | three typed variants, stable identities, transitive graph/edge validation, backward-compatible lock/TOML encoding |
| S5 lifecycle integration | `cli/internal/agentpluginscli/{app,source,saved_add,deps_resolver,deps_staging,deps_sync,environment_session,lifecycle,add_multi}.go`; new direct-source helpers; `cli/cmd/agentplugins/main.go` lazy wrappers/composition where needed; `domain/{types,state,identity}.go`; `usecase/{installation,binding,group_commit}.go` explicit source copies | saved/default/--no-save paths; immutable repair; update/switch identity; cache dispatch; no Directory relaxation; provenance output |
| S6 E2E/docs | new focused source journeys in `cli/internal/agentpluginscli/*source*e2e_test.go`; native transport fixture helpers; `docs/UAP_DEPENDENCIES.md`; small source guide `docs/PLUGIN_SOURCES.md`; relevant README/website command guidance only after implementation verified | actual native transports plus CLI lifecycle; truthful support table and credential setup; no release qualification claims |

On every content-store cache hit, including deps_candidates.go, loadStoredDependency, materializeLockedPackage, DescribePinned and replay/repair paths, reconstruct SourceIdentity from the validated selected locked package before loading or computing binding identity. Store.Metadata.Source is historical first-writer provenance only: two endpoints or two byte-distinct archives may share one tree entry. Do not copy per-source trees, rewrite metadata or adopt cached ownership. Test both population orders for same-tree distinct generic Git/archive origins and distinct archive byte pins.

Do not modify `directoryv1` signatures/parser/policy for new direct types. Replace the new direct call sites' dependency on `ResolveDirectExact` with pure source validation; historical anonymous GitHub paths retain their current checks. Store layout and standard loader need no new feature-specific branches. Project scope worker owns scope resolution/client capability work. Source helpers consume the already selected App and environment context, including project scope in Directory policy requests; they never reconstruct a user App during replay. Exchange App/model/coordinator touchpoints before edits; source changes are integrated above project core before the project source-seam scenario. Do not independently edit the project worktree.

## 5. Meaningful verification and E2E matrix

Unit/contract tests cover grammar, malformed/ref ambiguity, URL credentials, exact source key stability, backward old fixtures, source clone/copy persistence, lock edges/variant mixtures, archive hostile inputs and cancellation. Fake-runner tests cover command construction and environment policy but are not called transport E2E.

Native transport fixtures must execute the installed Git binary with no URLForRepo local-path substitution:

1. Real public HTTPS Git over loopback TLS: Go HTTPS CGI handler invokes `git http-backend` against a disposable bare repository; per-test CA trust only. Use extensionless clone endpoint to exercise auto classification and `git+https` explicit path.
2. Private HTTPS Git: same native smart-HTTP server requires Basic auth; disposable HOME contains a test credential helper restricted to exactly the fixture host/path; failures without credentials, success with credential helper, no secrets in JSON/state/lock/store diagnostics. Assert no authenticated fallback/download to unrelated origin.
3. Real SSH Git: `/usr/sbin/sshd` on a loopback high port, isolated temporary host/user key and known_hosts, one forced-command `git-upload-pack` boundary limited to the disposable bare repo, no password/public network. Use native `ssh` and optionally a disposable ssh-agent socket. Test ssh:// port and SCP-like grammar separately where SCP spelling cannot encode a port (SSH config alias). Verify untrusted host refusal and cancellation tears down owned child processes.
4. HTTPS archives: serve real ZIP, TAR, TAR.GZ/TGZ and extensionless variants from loopback TLS with redirects. Lock -> remove source/store copy -> fresh environment restore from exact bytes; moved URL/repacked byte change must fail locked replay. Local archive add verifies source selection and cleanup without declaring portability.
5. Each supported source kind reaches the real standard loader, store, CLI plan/apply and existing disposable client adapter fixture. A standard skill-only package is enough; no package runtime or MCP server execution. Add payload files whose commands would create a sentinel if run and prove no sentinel exists after every phase. Security evaluation fixture stays local and inert; no external content scanner upload.
6. Lifecycle: add saves intent/lock, repeat no-op, moving ref/URL update keeps source ownership, installed repair uses old immutable pin after remote moves, source switch blocked, export/import into empty store, transitive generic Git/archive dependencies with shared target union, remove retains unrelated package/client files. One representative client for full transport cross product; reuse existing cross-client tests for shared lifecycle boundaries instead of multiplying transport/client matrices unnecessarily.
7. Directory and Discovery regression cases retain the anonymous runner, signed exact GitHub source and digests. New direct HTTPS/SSH/archive inputs never fall through to a Directory selector. Existing exact GitHub saved lock/source key tests retain their original values.
8. Dry run/decline/cancellation: no durable environment/store/projection changes; downloads, private Git temp dirs and request-local snapshots cleaned; at most one exact acquisition per missing pin per command. Offline status/tree use lock/store without contacting sources. Failed auth or mutated archive leaves manifest/lock/state unchanged.

Fixture availability observed locally: git, ssh, sshd, openssl present. Use the root-installed Go 1.26.10 at `the Go 1.26.10 toolchain`. Standard CI should install openssh-server for a required Linux source-transport lane; local absence yields an explicit skip only outside that required lane. Add a small opt-in network smoke against existing public provider repositories if necessary, but do not let it substitute for deterministic native Git fixtures or upload package content.

## 6. Checkpoints and review sequence

1. Source plan R1 by project_core completed at `sources-review-r1.md`; this revision fixes mandatory identity-from-lock on all cache paths, wrapper-independent archive logical paths, transient browser suffix/resolved-path API, one versioned tuple-hash source-key encoding and selected-App/project integration. Reviewer verification pending; no code.
2. Root sends revised plan to `client_plans` for source plan R2. Fix findings and freeze API/file ownership. No code until root explicitly acknowledges both rounds complete.
3. S1/S4 typed model and grammar checkpoint, focused contracts; local commit. S3 archive adapter may proceed in disjoint files after contracts are shared.
4. S2 native Git checkpoint, real transport tests; local commit.
5. S5 saved/lifecycle/restore integration checkpoint, focused journeys; local commit.
6. Merge archive worker commits, full source journey fixtures/docs; local commit.
7. Two implementation review/fix rounds according to root routing; required focused/full module gates and E2E on exact candidate. Send SHAs, changed files, test commands/results, limits and unresolved items to root. Root performs all pushes/PR publication.

## 7. External primary references checked

* Git official environment/configuration documentation: https://git-scm.com/docs/git and https://git-scm.com/docs/git-config . HOME/global/system config and GIT_CONFIG_COUNT need explicit treatment; GIT_SSH_COMMAND is shell-interpreted and is trusted user configuration, never synthesized from source text.
* Git credential helpers: https://git-scm.com/docs/gitcredentials.html . Existing configured helper programs are native authentication infrastructure; helper content/output is not package content and must not be serialized/logged.
* Azure clone URL documentation: https://learn.microsoft.com/en-us/azure/devops/repos/git/clone?view=azure-devops . Clone URLs need not end `.git` and commonly include a username-only organization prefix; public/private native Git authentication belongs to Git rather than package metadata.
* GitLab repository documentation / live first-party tree grammar: https://docs.gitlab.com/user/project/repository/ and https://gitlab.com/gitlab-org/gitlab/-/tree/master/doc . `/-/tree/` is a clear repository/browser separator; nested subgroup paths must remain intact.

Azure browser query mappings and provider branch/path ambiguity tests must additionally be verified against provider-owned examples/source while implementing S1; no unsupported query interpretation should be guessed merely to make a fixture pass.

## R1 fix ledger

* Shared tree cache provenance: obligatory identity-from-selected-lock on every store load, including candidate loading, plus both cache population orders and byte-distinct/same-tree archives.
* Archive wrapper identity: logical root strips at most one common physical wrapper; Path is logical and stable across changing version wrappers. Selection does not require an archive-to-standard-loader dependency or a second download.
* Provider parse/resolution: BrowserTree carries undecided suffix without I/O; ResolveGitSource returns resolved final Path and immutable commit/ref.
* Source key/seam: fixed versioned SHA256 of canonical three-string JSON tuple; old keys exact; source lane consumes selected App and integrates above scope core.
* Additional self-review correction: SCP-like SSH accepts Azure SSH endpoints without .git suffix; shell/control/path ambiguity stays rejected.

## R2 fix ledger

* Archive stream termination frozen to one non-ZIP64 ZIP with no prefix/undeclared trailer, one TAR with two end blocks + whole zero padding, or one gzip member containing that TAR; CRC/size/end/trailing validation is mandatory and bounded.
* Classification order frozen: explicit transport/selectors, archive shapes, provider Git URLs, then HTTPS auto. Recognized archives never fall back to Git; one credential-free HTTPS byte acquisition feeds selection. Native fallback is limited to non-archive responses.
* ResolvedGit explicitly belongs to sourceacquisition, keeping sourcespec pure and free of RefResolution/adapter imports.
* Focused regressions added: provider-hosted release archives; valid/corrupt extensionless archives; private extensionless clone; HTML then Git failure; multi-candidate single-request archive; no helper/auth forwarding in probe.
