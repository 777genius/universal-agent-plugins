package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type policyGroupStager struct {
	ports.PackageStager
	deliveries []domain.StagedDelivery
	discards   int
}

func (stager *policyGroupStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, id string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	delivery, err := stager.PackageStager.Stage(ctx, envelope, plan, id, hints)
	if err == nil {
		stager.deliveries = append(stager.deliveries, delivery)
	}
	return delivery, err
}

func (stager *policyGroupStager) Discard(ctx context.Context, delivery domain.StagedDelivery) error {
	stager.discards++
	return stager.PackageStager.Discard(ctx, delivery)
}

func TestInstallationPolicyGroupRechecksAfterFinalConfiguringCallback(t *testing.T) {
	for _, change := range []string{"rules", "denial"} {
		t.Run(change, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			first := addInput(t, client, "https://example.com/policy-final-callback")
			second := first
			second.Client = domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			fingerprint, gateErr := "reviewed", error(nil)
			denied := policyDenied()
			service.InstallationPolicy = installationPolicyFunc(func(context.Context, domain.InstallPolicyRequest) (string, error) { return fingerprint, gateErr })
			stager := &policyGroupStager{PackageStager: service.Stager}
			activator := &failNthGroupActivator{}
			service.Stager, service.Activator = stager, activator
			kernelEffects, callbacks := 0, 0
			service.Kernel.Directory.Fault = func(string) error {
				kernelEffects++
				return errors.New("policy refusal must precede the directory kernel")
			}
			group := GroupInput{Targets: []AddInput{first, second}, Confirmed: true, OperationGroupID: "policy-final-callback"}
			group.Progress = func(event GroupProgressEvent) {
				if event.Phase != GroupProgressConfiguring {
					return
				}
				callbacks++
				if event.ClientID == second.Client.ClientID {
					fingerprint = "changed-after-final-callback"
					if change == "denial" {
						gateErr = denied
					}
				}
			}
			result, err := service.AddGroup(t.Context(), group)
			expected := domain.ErrInstallPolicyChanged
			if change == "denial" {
				expected = denied
			}
			if !errors.Is(err, expected) || result.Mutated || len(result.Receipts) != 0 || kernelEffects != 0 || activator.calls != 0 {
				t.Fatalf("final callback policy change reached mutation: result=%+v err=%v kernel=%d activations=%d", result, err, kernelEffects, activator.calls)
			}
			if callbacks != 2 || len(stager.deliveries) != 2 || stager.discards != 2 {
				t.Fatalf("late refusal did not discard the complete staged group: callbacks=%d stages=%d discards=%d", callbacks, len(stager.deliveries), stager.discards)
			}
			if _, err := os.Lstat(store.Path); !os.IsNotExist(err) {
				t.Fatalf("late policy refusal published state: %v", err)
			}
			for _, delivery := range stager.deliveries {
				for _, path := range []string{delivery.ActivePath, delivery.StagingPath} {
					if _, err := os.Lstat(path); !os.IsNotExist(err) {
						t.Fatalf("late policy refusal retained group bytes at %s: %v", path, err)
					}
				}
			}
		})
	}
}
