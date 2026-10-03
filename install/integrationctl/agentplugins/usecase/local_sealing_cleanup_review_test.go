package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Delegate real staging, verification and discard. Only the returned digest is
// damaged, after the real package and owned PLUGIN_DATA exist on disk.
type rejectLocalSealStager struct {
	ports.PackageStager
	staged        domain.StagedDelivery
	markerAtStage []byte
}

func (s *rejectLocalSealStager) StageWithPluginData(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operation string, hints domain.CompatibilityHints, dataPath string) (domain.StagedDelivery, error) {
	delivery, err := s.PackageStager.(ports.PluginDataAwareStager).StageWithPluginData(ctx, envelope, plan, operation, hints, dataPath)
	if err != nil {
		return delivery, err
	}
	if err := s.Verify(ctx, delivery.StagingPath, delivery.ArtifactDigest); err != nil {
		return delivery, err
	}
	s.markerAtStage, err = os.ReadFile(filepath.Join(dataPath, ".agentplugins-data-owner.json"))
	if err != nil {
		return delivery, err
	}
	s.staged = delivery
	delivery.ArtifactDigest = "TEST-invalid-projection-digest"
	return delivery, nil
}

// Red on 72d9: new/ leaves a real owned data directory without a state receipt.
// retained/ protects existing data, its ownership marker, and retained state.
// The existing TEST Local adapter selects the mode; this is not native NewLocal
// qualification and launches no client or package runtime.
func TestLocalSelectionSealFailureCleansOnlyNewPluginData(t *testing.T) {
	for _, disposition := range []string{"new", "retained"} {
		t.Run(disposition, func(t *testing.T) {
			root := localProcessRoot(t)
			adapter := &testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}
			service := localGroupService(t, root, adapter)
			input := localGroupInput(t, root)
			input.Confirmed = true
			manager := service.PluginData.(providers.PluginDataManager)
			var retainedMarker []byte
			if disposition == "retained" {
				installed, err := service.Add(t.Context(), input)
				if err != nil || !installed.Mutated {
					t.Fatalf("seed real install: %+v, %v", installed, err)
				}
				input.InstallationID = installed.InstallationID
				removed, err := service.Remove(t.Context(), usecase.RemoveInput{SelectedDelivery: installed.Plan.SelectedDelivery, Selector: installed.InstallationID, Client: input.Client, Scope: input.Scope, Confirmed: true})
				if err != nil || !removed.Mutated {
					t.Fatalf("retain data through normal remove: %+v, %v", removed, err)
				}
				state, err := service.StateStore.Load()
				if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained || len(state.Installations[0].DataReceipts) != 1 {
					t.Fatalf("retained setup state: %+v, %v", state, err)
				}
				for _, receipt := range state.Installations[0].DataReceipts {
					retainedMarker, err = os.ReadFile(filepath.Join(receipt.Locator, ".agentplugins-data-owner.json"))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(receipt.Locator, "persistent-marker"), []byte("keep these retained bytes"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			// A foreign sibling must survive cleanup of the exact owned locator.
			foreign := filepath.Join(manager.Base, "TEST-unowned-sibling", "foreign-marker")
			if err := os.MkdirAll(filepath.Dir(foreign), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(foreign, []byte("foreign bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			statePath := filepath.Join(root, "state", "state-v2.json")
			stateBefore, stateBeforeErr := os.ReadFile(statePath)
			if stateBeforeErr != nil && !errors.Is(stateBeforeErr, os.ErrNotExist) {
				t.Fatal(stateBeforeErr)
			}
			profilePath := filepath.Join(root, "profile", "settings.json")
			profileBefore, err := os.ReadFile(profilePath)
			if err != nil {
				t.Fatal(err)
			}
			s := &rejectLocalSealStager{PackageStager: service.Stager}
			service.Stager = s
			result, err := service.Add(t.Context(), input)
			if err == nil || err.Error() != "selected delivery projection digest is invalid" || result.Mutated || result.Receipt.OperationID != "" {
				t.Fatalf("seal failure result: %+v, %v", result, err)
			}
			facts, local := result.Plan.SelectedDelivery.LocalFacts()
			if !local || !facts.NativeStop || s.staged.StagingPath == "" || !strings.HasPrefix(s.staged.ArtifactDigest, "sha256:") {
				t.Fatal("did not reach selected Local seal after verified real staging")
			}
			var receipt domain.DataReceipt
			if err := json.Unmarshal(s.markerAtStage, &receipt); err != nil {
				t.Fatal(err)
			}
			if receipt.Locator != filepath.Join(manager.Base, result.Plan.PhysicalArtifactID) || receipt.State != domain.DataReceiptOwned || receipt.DataReceiptID == "" {
				t.Fatalf("staging did not use exact owned PLUGIN_DATA: %+v", receipt)
			}
			for _, path := range []string{s.staged.StagingPath, result.Plan.ActivePath} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("staging/active package survived seal rejection: %s: %v", path, err)
				}
			}
			stateAfter, stateAfterErr := os.ReadFile(statePath)
			if string(stateBefore) != string(stateAfter) || (stateBeforeErr == nil) != (stateAfterErr == nil) || (stateAfterErr != nil && !errors.Is(stateAfterErr, os.ErrNotExist)) {
				t.Errorf("seal rejection changed durable state: before=%v after=%v", stateBeforeErr, stateAfterErr)
			}
			profileAfter, err := os.ReadFile(profilePath)
			if err != nil || string(profileBefore) != string(profileAfter) || len(readLocalDocument(t, profilePath)["chat.pluginLocations"].(map[string]any)) != 0 {
				t.Errorf("seal rejection changed profile/registered package: %v", err)
			}
			if body, err := os.ReadFile(foreign); err != nil || string(body) != "foreign bytes" {
				t.Errorf("cleanup damaged foreign sibling: %q, %v", body, err)
			}
			if disposition == "new" {
				if _, err := os.Lstat(receipt.Locator); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("NEW owned PLUGIN_DATA orphan survived seal rejection without a state receipt: %v", err)
				}
			} else {
				markerAfter, err := os.ReadFile(filepath.Join(receipt.Locator, ".agentplugins-data-owner.json"))
				if err != nil || string(markerAfter) != string(retainedMarker) || string(s.markerAtStage) != string(retainedMarker) {
					t.Errorf("retained ownership receipt bytes changed: %v", err)
				}
				if body, err := os.ReadFile(filepath.Join(receipt.Locator, "persistent-marker")); err != nil || string(body) != "keep these retained bytes" {
					t.Errorf("retained data bytes changed: %q, %v", body, err)
				}
				if err := manager.ValidateData(t.Context(), receipt); err != nil {
					t.Errorf("retained receipt no longer validates: %v", err)
				}
			}
			t.Log("verified real staging and data marker; seal error unchanged; staging/active package absent; state/profile/foreign sibling preserved")
		})
	}
}
