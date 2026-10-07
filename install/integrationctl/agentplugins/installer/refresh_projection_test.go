package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefreshProjectionThroughEngineUsesCurrentHostArgs(t *testing.T) {
	skipWindowsLauncherExecuteBit(t)
	ctx := testCtx(t)
	probe := buildProbe(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkg, config := filepath.Join(base, "package"), filepath.Join(base, "config")
	writePackage(t, pkg, probe)
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	hostArgs := []string{"portable-launch", "--global-config", "/old/global.json", "--primary", "/old/primary"}
	eng, err := newTestEngine(t, Config{
		StateRoot: filepath.Join(base, "uap"), HelperExecutable: probe, ServerName: "sample-notify",
		ProjectArgs: func(BindingFacts) ([]string, error) { return append([]string(nil), hostArgs...), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	apply := func(req Request) (Result, error) {
		prepared, err := eng.Prepare(ctx, req)
		if err != nil {
			return Result{}, err
		}
		defer func() { _ = prepared.Close() }()
		return eng.Apply(ctx, prepared, Decision{Confirmed: true})
	}
	req := Request{
		Operation: OpInstall, PackageRoot: pkg, ClientID: "codex", ClientConfigRoot: config,
		ClientExecutable: probe, InstallationID: "00000000-0000-4000-8000-000000000155",
		OperationID: "projection-install", RequiredComponents: []string{"mcp", "skills"},
	}
	installed, err := apply(req)
	if err != nil || installed.Outcome != OutcomeCompleted {
		t.Fatalf("install = %+v, %v", installed, err)
	}
	projectedPath := filepath.Join(installed.Binding.TargetPath, "mcp.json")
	projected := func() string {
		body, readErr := os.ReadFile(projectedPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		return string(body)
	}
	if got := projected(); !strings.Contains(got, "/old/global.json") || !strings.Contains(got, "/old/primary") {
		t.Fatalf("initial projection = %s", got)
	}
	before, err := eng.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	oldBinding, _, ok := findBinding(before.Installations[0], "codex")
	if !ok {
		t.Fatal("installed binding missing")
	}
	hostArgs = []string{"portable-launch", "--global-config", "/new/global.json", "--primary", "/new/primary"}
	req.Operation, req.OperationID = OpRepair, "projection-exact-repair"
	repaired, err := apply(req)
	if err != nil || repaired.Outcome != OutcomeUnchanged || !strings.Contains(projected(), "/old/global.json") {
		t.Fatalf("ordinary repair = %+v, %v", repaired, err)
	}
	req.Operation, req.OperationID = OpRefreshProjection, "projection-refresh"
	prepared, err := eng.Prepare(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Plan().NoChange {
		t.Fatal("refresh preview incorrectly skipped staging")
	}
	refresh, err := eng.Apply(ctx, prepared, Decision{Confirmed: true})
	_ = prepared.Close()
	if err != nil || refresh.Outcome != OutcomeCompleted || !refresh.Mutated {
		t.Fatalf("refresh = %+v, %v", refresh, err)
	}
	if got := projected(); !strings.Contains(got, "/new/global.json") || !strings.Contains(got, "/new/primary") || strings.Contains(got, "/old/global.json") {
		t.Fatalf("refreshed projection = %s", got)
	}
	after, err := eng.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	newBinding, _, ok := findBinding(after.Installations[0], "codex")
	if !ok || len(newBinding.Receipts) != len(oldBinding.Receipts)+1 || newBinding.PackageRevision.TreeDigest != oldBinding.PackageRevision.TreeDigest {
		t.Fatalf("refreshed binding = %+v", newBinding)
	}
	req.OperationID = "projection-repeat"
	repeat, err := apply(req)
	if err != nil || repeat.Outcome != OutcomeUnchanged || !repeat.NoChange || repeat.Mutated {
		t.Fatalf("repeat refresh = %+v, %v", repeat, err)
	}
	afterRepeat, err := eng.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	repeatedBinding, _, _ := findBinding(afterRepeat.Installations[0], "codex")
	if len(repeatedBinding.Receipts) != len(newBinding.Receipts) {
		t.Fatalf("repeat wrote %d receipts", len(repeatedBinding.Receipts))
	}
	if err := os.WriteFile(filepath.Join(pkg, "different.txt"), []byte("new source bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	req.OperationID = "projection-wrong-revision"
	if _, err := eng.Prepare(ctx, req); !errors.Is(err, ErrUpdateRequired) {
		t.Fatalf("changed source refresh error = %v", err)
	}
}

func TestRefreshProjectionRejectsGroupAtFacade(t *testing.T) {
	eng, err := newTestEngine(t, Config{StateRoot: filepath.Join(t.TempDir(), "uap")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = eng.Prepare(context.Background(), Request{Operation: OpRefreshProjection, Targets: []ClientTarget{{ClientID: "claude"}, {ClientID: "codex"}}})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("group refresh error = %v", err)
	}
}

func TestRefreshProjectionPublishesCommittedBindingAndRetriesFailedCallback(t *testing.T) {
	skipWindowsLauncherExecuteBit(t)
	ctx := testCtx(t)
	probe := buildProbe(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkg, config := filepath.Join(base, "package"), filepath.Join(base, "config")
	writePackage(t, pkg, probe)
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	hostArgs := []string{"portable-launch", "--global-config", "/old/global.json"}
	eng, err := newTestEngine(t, Config{
		StateRoot: filepath.Join(base, "uap"), HelperExecutable: probe, ServerName: "sample-notify",
		ProjectArgs: func(BindingFacts) ([]string, error) { return append([]string(nil), hostArgs...), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{
		Operation: OpInstall, PackageRoot: pkg, ClientID: "codex", ClientConfigRoot: config,
		ClientExecutable: probe, InstallationID: "00000000-0000-4000-8000-000000000156",
		OperationID: "callback-install", RequiredComponents: []string{"mcp", "skills"},
	}
	apply := func() (Result, error) {
		prepared, prepareErr := eng.Prepare(ctx, req)
		if prepareErr != nil {
			return Result{}, prepareErr
		}
		defer func() { _ = prepared.Close() }()
		return eng.Apply(ctx, prepared, Decision{Confirmed: true})
	}
	installed, err := apply()
	if err != nil || installed.Outcome != OutcomeCompleted {
		t.Fatalf("install = %+v, %v", installed, err)
	}
	runner := &recordingCodexRunner{}
	eng.cfg.Runner = runner
	hostArgs = []string{"portable-launch", "--global-config", "/new/global.json"}
	callbackCalls := 0
	eng.cfg.OnCommittedBinding = func(_ context.Context, facts BindingFacts) error {
		callbackCalls++
		if facts.BindingID != installed.Binding.BindingID || facts.DataRoot != installed.Binding.DataRoot {
			t.Fatalf("callback binding facts = %+v", facts)
		}
		body, readErr := os.ReadFile(filepath.Join(facts.TargetPath, "mcp.json"))
		if readErr != nil || !strings.Contains(string(body), "/new/global.json") {
			t.Fatalf("callback observed stale projection: %s, %v", body, readErr)
		}
		state, loadErr := eng.store.Load()
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		binding, _, found := findBinding(state.Installations[0], "codex")
		if !found || len(binding.Receipts) != 2 || len(runner.calls) != 0 {
			t.Fatalf("callback order: binding=%+v runner calls=%v", binding, runner.calls)
		}
		if callbackCalls == 1 {
			if binding.Activation != "prepared" {
				t.Fatalf("first callback activation = %s", binding.Activation)
			}
			return errors.New("host publication failed")
		}
		if binding.Activation != "failed" {
			t.Fatalf("retry callback activation = %s", binding.Activation)
		}
		return nil
	}
	req.Operation, req.OperationID = OpRefreshProjection, "callback-refresh"
	first, err := apply()
	if err == nil || !strings.Contains(err.Error(), "host publication failed") || first.Outcome != OutcomeIncomplete || !first.Mutated {
		t.Fatalf("failed callback refresh = %+v, %v", first, err)
	}
	if callbackCalls != 1 || len(runner.calls) != 0 {
		t.Fatalf("callback/activation calls after failure = %d/%d", callbackCalls, len(runner.calls))
	}
	req.OperationID = "callback-retry"
	retry, err := apply()
	if err != nil || retry.Outcome != OutcomeCompleted || !retry.Mutated {
		t.Fatalf("callback retry = %+v, %v", retry, err)
	}
	if callbackCalls != 2 || len(runner.calls) == 0 {
		t.Fatalf("callback/activation calls after retry = %d/%d", callbackCalls, len(runner.calls))
	}
	state, err := eng.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding, _, _ := findBinding(state.Installations[0], "codex")
	if len(binding.Receipts) != 2 || binding.Activation != "active" {
		t.Fatalf("retry state = %+v", binding)
	}
	req.OperationID = "callback-repeat"
	repeat, err := apply()
	if err != nil || repeat.Outcome != OutcomeUnchanged || !repeat.NoChange || callbackCalls != 2 {
		t.Fatalf("stable repeat = %+v, %v; callbacks=%d", repeat, err, callbackCalls)
	}
}

func TestRefreshProjectionPublishesClaudeLocatorBeforeVerification(t *testing.T) {
	skipWindowsLauncherExecuteBit(t)
	ctx := testCtx(t)
	probe := buildProbe(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkg, config := filepath.Join(base, "package"), filepath.Join(base, "config")
	writePackage(t, pkg, probe)
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	runner := &capturingRunner{inner: listingRunner{configRoot: config}}
	hostArgs := []string{"portable-launch", "--primary", "/old/primary"}
	eng, err := newTestEngine(t, Config{
		StateRoot: filepath.Join(base, "uap"), HelperExecutable: probe, Runner: runner,
		ServerName:  "sample-notify",
		ProjectArgs: func(BindingFacts) ([]string, error) { return append([]string(nil), hostArgs...), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{
		Operation: OpInstall, PackageRoot: pkg, ClientID: "claude", ClientConfigRoot: config,
		ClientExecutable: probe, InstallationID: "00000000-0000-4000-8000-000000000157",
		OperationID: "claude-projection-install", RequiredComponents: []string{"mcp", "skills"},
	}
	apply := func() (Result, error) {
		prepared, prepareErr := eng.Prepare(ctx, req)
		if prepareErr != nil {
			return Result{}, prepareErr
		}
		defer func() { _ = prepared.Close() }()
		return eng.Apply(ctx, prepared, Decision{Confirmed: true})
	}
	installed, err := apply()
	if err != nil || installed.Outcome != OutcomeCompleted {
		t.Fatalf("Claude install = %+v, %v", installed, err)
	}
	runner.calls = nil
	hostArgs = []string{"portable-launch", "--primary", "/new/primary"}
	callbacks := 0
	eng.cfg.OnCommittedBinding = func(_ context.Context, facts BindingFacts) error {
		callbacks++
		body, readErr := os.ReadFile(filepath.Join(facts.TargetPath, ".mcp.json"))
		if readErr != nil || !strings.Contains(string(body), "/new/primary") {
			t.Fatalf("Claude callback observed stale projection: %s, %v", body, readErr)
		}
		state, loadErr := eng.store.Load()
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		binding, _, found := findBinding(state.Installations[0], "claude")
		if !found || binding.Activation != "prepared" || len(runner.calls) != 0 {
			t.Fatalf("Claude callback order: binding=%+v calls=%v", binding, runner.calls)
		}
		return nil
	}
	req.Operation, req.OperationID = OpRefreshProjection, "claude-projection-refresh"
	refreshed, err := apply()
	if err != nil || refreshed.Outcome != OutcomeCompleted || callbacks != 1 || len(runner.calls) == 0 {
		t.Fatalf("Claude refresh = %+v, %v; callbacks=%d calls=%v", refreshed, err, callbacks, runner.calls)
	}
	req.OperationID = "claude-projection-repeat"
	repeat, err := apply()
	if err != nil || repeat.Outcome != OutcomeUnchanged || callbacks != 1 {
		t.Fatalf("Claude repeat = %+v, %v; callbacks=%d", repeat, err, callbacks)
	}
}
