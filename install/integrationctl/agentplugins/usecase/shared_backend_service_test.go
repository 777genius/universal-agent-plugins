package usecase

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSingleTargetServiceResolvesEitherLogicalSharedSurfaceToOnePhysicalBinding(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	copilot := domain.DetectedClient{ClientID: domain.ClientCopilot, DisplayName: "Copilot", Status: domain.DetectionDetected}
	vscode := domain.DetectedClient{ClientID: domain.ClientVSCode, DisplayName: "VS Code", Status: domain.DetectionDetected}

	add := addInput(t, copilot, "https://example.com/shared-service")
	add.Confirmed = true
	added, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	update := addInput(t, vscode, "https://example.com/shared-service")
	update.OperationID = "operation-two"
	setEnvelopeVersion(t, &update.Envelope, "2.0.0", "sha256:shared-v2", "sha256:shared-manifest-v2")
	update.Confirmed = true
	if _, err := service.Update(context.Background(), update); err != nil {
		t.Fatalf("update through alternate shared surface: %v", err)
	}

	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("shared service created duplicate physical bindings: %+v", state.Installations)
	}
	binding := onlyBinding(state.Installations[0])
	if binding.ClientBindingID != added.Receipt.ClientBindingID || binding.ClientID != string(domain.ClientCopilot) || binding.PackageRevision.Version != "2.0.0" || !reflect.DeepEqual(binding.AffectedSurfaces, []string{"copilot", "vscode"}) || len(binding.Receipts) != 2 {
		t.Fatalf("shared service binding = %+v", binding)
	}
	if got, want := managedPackageOwnershipID(t, binding), "package:copilot:"+binding.PhysicalArtifact; got != want {
		t.Fatalf("shared service ownership = %q, want %q", got, want)
	}

	planned, err := service.RemoveGroup(context.Background(), RemoveGroupInput{
		Selector: added.InstallationID,
		Targets: []RemoveInput{
			{Client: copilot, Scope: domain.ScopeUser, ExternalUninstalled: true},
			{Client: vscode, Scope: domain.ScopeUser, ExternalUninstalled: true},
		},
		OperationGroupID: "shared-remove-plan",
		DryRun:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Targets) != 2 || !planned.Targets[0].Deactivation.ArtifactRemovalAllowed || !planned.Targets[1].Deactivation.ArtifactRemovalAllowed {
		t.Fatalf("shared removal preflight = %+v", planned.Targets)
	}
}

func TestSharedPhysicalOwnerSurvivesAlternateUpdateAndRepairInBothDirections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		owner     domain.ClientID
		alternate domain.ClientID
	}{
		{name: "copilot owned through vscode", owner: domain.ClientCopilot, alternate: domain.ClientVSCode},
		{name: "vscode owned through copilot", owner: domain.ClientVSCode, alternate: domain.ClientCopilot},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service, store, _ := serviceFixture(t)
			owner := domain.DetectedClient{ClientID: test.owner, Status: domain.DetectionDetected}
			alternate := domain.DetectedClient{ClientID: test.alternate, Status: domain.DetectionDetected}
			add := addInput(t, owner, "https://example.com/shared-owner")
			add.Confirmed = true
			added, err := service.Add(context.Background(), add)
			if err != nil {
				t.Fatal(err)
			}

			update := addInput(t, alternate, "https://example.com/shared-owner")
			update.OperationID = "operation-two"
			setEnvelopeVersion(t, &update.Envelope, "2.0.0", "sha256:shared-owner-v2", "sha256:shared-owner-manifest-v2")
			update.Confirmed = true
			updated, err := service.Update(context.Background(), update)
			if err != nil {
				t.Fatalf("alternate update: %v", err)
			}
			if updated.Plan.ClientID != test.alternate {
				t.Fatalf("update result client = %s, want requested %s", updated.Plan.ClientID, test.alternate)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := onlyBinding(state.Installations[0])
			wantObject := "package:" + string(test.owner) + ":" + binding.PhysicalArtifact
			if binding.ClientBindingID != added.Receipt.ClientBindingID || binding.ClientID != string(test.owner) || managedPackageOwnershipID(t, binding) != wantObject {
				t.Fatalf("alternate update changed physical owner: %+v", binding)
			}

			if err := os.RemoveAll(binding.TargetLocator); err != nil {
				t.Fatal(err)
			}
			repair := update
			repair.OperationID = "operation-three"
			repaired, err := service.Repair(context.Background(), repair)
			if err != nil {
				t.Fatalf("alternate repair: %v", err)
			}
			if repaired.Plan.ClientID != test.alternate {
				t.Fatalf("repair result client = %s, want requested %s", repaired.Plan.ClientID, test.alternate)
			}
			state, err = store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding = onlyBinding(state.Installations[0])
			if binding.ClientID != string(test.owner) || managedPackageOwnershipID(t, binding) != wantObject || len(binding.Receipts) != 3 {
				t.Fatalf("alternate repair changed physical owner: %+v", binding)
			}
		})
	}
}

func managedPackageOwnershipID(t *testing.T, binding domain.ClientBinding) string {
	t.Helper()
	value := ""
	for _, object := range binding.NativeObjects {
		if object.Kind != "managed_package_directory" {
			continue
		}
		if value != "" {
			t.Fatalf("multiple managed package objects: %+v", binding.NativeObjects)
		}
		value = object.ObjectID
	}
	if strings.TrimSpace(value) == "" {
		t.Fatalf("managed package object missing: %+v", binding.NativeObjects)
	}
	return value
}
