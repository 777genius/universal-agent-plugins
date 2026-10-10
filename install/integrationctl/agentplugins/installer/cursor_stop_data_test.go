package installer

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cursor"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/cursorhooks"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// The retained physical fixture is Linux-qualified. This decorator exercises
// the existing Darwin plain-file owner without running any client or helper.
type cursorStopDataAdapter struct{ physicalSelectedCursor }

func (a cursorStopDataAdapter) RefinePlan(ctx context.Context, in clients.PlanInput, p *domain.DeliveryPlan) error {
	if err := a.physicalSelectedCursor.RefinePlan(ctx, in, p); err != nil {
		return err
	}
	if runtime.GOOS != "darwin" {
		return nil
	}
	f, _ := p.SelectedDelivery.CursorFacts()
	original, err := nativeconfig.New().ReadExactFile(f.HooksPath)
	if err != nil {
		return err
	}
	planned, err := cursorhooks.Plan(cursorhooks.Request{Operation: cursorhooks.Install, Document: original.Body, Shell: cursorhooks.DarwinUserShell20260918, ExecutableVerified: true, Specs: []cursorhooks.HookSpec{{Executable: f.Executable, Selector: f.Selector}}})
	if err != nil {
		return err
	}
	f.TargetOS, f.TargetArch, f.CursorVersion = "darwin", "arm64", "2026.09.18-9a7762b"
	f.Shell = string(planned.Receipt.Shell)
	f.PlannedReceipt = cursorStopDataReceipt(planned.Receipt)
	f.EntryDigest = f.PlannedReceipt.EntryDigest
	p.SelectedDelivery, err = domain.NewCursorDelivery(f)
	return err
}

func cursorStopDataReceipt(r *cursorhooks.Receipt) domain.CursorHookReceipt {
	return domain.CursorHookReceipt{Version: r.Version, Event: r.Event, Executable: r.Spec.Executable, Selector: r.Spec.Selector, Shell: string(r.Shell), EntryDigest: r.EntryDigest, RemainderDigest: r.RemainderDigest}
}

func (a cursorStopDataAdapter) Activate(ctx context.Context, env clients.Env, r domain.ActivationRequest) (out domain.ActivationOutcome, err error) {
	if runtime.GOOS != "darwin" {
		return a.physicalSelectedCursor.Activate(ctx, env, r)
	}
	f, ok := r.Plan.SelectedDelivery.CursorFacts()
	if !ok || r.Client.ProfileAuthority == nil || r.VerifyOnly {
		return out, fmt.Errorf("TEST Stop install requires selection and original authority")
	}
	if err := profileauthority.Revalidate(ctx, *r.Client.ProfileAuthority); err != nil {
		return out, err
	}
	file, err := nativeconfig.New().BeginPlainExactFile(f.HooksPath)
	if err != nil {
		return out, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	original := file.Original()
	if original.Exists != f.OriginalExists || fmt.Sprintf("sha256:%x", sha256.Sum256(original.Body)) != f.OriginalRawDigest {
		return out, fmt.Errorf("TEST hooks basis changed")
	}
	planned, err := cursorhooks.Plan(cursorhooks.Request{Operation: cursorhooks.Install, Document: original.Body, Shell: cursorhooks.DarwinUserShell20260918, ExecutableVerified: true, Specs: []cursorhooks.HookSpec{{Executable: f.Executable, Selector: f.Selector}}})
	if err != nil {
		return out, err
	}
	if cursorStopDataReceipt(planned.Receipt) != f.PlannedReceipt {
		return out, fmt.Errorf("TEST frozen receipt differs")
	}
	if err := file.Apply(planned.Desired); err != nil {
		return out, errors.Join(err, file.Rollback())
	}
	current, err := nativeconfig.New().ReadExactFile(f.HooksPath)
	if err != nil {
		return out, err
	}
	if err := cursorhooks.VerifyOwned(current.Body, planned.Receipt); err != nil {
		return out, err
	}
	out = shared.StartedActivation(r)
	out.Activation, out.NativeEffect = domain.ActivationManual, domain.NativeEffectCommitted
	out.NativeObjects = []domain.NativeObjectOwnership{r.Plan.SelectedDelivery.CursorOwnership(f.PlannedReceipt)}
	return out, nil
}

// Stop-only used to commit no data receipt when stdio MCP was unsupported,
// causing the real post-commit host callback to receive an empty DataRoot.
func TestCursorStopOnlyCommittedOwnedData(t *testing.T) {
	if (runtime.GOOS != "linux" || runtime.GOARCH != "amd64") && (runtime.GOOS != "darwin" || runtime.GOARCH != "arm64") {
		t.Skip("NOT_RUN Cursor Stop data: fixed Linux amd64 or Darwin arm64 TEST tuple required")
	}
	e, req := physicalEngine(t)
	token, err := profileauthority.Capture(t.Context(), req.ClientConfigRoot)
	if errors.Is(err, directoryidentity.ErrUnsupported) {
		t.Skipf("NOT_RUN Cursor Stop data: full ancestry unsupported: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(req.ClientConfigRoot, "hooks.json")
	if err := os.WriteFile(hooks, []byte(`{"version":1,"hooks":{},"TEST-foreign":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	// Keep a real executable for the selected Stop while making authored MCP
	// unavailable. A supported skill keeps this a valid public installation.
	if err := os.WriteFile(filepath.Join(req.PackageRoot, "mcp.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"sample-notify":{"type":"stdio","command":"./bin/TEST-missing","args":[],"env":{}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	req.ClientID, req.ClientExecutable = "cursor", filepath.Join(req.PackageRoot, "bin", "probe")
	data := filepath.Join(e.cfg.PluginDataBase, domain.ComputePhysicalArtifactID("sample-notify", req.InstallationID))
	a := cursorStopDataAdapter{physicalSelectedCursor{physicalCompatibilityCursor: physicalCompatibilityCursor{Adapter: cursor.New(), observe: func(_ string, c domain.DetectedClient) error {
		if c.ProfileAuthority == nil || !c.ProfileAuthority.Equal(token) || c.ProfileNamespace != e.cfg.StateRoot {
			return fmt.Errorf("TEST original token/namespace lost")
		}
		return profileauthority.Revalidate(t.Context(), token)
	}}, selector: filepath.Join(data, "TEST-selector.json")}}
	e.cfg.Registry, err = clients.NewRegistry(a)
	if err != nil {
		t.Fatal(err)
	}
	canonical := physicalByteDigest(t, req.PackageRoot)
	projectCalls, committedCalls := 0, 0
	var committed BindingFacts
	e.cfg.ServerName = "sample-notify"
	e.cfg.ProjectArgs = func(BindingFacts) ([]string, error) {
		projectCalls++
		return nil, fmt.Errorf("TEST unselected MCP must not project args")
	}
	e.cfg.OnCommittedBinding = func(ctx context.Context, facts BindingFacts) error {
		committedCalls++
		if facts.DataRoot != data || facts.DataReceiptID == "" || facts.ProfileAuthority == nil || !facts.ProfileAuthority.Equal(token) {
			return fmt.Errorf("TEST committed callback lacks owned data/original token: %+v", facts)
		}
		state, err := e.store.Load()
		if err != nil {
			return err
		}
		installation, ok := findInstall(state, req.InstallationID)
		if !ok {
			return fmt.Errorf("TEST committed installation absent")
		}
		binding, _, ok := findBinding(installation, domain.ClientCursor)
		receipt := installation.DataReceipts[facts.DataReceiptID]
		if !ok || binding.DataReceiptID != facts.DataReceiptID || binding.ClientBindingID != facts.BindingID || binding.TargetLocator != facts.TargetPath || receipt.Locator != facts.DataRoot || receipt.DataReceiptID != facts.DataReceiptID || receipt.PhysicalBackend != binding.PhysicalArtifact || receipt.State != domain.DataReceiptOwned || receipt.OwnershipDigest == "" {
			return fmt.Errorf("TEST callback did not read the committed owned receipt")
		}
		if err := (providers.PluginDataManager{Base: e.cfg.PluginDataBase}).ValidateData(ctx, receipt); err != nil {
			return err
		}
		committed = facts
		return profileauthority.Revalidate(ctx, token)
	}
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := h.Close(); err != nil {
			t.Error(err)
		}
	}()
	unsupported := false
	for _, c := range h.Plan().Delivery.Components {
		if c.Kind == string(domain.ComponentMCPServer) && c.Support != string(domain.SupportUnsupported) {
			t.Fatal("TEST unavailable MCP was selected")
		}
		if c.Kind == string(domain.ComponentMCPServer) && c.Name == "sample-notify" {
			unsupported = c.Support == string(domain.SupportUnsupported) && c.Reason == "stdio_runtime_unavailable"
		}
	}
	if !unsupported {
		t.Fatalf("TEST missing runtime did not produce the expected unsupported MCP decision: %+v", h.Plan().Delivery.Components)
	}
	result, err := e.Apply(t.Context(), h, Decision{Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if projectCalls != 0 || committedCalls != 1 || committed.BindingID != result.Binding.BindingID {
		t.Fatalf("TEST callbacks: project=%d committed=%d binding=%s", projectCalls, committedCalls, committed.BindingID)
	}
	state, err := e.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation, ok := findInstall(state, req.InstallationID)
	if !ok || len(installation.DataReceipts) != 1 {
		t.Fatal("TEST owned receipt missing from real Store")
	}
	binding, _, ok := findBinding(installation, domain.ClientCursor)
	if !ok || binding.DataReceiptID != committed.DataReceiptID || binding.ProfileAuthority == nil || !binding.ProfileAuthority.Equal(token) || binding.ProfileNamespace != e.cfg.StateRoot {
		t.Fatal("TEST committed binding lost data or original authority")
	}
	f, ok := binding.SelectedDelivery.CursorFacts()
	if !ok || binding.SelectedDelivery.ValidateCursorObjects(binding.NativeObjects) != nil {
		t.Fatal("TEST actual Stop receipt missing")
	}
	acknowledged := false
	for _, object := range binding.NativeObjects {
		if object.Kind == "cursor_user_stop" && object.CursorReceipt == f.PlannedReceipt {
			acknowledged = true
		}
	}
	if !acknowledged {
		t.Fatal("TEST acknowledged receipt differs from frozen Stop plan")
	}
	if _, err := os.Lstat(filepath.Join(result.Binding.TargetPath, "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("TEST unsupported MCP was materialized: %v", err)
	}
	if physicalByteDigest(t, req.PackageRoot) != canonical {
		t.Fatal("TEST canonical source changed")
	}
	if err := e.VerifyProfileAuthority(t.Context(), req.InstallationID, binding.ClientBindingID); err != nil {
		t.Fatal(err)
	}
}
