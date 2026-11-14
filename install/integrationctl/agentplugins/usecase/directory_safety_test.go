package usecase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func TestFreshAddRequiresAbsenceAfterStaging(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "group"}[grouped], func(t *testing.T) {
			service, store, client := serviceFixture(t)
			var active string
			service.Stager = afterStageStager{PackageStager: service.Stager, after: func(d domain.StagedDelivery) error {
				active = d.ActivePath
				if err := os.Mkdir(active, 0700); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(active, "foreign"), []byte("keep add collision"), 0600)
			}}
			input := addInput(t, client, "https://example.com/collision")
			input.Confirmed = true
			var err error
			if grouped {
				_, err = service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{input}, OperationGroupID: "collision-group", Confirmed: true})
			} else {
				_, err = service.Add(context.Background(), input)
			}
			if err == nil {
				t.Fatal("fresh add adopted an occupied path")
			}
			assertSafetyBytes(t, filepath.Join(active, "foreign"), "keep add collision")
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(state.Installations) != 0 {
				t.Fatal("collision committed state")
			}
		})
	}
}

func TestMissingRepairExclusivePublicationPreservesLateForeignDirectory(t *testing.T) {
	service, _, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/repair-collision")
	input.Confirmed = true
	added, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(added.Plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	service.Kernel.Directory.Fault = func(at string) error {
		if at != dirswap.PhaseBackupPending {
			return nil
		}
		if err := os.Mkdir(added.Plan.ActivePath, 0700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(added.Plan.ActivePath, "foreign"), []byte("keep repair collision"), 0600)
	}
	input.OperationID = "repair-collision"
	_, err = service.Repair(context.Background(), input)
	if err == nil {
		t.Fatal("repair adopted foreign content")
	}
	assertSafetyBytes(t, filepath.Join(added.Plan.ActivePath, "foreign"), "keep repair collision")
	r, loadErr := service.Kernel.Directory.Load(input.OperationID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if _, statErr := os.Stat(filepath.Join(r.StagingPath, "plugin.json")); statErr != nil {
		t.Fatalf("recovery staging was discarded: %v", statErr)
	}
	if !strings.Contains(err.Error(), r.StagingPath) {
		t.Fatalf("recovery staging path missing: %v", err)
	}
}

func TestUpdateRetainsChangedBackupAndStaging(t *testing.T) {
	service, _, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/update-race")
	input.Confirmed = true
	added, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(added.Plan.ActivePath, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	input.OperationID = "update-race"
	setEnvelopeVersion(t, &input.Envelope, "2.0.0", "sha256:tree-two", "sha256:manifest-two")
	service.Kernel.Directory.Fault = func(at string) error {
		if at != dirswap.PhaseBackupPending {
			return nil
		}
		return os.WriteFile(filepath.Join(added.Plan.ActivePath, "foreign"), []byte("keep update collision"), 0600)
	}
	_, err = service.Update(context.Background(), input)
	if err == nil {
		t.Fatal("update accepted unreviewed file")
	}
	r, loadErr := service.Kernel.Directory.Load(input.OperationID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	assertSafetyBytes(t, filepath.Join(r.BackupPath, "foreign"), "keep update collision")
	assertSafetyBytes(t, filepath.Join(r.BackupPath, "plugin.json"), string(original))
	if _, err := os.Stat(filepath.Join(r.StagingPath, "plugin.json")); err != nil {
		t.Fatalf("staging copy lost: %v", err)
	}
}

type afterStageStager struct {
	ports.PackageStager
	after func(domain.StagedDelivery) error
}

func (s afterStageStager) Stage(ctx context.Context, e domain.PackageEnvelope, p domain.DeliveryPlan, op string, h domain.CompatibilityHints) (domain.StagedDelivery, error) {
	d, err := s.PackageStager.Stage(ctx, e, p, op, h)
	if err == nil {
		err = s.after(d)
	}
	return d, err
}
func assertSafetyBytes(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || string(body) != want {
		t.Fatalf("bytes at %s = %q, %v; want %q", path, body, err, want)
	}
}

func TestRetainedDataPurgeRejectsMovedBackupMarkerMutation(t *testing.T) {
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "./retained-marker-race")
	input.Confirmed = true
	added, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Remove(context.Background(), RemoveInput{Selector: added.InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true, OperationID: "retain-data"}); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	var data domain.DataReceipt
	for _, receipt := range state.Installations[0].DataReceipts {
		data = receipt
	}
	if data.Locator == "" {
		t.Fatal("missing retained data")
	}
	markerName := ".agentplugins-data-owner.json"
	marker, err := os.ReadFile(filepath.Join(data.Locator, markerName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data.Locator, "user-data"), []byte("persistent bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	var moved dirswap.Receipt
	service.Kernel.Directory.Fault = func(at string) error {
		if at != dirswap.PhaseActivationPending {
			return nil
		}
		open, err := service.Kernel.Directory.ListOpen()
		if err != nil {
			return err
		}
		for _, r := range open {
			if r.ActivePath != data.Locator {
				continue
			}
			moved = r
			assertSafetyBytes(t, filepath.Join(r.BackupPath, markerName), string(marker))
			return os.WriteFile(filepath.Join(r.BackupPath, markerName), []byte("foreign marker bytes"), 0600)
		}
		return nil
	}
	err = service.PurgeRetainedData(context.Background(), added.InstallationID, true)
	if err == nil || moved.BackupPath == "" || !strings.Contains(err.Error(), moved.BackupPath) {
		t.Fatalf("missing purge recovery evidence: %v", err)
	}
	assertSafetyBytes(t, filepath.Join(moved.BackupPath, markerName), "foreign marker bytes")
	assertSafetyBytes(t, filepath.Join(moved.BackupPath, "user-data"), "persistent bytes")
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Installations) != 1 || after.Installations[0].DataReceipts[data.DataReceiptID].Locator != data.Locator {
		t.Fatal("failed purge rewrote original logical receipt")
	}
}
