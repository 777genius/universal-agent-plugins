package usecase

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestResolveWorkspaceLockSourceResolvesRelativeToLockPath(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join("/repo", ".uap-integrations.lock")
	got := resolveWorkspaceLockSource(lockPath, "./plugins/demo")
	want := filepath.Join("/repo", "plugins", "demo")
	if got != want {
		t.Fatalf("resolveWorkspaceLockSource() = %q want %q", got, want)
	}
}

func TestCloneInstallationRecordDeepCopiesNestedState(t *testing.T) {
	t.Parallel()
	original := domain.InstallationRecord{
		IntegrationID: "demo",
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {
				CapabilitySurface:       []string{"commands"},
				EnvironmentRestrictions: []domain.EnvironmentRestrictionCode{domain.RestrictionReloadRequired},
				OwnedNativeObjects:      []domain.NativeObjectRef{{Kind: "config", Path: "/tmp/config.json"}},
				AdapterMetadata:         map[string]any{"workspace_root": "/tmp/workspace"},
				CatalogPolicy:           &domain.CatalogPolicySnapshot{Installation: "AVAILABLE"},
			},
		},
	}

	cloned := cloneInstallationRecord(original)
	target := cloned.Targets[domain.TargetCursor]
	target.CapabilitySurface[0] = "agents"
	target.EnvironmentRestrictions[0] = domain.RestrictionRestartRequired
	target.OwnedNativeObjects[0].Path = "/tmp/other.json"
	target.AdapterMetadata["workspace_root"] = "/tmp/other"
	target.CatalogPolicy.Installation = "ON_INSTALL"
	cloned.Targets[domain.TargetCursor] = target

	unchanged := original.Targets[domain.TargetCursor]
	if !reflect.DeepEqual(unchanged.CapabilitySurface, []string{"commands"}) {
		t.Fatalf("CapabilitySurface mutated: %#v", unchanged.CapabilitySurface)
	}
	if !reflect.DeepEqual(unchanged.EnvironmentRestrictions, []domain.EnvironmentRestrictionCode{domain.RestrictionReloadRequired}) {
		t.Fatalf("EnvironmentRestrictions mutated: %#v", unchanged.EnvironmentRestrictions)
	}
	if unchanged.OwnedNativeObjects[0].Path != "/tmp/config.json" {
		t.Fatalf("OwnedNativeObjects mutated: %#v", unchanged.OwnedNativeObjects)
	}
	if unchanged.AdapterMetadata["workspace_root"] != "/tmp/workspace" {
		t.Fatalf("AdapterMetadata mutated: %#v", unchanged.AdapterMetadata)
	}
	if unchanged.CatalogPolicy.Installation != "AVAILABLE" {
		t.Fatalf("CatalogPolicy mutated: %#v", unchanged.CatalogPolicy)
	}
}

func TestResolveRequestedTargetsRejectsUnknownTarget(t *testing.T) {
	t.Parallel()
	_, err := resolveRequestedTargets(domain.IntegrationManifest{
		Deliveries: []domain.Delivery{{TargetID: domain.TargetCursor}},
	}, []string{"gemini"})
	if err == nil {
		t.Fatal("expected unsupported target error")
	}
	if err.Error() != "manifest does not expose target gemini" {
		t.Fatalf("err = %v", err)
	}
}
