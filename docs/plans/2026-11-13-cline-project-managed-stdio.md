# Cline Project managed stdio launcher correction

Root authorized this narrow production correction after genuine d7 core job131802753458 failed TestClineProjectSessionComposedLifecycle: authored bare `go` remained native command, while the legitimate0f9 session guard requires an absolute managed launcher. Baseline d7ae348f81f0c7976f39dfc4b70ce87441b89b9b. No vendor/native execution, remote writes or commits in this authoring worktree.

ProjectClineNative serves both User and Project: preserve User output and semantics. For Project stdio only, reuse existing delivered managedstdio helper; retain raw ORIGINAL authored bare/./bundled command and raw cwd anchor, already-once-expanded arguments/env and absolute resolved native cwd. Use existing managedstdio.Arguments private protocol; no shell, command lookup at installation, new state/schema, generic refactor or relaxed absolute-session guard. The helper alone resolves the bare runtime against the selected process PATH before chdir and validates bundled paths against package root.

Factor one cohesive Cline helper (~90–120 production lines). Recompute exact expected Project stdio from source envelope+active/data and compare a strict exact-read immutable projection before active reuse; old unwrapped, wrong data/command/arguments/cwd projections return ErrProjectionRefreshRequired without writes. Existing Stager.ProjectActiveNative independently verifies immutable package digest first; retain existing delivered helper/package receipt authority.

Self-critique1: wrapping the already-resolved absolute command would fail private launcher grammar. Preserve raw source command, literal unknown placeholders, empty/spaced args and package/data cwd anchor; never apply an extra expansion. User/native remote codecs remain untouched.
Self-critique2: fixing staging alone leaves old active packages reusable. Add exact active projection comparison with existing refresh refusal and test stale source, wrong data and immutable no-write behavior. Do not change test fixture to hide bare-command support. Existing native Linux direct caller must accept the new absolute outer managed helper, preserving actual installed-config behavior.

Expected scope ~120production+180tests+thisplan; clients/cline native_project.go/project.go, cohesive project_stdio.go/test, existing composed and native contracts only if assertions legitimately change. No shared process/provider implementation or workflow change. Unit cases cover barego, bundled command, default/package/data cwd, opaque arguments/env, User unchanged, stale/altered active projection and cancellation. Focused client/provider Cline/composed tests, portable crossbuild, full lint/core before freeze. Root+independent review before publication; hosted final-source native evidence remains required, not fabricated locally.

## Final local validation and independent review

Implementation stays in exactly five paths: this plan, clients/cline/native_project.go, project.go, project_stdio.go and project_stdio_test.go. New cohesive helper90physical lines/test178; two existing files+5/-10, no legacy size growth. User route remains the preexisting direct projection. Project active readback additionally rejects duplicate/unknown projection fields with the existing exact reader and typed decoder; refresh errors preserve their original cause with %w.

- Final focused Session/ProjectClineManagedStdio/ClineProjectSession/ClineProjects contracts: clients0.017s, providers0.014s, public CLI composed3.095s, all PASS. The originally failing TestClineProjectSessionComposedLifecycle passes without changing its bare-go fixture or strict session guard.
- Broader client/provider Cline suites PASS0.018/0.094s. The broader CLI Cline suite remains FAIL solely at TestProjectClineSavedLifecycleSeparatesUserAndProjects, existing local Git initialization refusal (ordinary_unsupported, exit unavailable). This is not waived or reported as overall PASS; coherent parent integration/core/hosted gates remain required.
- Actual Windows client test binary cross-compilation PASS on final source; this is compilation only, not Windows/native execution.
- Full make lint PASS all four whole-file and four changed-line passes,138 baseline exclusions unchanged; diff check clean. Initial errorlint wrappers corrected without exemption changes.
- Independent Antigravity source reviewer GO; its actual new managed-stdio tests PASS0.013s. Reviewed raw command/cwd, once expansion, User preservation, immutable expected projection and existing stager package-digest prerequisite.

Parent owns integration with current main, full coherent core checks and publication. No vendor/native command, release, remote write, commit or staging occurred in this worktree. Cross-platform native-acceptance plan remains a separate paused checkpoint; it is not bundled with this production fix.

Root final review correction: active stdio verification now lazily reads and strictly decodes one exact projection snapshot for all selected stdio servers; remote-only plans do not read it. Actual multi-server positive and late bad-member refusal/no-write contracts added, plus remote-only absence coverage. Existing object builder retains its separate single-snapshot read under the stager's prior immutable-digest check; this does not claim a new race-free cross-call operation or add a sixth file.
Post-review focused repeat on final Go bytes: clients0.014s/providers0.009s/CLI3.203s PASS; full-rule Cline package golangci-lint PASS0issues. Previous full eight-pass lint remains recorded separately; parent reruns coherent integration gates.

## Root integration verification

The five reviewed Cline files were integrated byte-for-byte with current main54d
through prepared merge34aa. Root full make lint PASS: all eight passes,138 baseline
rules unchanged. Full make test-core FAIL:90 top-level tests in nine packages,
the exact same names/packages as the retained pre-integration follow-up run;
this comparison is not a waiver or an exact isolated baseline qualification.
The actual public CLI TestClineProjectSessionComposedLifecycle separately PASS
in12.020s without changing its fixture. Hosted checks on the published final
commit remain required. Subsequent accompanying edits affect only the reviewed
Grok Python EOF contract and Hermes workflow/test diagnostics, not Go source.
