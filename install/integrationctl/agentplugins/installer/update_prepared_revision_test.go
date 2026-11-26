package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestUpdateStalePreparedLocalRevision(t *testing.T) {
	skipWindowsLauncherExecuteBit(t)
	ctx := testCtx(t)
	probe := buildProbe(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkg, config := filepath.Join(base, "package"), filepath.Join(base, "claude")
	writePackage(t, pkg, probe)
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	runner := &revisionListingRunner{inner: listingRunner{configRoot: config}}
	committed := 0
	eng, err := newTestEngine(t, Config{
		StateRoot: filepath.Join(base, "uap"), HelperExecutable: probe, Runner: runner,
		OnCommittedBinding: func(context.Context, BindingFacts) error { committed++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{
		Operation: OpInstall, PackageRoot: pkg, ClientID: "claude", ClientConfigRoot: config,
		ClientExecutable: probe, InstallationID: "00000000-0000-4000-8000-0000000000d1",
		OperationID: "revision-install", RequiredComponents: []string{"mcp", "skills"},
	}
	initial := prepareRevisionOperation(t, eng, req)
	if got, err := eng.Apply(ctx, initial, Decision{Confirmed: true}); err != nil || got.Outcome != OutcomeCompleted {
		t.Fatalf("install C0: %+v %v", got, err)
	}
	req.Operation = OpUpdate
	aBody := []byte("---\nname: sample-notify\ndescription: Revision A\n---\nPrepared A payload.\n")
	bBody := []byte("---\nname: sample-notify\ndescription: Revision B\n---\nCommitted B payload.\n")
	skill := filepath.Join("skills", "sample-notify", "SKILL.md")
	writeRevisionBytes(t, filepath.Join(pkg, skill), aBody)
	req.OperationID = "revision-a-stale"
	a := prepareRevisionOperation(t, eng, req)
	writeRevisionBytes(t, filepath.Join(pkg, skill), bBody)
	req.OperationID = "revision-b"
	b := prepareRevisionOperation(t, eng, req)
	if a.snapshot.Root == b.snapshot.Root || a.Plan().TreeDigest == b.Plan().TreeDigest || a.envelope.Manifest.Version != "1.0.0" || b.envelope.Manifest.Version != "1.0.0" {
		t.Fatal("A and B must be independent same-version snapshots with distinct bytes")
	}
	assertRevisionBytes(t, filepath.Join(a.snapshot.Root, skill), aBody)
	assertRevisionBytes(t, filepath.Join(b.snapshot.Root, skill), bBody)
	assertLegacyRevisionPreparation(t, a)
	assertLegacyRevisionPreparation(t, b)
	if got, err := eng.Apply(ctx, b, Decision{Confirmed: true}); err != nil || got.Outcome != OutcomeCompleted {
		t.Fatalf("apply B: %+v %v", got, err)
	}
	bState, err := eng.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	bInstall, ok := findInstall(bState, req.InstallationID)
	if !ok {
		t.Fatal("B installation missing")
	}
	bBinding, _, ok := findBinding(bInstall, "claude")
	if !ok || bBinding.PackageRevision == nil || bBinding.PackageRevision.TreeDigest != b.Plan().TreeDigest || !bBinding.SelectedDelivery.IsZero() || bBinding.ProfileAuthority != nil {
		t.Fatalf("B binding must retain exact revision on legacy delivery: %+v", bBinding)
	}
	active := b.Plan().TargetPath
	nativeSkill := filepath.Join(active, ".agentplugins-runtime", skill)
	assertRevisionBytes(t, nativeSkill, bBody)
	nativeManifest, err := os.ReadFile(filepath.Join(active, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	beforeMutations, beforeCommitted := runner.mutations, committed
	t.Logf("captured C0 native objects=%+v; committed B native objects=%+v", a.deliveries[0].request.PreviousNativeObjects, bBinding.NativeObjects)
	got, applyErr := eng.Apply(ctx, a, Decision{Confirmed: true})
	t.Logf("stale prepared A outcome=%s reason=%s err=%v", got.Outcome, got.Reason, applyErr)
	if !errors.Is(applyErr, ErrPlanChanged) || got.Outcome != OutcomeConflict || got.Reason != "plan_changed" {
		t.Errorf("stale A must refuse the changed committed B revision: outcome=%s reason=%s err=%v", got.Outcome, got.Reason, applyErr)
	}
	assertRevisionBytes(t, nativeSkill, bBody)
	assertRevisionBytes(t, filepath.Join(active, ".claude-plugin", "plugin.json"), nativeManifest)
	after, err := eng.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	afterInstall, _ := findInstall(after, req.InstallationID)
	afterBinding, _, found := findBinding(afterInstall, "claude")
	if !found || afterBinding.ClientBindingID != bBinding.ClientBindingID || afterBinding.TargetLocator != bBinding.TargetLocator || !reflect.DeepEqual(afterBinding.PackageRevision, bBinding.PackageRevision) {
		t.Errorf("stale A changed B binding authority: before=%+v after=%+v", bBinding, afterBinding)
	}
	if !reflect.DeepEqual(afterBinding.Receipts, bBinding.Receipts) || !reflect.DeepEqual(after.TransactionReceipts, bState.TransactionReceipts) || !reflect.DeepEqual(after.EnvironmentReceipts, bState.EnvironmentReceipts) || !reflect.DeepEqual(afterInstall.DataReceipts, bInstall.DataReceipts) {
		t.Error("stale A changed committed B receipts")
	}
	if afterBinding.Materialization != bBinding.Materialization || afterBinding.Activation != bBinding.Activation || afterBinding.Verification != bBinding.Verification || afterInstall.Source.TreeDigest != b.Plan().TreeDigest {
		t.Error("stale A changed committed B lifecycle or accepted digest")
	}
	if runner.mutations != beforeMutations || committed != beforeCommitted {
		t.Errorf("stale A reached effects: native mutations %d -> %d; committed callbacks %d -> %d", beforeMutations, runner.mutations, beforeCommitted, committed)
	}
	if t.Failed() {
		return
	}
	writeRevisionBytes(t, filepath.Join(pkg, skill), aBody)
	req.OperationID = "revision-a-fresh"
	fresh := prepareRevisionOperation(t, eng, req)
	if got, err := eng.Apply(ctx, fresh, Decision{Confirmed: true}); err != nil || got.Outcome != OutcomeCompleted {
		t.Fatalf("fresh A after B: %+v %v", got, err)
	}
	assertRevisionBytes(t, nativeSkill, aBody)
}

func prepareRevisionOperation(t *testing.T, eng *Engine, req Request) *PreparedOperation {
	t.Helper()
	prepared, err := eng.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = prepared.Close() })
	return prepared
}

func assertLegacyRevisionPreparation(t *testing.T, prepared *PreparedOperation) {
	t.Helper()
	installation, ok := findInstall(prepared.recorded, prepared.plan.InstallationID)
	binding, _, found := findBinding(installation, "claude")
	if !ok || !found || binding.PackageRevision == nil || !binding.SelectedDelivery.IsZero() || binding.ProfileAuthority != nil || len(prepared.deliveries) != 1 {
		t.Fatalf("expected captured legacy Claude revision: binding=%+v deliveries=%d", binding, len(prepared.deliveries))
	}
	delivery := prepared.deliveries[0]
	if !delivery.plan.SelectedDelivery.IsZero() || delivery.plan.ProfileAuthority() != nil || delivery.request.Client.ProfileAuthority != nil {
		t.Fatal("fixture did not exercise legacy selected delivery confirmation")
	}
}

func writeRevisionBytes(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func assertRevisionBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("revision bytes at %s: got=%q want=%q err=%v", path, got, want, err)
	}
}

type revisionListingRunner struct {
	inner     listingRunner
	mutations int
}

func (r *revisionListingRunner) Run(ctx context.Context, command ports.Command) (ports.CommandResult, error) {
	if len(command.Argv) >= 3 && (command.Argv[1] == "plugin" || command.Argv[1] == "mcp") && command.Argv[2] != "list" && command.Argv[2] != "get" {
		r.mutations++
	}
	return r.inner.Run(ctx, command)
}
