# Kimi: preserve project MCP selection during intact repair

Status: PRE-CODE DRAFT. This author has made no repository changes and run no Go,
lint, installer executable or vendor process. Obtain two independent plan reviews
and fix their findings before implementation. Root coordinates an isolated worktree
and the Go/lint slot; original checkouts and publication manifests remain untouched.
This draft is not a code GO, a native qualification or release authorization.

## 1. Exact evidence and current source

Read AGENTS.md, docs/PROJECT_INSTALL_OWNER_SCOPE.md,
docs/AUTHORING_CAPABILITY_PRESERVATION.md and the project completion plan.
Preserve useful user/legacy behavior. Active PR521–529 remain the scope; VS Code
is deferred and its dedicated source/tests must not change. No release.

Starting Gemini-independent Kimi leaf: b2bb2b7f85b5461420f932478b197905e6fc5ad9,
tree b12354b2565a05d14037556f8493b149d7d36ea2. The original Kimi continuation
checkout was clean at that head during the source review. A narrow search found
no newer Kimi repair-selection plan, worktree or implementation. Recheck exact
head/clean status and coordinate with the current author before implementation;
never overwrite concurrent work.

Genuine Kimi component run40569825577/job122438517506 failed after21.40s.
Native12767504412:14298 bytes, SHA256
bb61975315455a478c3fc54083f23426c2c75b90be7f641132f98dbeac0e3af4,
40 closed ordinary members. Frontend12767812666:3519 bytes, SHA256
06f000c1fc78c4546632cbb72d3b9d7704c06e4275538df2216e9d233082b075,
25 members. Independent audit:
/tmp/uap-kimi-b2bb-independent-20261027-uyq51c5q/audit.json, SHA256
13138425fd8b44e166b3297882b04f023605b69dd00437b2cf6dde97800ea932.
Fresh API metadata, nested closed ZIP/CRC/ordinary membership, exact source/tree,
source/build cleanliness,24 frontend stream joins,4 nested service-stream joins,
37 HTTP joins and source-bound Python Kimi intake all pass.

The repeated-add direct-result correction now advances: repeat-add-a and unchanged
update both complete without state/native-byte changes. Intact repair exits0 and
reports mutated=true, then the existing fixture correctly refuses with
“Kimi intact repair changed native bytes”. Its combined assertion checks the
selected skill OR project MCP file; after-repair bytes are not exported, so do
not claim to have directly read either changed file.

Actual public result facts provide a separate concrete lead: add-a and unchanged
update classify the held MCP server as native. Intact repair keeps the same
installation/source tree/manifest/physical identity but changes that MCP component
to unsupported, reason not_in_recorded_delivery, and emits
managed_component_removal_planned. The old03d repeated-add parser defect is not
this failure. All five qualification, account/model flags remain false, and the
stable observation, later update/missing repair/removal/restart were not reached.
The separate Kimi single-tool lane supplies no qualification to this component lane.

## 2. Source-proven scope/layout mismatch

At exact b2bb:

- clients/kimi/adapter.go ManagedMCPSelection always returns the user layout:
  .kimi-plugin/plugin.json, nested mcpServers, with that member optional.
- clients/kimi/project_mcp.go projectKimiComponents writes the project delivery's
  selected servers into the managed artifact's root mcp.json. It does not author
  the user plugin manifest. Native .kimi-code/mcp.json is a separate projection;
  read selection only from the verified managed artifact, never native mutable
  configuration, a source package, or an unverified receipt claim.
- providers.Stager.ManagedMCPNames verifies the entire recorded artifact against
  its held digest, dispatches the adapter's unscoped SelectionReader, reads that
  one layout, then verifies the artifact again. Missing selection file yields
  the existing nonnil empty selection. Therefore a valid project artifact is
  read through the wrong user manifest layout and produces an empty selection.
- usecase.preflightTargetComponents uses that verified empty selection during
  repair. restrictRepairMCPSelection then marks the project MCP unsupported with
  the observed not_in_recorded_delivery reason; readiness/removal handling emits
  the observed planned-removal diagnostic.

This trace explains the observed removal plan. It is not a substitute for the
unexported post-repair byte observation. Preserve the current fixture refusal.
Do not add a formatting exception, semantic-only byte comparison or native-file
baseline adoption.

## 3. Narrow additive scope-aware selection contract

### Adapter policy

Add one optional pure adapter interface next to SelectionReader, provisionally
ScopedSelectionReader with ManagedMCPSelectionForScope(domain.InstallScope)
(clients.SelectionLayout,error). It chooses a managed-artifact layout only;
it observes no filesystem, runs no process and adds no client capability.

Kimi implements it with an explicit closed scope switch:

- user: return the existing .kimi-plugin/plugin.json layout exactly, including
  ServersMemberOptional=true;
- project: return root mcp.json, nested mcpServers, without optional member;
- unknown/empty scope: return an error, without guessing.

Retain the existing ManagedMCPSelection method and its user API behavior exactly.
No choice based on file presence, directory spelling, detected ConfigRoot,
manifest contents, project-like paths or fallback to another candidate file.
A project skills-only artifact may still have no mcp.json and yield the existing
verified empty selection; a present dedicated document without mcpServers keeps
its existing refusal. User manifest omission semantics remain unchanged.

### Provider and usecase propagation

Add an explicit scoped Stager method, provisionally ManagedMCPNamesForScope(ctx,
clientID,scope,root,digest), while retaining ManagedMCPNames's existing signature
and behavior for existing direct/user callers. Both share the existing strict
artifact read implementation; preserve both complete digest verifications, error
behavior, fixed selection document parser and deterministic sorted names.
The new method validates scope, asks an adapter ScopedSelectionReader when present,
and otherwise uses the adapter's existing SelectionReader or generic default.
This fallback is for unchanged adapter policy; no generic client-ID table or
Kimi condition belongs in providers/usecase.

In usecase, add a separate optional scoped managed-selection reader port.
recordedManagedMCPNames receives the already planned scope explicitly. Before
reading, require a known scope equal to the selected prior binding's scope;
retain exact target resolution/path verification and the prior managed digest.
Prefer the scoped port when present. The production providers.Stager implements
it, and profileStager forwards it after its existing original-profile check.

Keep the legacy unscoped reader fallback for unchanged host stagers that implement
only the old optional port; do not remove their existing API or silently treat
that old port as absent. A wrapper must forward the scoped inner port when it
exists; it may not accidentally erase it and fall back. If a scoped reader returns
an error, propagate it: never retry unscoped or interpret that refusal as empty.
The production CLI/Engine/profile-wrapped Kimi paths must demonstrably reach the
scoped method. This correction makes no new scope-correctness claim for an
external host's custom legacy-only stager implementation.

No staging, activation, repair selection restriction, native configuration
mutation, confirmation, version admission, data authority, scope authority or
state format is relaxed. Preserve the damaged/missing artifact route: names
from damaged bytes remain untrusted; the existing exact rebuilt-artifact digest
and ownership gates remain mandatory. This is not permission to resurrect
unrecorded MCP selection.

### Compatibility alternatives and decision

Changing the old exported ManagedMCPNames signature would unnecessarily break
existing direct consumers. Replacing Kimi's unscoped user layout globally would
break existing user registry semantics. Selecting whichever layout file exists
would infer scope from bytes and could confuse foreign/contradictory candidates.
A project-only registry adapter instance would leave Engine/SDK hosts that inject
the ordinary context-aware Kimi adapter inconsistent. The additive scoped port
carries the existing explicit authority through both composition paths while
retaining old contracts; its small forwarding surface is covered directly.

## 4. Meaningful inert regression before native rerun

Use existing fixture/composition helpers where their routes match. No native
vendor, scanner/installer binary, managed launcher or hosted-guard surrogate runs
locally. In-process Go filesystem/App/Engine execution is inert product testing,
not a native qualification. Do not replace the public repair path with a fake
result or hand-written native repair.

1. Reproduce intact project repair through actual composed CLI App entrypoints:
   independently author a local standard package containing skill plus MCP,
   original user/A/B foreign settings and retained data, with inert detected
   pinned Kimi and explicit failing-on-use native runner/launcher seams. Run
   public project add, then public repair of the installed ID WITHOUT corrupting
   managed/native/source bytes first. Before repair hold exact selected skill
   and project MCP bytes and original user/B trees. Require the unchanged source,
   installation, physical binding/data identities, project Prepared/Installed
   state, selected MCP native support and ownership retained, no
   not_in_recorded_delivery/removal diagnostic, and exact skill/MCP bytes after.
   State/receipt evolution may reflect the real mutated repair; do not assert a
   fictitious no-op classification. This regression must fail against b2bb's
   original implementation and pass only with the scoped correction.
2. Exercise the same intact repair using the actual public Engine/SDK composition
   with a directly injected ordinary Kimi adapter. Preserve the existing corrupt
   or missing old-pin repair test separately: that damaged-byte fallback did not
   cover this intact verified-artifact case. Prove original user/B/data isolation
   and run no native process.
3. Through the actual Stager, stage/verify project and user artifacts and read
   their recorded selections using explicit scopes. Project selection must use
   root mcp.json; user and the old unscoped API must use the original manifest.
   A bounded independently authored digest-verified artifact with conflicting
   candidate files proves the selected layout is authoritative and no fallback
   occurs. Skills-only user/project cases retain their intended empty selection.
   A malformed selected dedicated document refuses even if another candidate is
   well formed. Unsupported scope errors must occur before interpretation.
4. Verify both artifact-digest checks and no damaged-byte selection. Altered
   managed bytes or mismatched held digest refuse selection; wrong scope/prior
   binding/target locator refuses at the usecase authority boundary. Foreign
   native namespace collisions or changed managed native objects retain existing
   refusal and no target mutation. The ordinary safe repair of damaged owned
   bytes still requires its existing exact reconstruction/ownership conditions.
5. Add focused forwarding contracts for profileStager and scoped-versus-legacy
   stagers: exact held scope reaches the new port, original profile failure
   prevents the read, scoped errors never fall back, legacy user behavior is
   retained, and adapters without scoped policy retain their previous layout.
   Do not add generic client-ID cases or grow architecture allowlists/baselines.

The existing native byte-stability assertion at intact repair remains unchanged.
Run the regression RED on the original source via inert overlay or isolated
checkout, then GREEN on the final exact code. Do not edit original native evidence
or execute the full opt-in native fixture locally.

## 5. Files, review and gates

Expected product files: clients/identity.go; Kimi adapter-owned selection method;
providers/managed_selection.go; usecase/managed_selection.go and native_binding.go.
Use small new focused provider/usecase/Kimi contracts and a real composed CLI
regression plus a public Engine regression, extending existing fixtures rather
than introducing a second installer. Preserve all existing tests and useful
legacy source. No dedicated VS Code file, global client registry policy, shared
native parser, native collector, qualification flag, Python evidence schema,
provider credential, runtime pin, timeout, artifact member or production state
format needs to change.

After two independent exact plan GOs, root supplies an isolated branch at the
fresh agreed source. Commit the accepted plan before implementation. Obtain two
independent exact-code reviews, fix findings and retain incremental commits.
Root coordinates focused existing/new Kimi public lifecycle/provider/usecase/
selection tests, relevant default-adapter compatibility contracts, all-enabled
affected lint, the unfiltered whole-file architecture/size lint gate, gofmt and
whitespace. Attribute actual test commands/results to their executor. Do not run
concurrent Go/lint or repeat optional gates without a remaining concrete risk.

Root alone performs fresh expected-head publication to the existing Kimi PR,
without force or overwriting concurrent source. If another author has moved the
leaf, preserve/reconcile/review their exact combined bytes first. A new genuine
hosted component run and independent raw audit must establish native consequences.
The separate single-tool approval/history boundaries remain independent. No old
failed capture becomes successful, no qualification is inherited, and no merge,
release or VS Code authorization follows from this plan.
