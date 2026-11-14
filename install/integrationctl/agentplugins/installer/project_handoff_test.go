package installer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type handoffCaptureAdapter struct {
	id       domain.ClientID
	requests []domain.ActivationRequest
}

func (a *handoffCaptureAdapter) ID() domain.ClientID { return a.id }
func (a *handoffCaptureAdapter) Activate(_ context.Context, _ clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.requests = append(a.requests, req)
	return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled}, nil
}
func (*handoffCaptureAdapter) Deactivate(context.Context, clients.Env, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	panic("handoff test must not deactivate")
}

// Exercise the actual SDK seam and provider dispatch, retaining identity/path
// checks. The inert leaf observes the request; real file behavior is covered by
// TestProjectPreparedGroupPublicLifecycleAndHandoff and the single replay.
func TestProjectHandoffPreservesOnlyQualifiedReadOnlyRequest(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		change                              func(*domain.ActivationRequest)
		wantVerify, wantError, profileFails bool
	}{
		{name: "project native files", wantVerify: true},
		{name: "acting request", change: func(r *domain.ActivationRequest) { r.VerifyOnly = false }},
		{name: "user handoff", change: func(r *domain.ActivationRequest) {
			r.Plan.Scope = domain.ScopeUser
			r.Plan.TargetContext = domain.TargetContext{Scope: domain.ScopeUser, UserConfigRoot: r.Client.ConfigRoot}
		}},
		{name: "project CLI registry", change: func(r *domain.ActivationRequest) {
			r.Client.ClientID = domain.ClientClaude
			r.Plan.ClientID = domain.ClientClaude
			r.Delivery.ClientID = domain.ClientClaude
		}},
		{name: "project prepared", change: func(r *domain.ActivationRequest) {
			r.Client.ClientID = domain.ClientGrok
			r.Plan.ClientID = domain.ClientGrok
			r.Delivery.ClientID = domain.ClientGrok
		}},
		{name: "missing project", change: func(r *domain.ActivationRequest) { r.Plan.TargetContext.ProjectRoot = "" }},
		{name: "mixed scope", change: func(r *domain.ActivationRequest) { r.Plan.TargetContext.Scope = domain.ScopeUser }},
		{name: "mixed original user", change: func(r *domain.ActivationRequest) {
			r.Plan.TargetContext.UserConfigRoot = filepath.Join(r.Client.ConfigRoot, "other")
		}},
		{name: "unclean project", change: func(r *domain.ActivationRequest) { r.Plan.TargetContext.ProjectRoot += string(filepath.Separator) }},
		{name: "client mismatch", change: func(r *domain.ActivationRequest) { r.Plan.ClientID = domain.ClientCodex }, wantError: true},
		{name: "selected delivery", change: func(r *domain.ActivationRequest) {
			if err := json.Unmarshal([]byte(`{"mode":"unknown-test-delivery"}`), &r.Plan.SelectedDelivery); err != nil {
				panic(err)
			}
		}, wantError: true},
		{name: "profile recheck failure", profileFails: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			active := filepath.Join(base, "project", "managed", "package")
			if err := os.MkdirAll(active, 0700); err != nil {
				t.Fatal(err)
			}
			user := filepath.Join(base, "user")
			req := domain.ActivationRequest{
				Client:       domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: user},
				Plan:         domain.DeliveryPlan{ClientID: domain.ClientCline, Scope: domain.ScopeProject, ActivePath: active, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: filepath.Join(base, "project"), UserConfigRoot: user}},
				Delivery:     domain.StagedDelivery{ClientID: domain.ClientCline, ActivePath: active, OwnedBase: filepath.Dir(active)},
				DeclaredName: "handoff", PackageUnchanged: true, VerifyOnly: true,
			}
			if tc.change != nil {
				tc.change(&req)
			}
			adapter := &handoffCaptureAdapter{id: req.Client.ClientID}
			registry, err := clients.NewRegistry(adapter)
			if err != nil {
				t.Fatal(err)
			}
			binding := domain.ClientBinding{ClientBindingID: "current", ClientID: string(req.Client.ClientID), TargetLocator: active, Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared}
			state := domain.StateFileV2{Installations: []domain.Installation{{InstallationID: "group", Clients: map[string]domain.ClientBinding{"current": binding}}}}
			kernel := nativeconfig.New()
			profiles, callbacks := 0, 0
			profileErr := errors.New("profile changed")
			seam := seamActivator{
				inner: providers.Activator{Registry: registry, NativeConfig: &kernel}, store: groupReportingStore{state: state},
				// Group facts may refer to another target. The current request
				// supplies the client's qualified scope and original user root.
				facts: BindingFacts{InstallationID: "group", BindingID: "first", ClientID: "codex"},
				profileCheck: func(context.Context) error {
					profiles++
					if tc.profileFails {
						return profileErr
					}
					return nil
				},
				onCommitted: func(context.Context, BindingFacts) error { callbacks++; return nil },
			}
			_, err = seam.Activate(t.Context(), req)
			if (err != nil) != tc.wantError || callbacks != 0 {
				t.Fatalf("err=%v callbacks=%d", err, callbacks)
			}
			if tc.profileFails && !errors.Is(err, profileErr) {
				t.Fatal("profile failure lost", err)
			}
			if tc.wantError {
				if len(adapter.requests) != 0 {
					t.Fatal("rejected request reached adapter")
				}
				return
			}
			if profiles != 1 || len(adapter.requests) != 1 || adapter.requests[0].VerifyOnly != tc.wantVerify {
				t.Fatalf("profiles=%d requests=%+v", profiles, adapter.requests)
			}
		})
	}
}
