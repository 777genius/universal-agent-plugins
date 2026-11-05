package usecase

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDevinGlobalNewUserReceiptDoesNotRebindLegacyCascade(t *testing.T) {
	root := t.TempDir()
	cascade := filepath.Join(root, "legacy-cascade")
	global := filepath.Join(root, "original-xdg", "devin")
	active := filepath.Join(root, "independent-uap-state", "managed", "clients", "windsurf", "source")
	input := AddInput{
		Client: domain.DetectedClient{ClientID: domain.ClientWindsurf, ConfigRoot: cascade, NativeConfigRoot: global},
		Scope: domain.ScopeUser,
	}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientWindsurf, Scope: domain.ScopeUser,
		NativeRegistryRoot: global, ActivePath: active,
	}
	created := stagedClientBinding(domain.ClientBinding{}, false, input, plan, "new", "now")
	if created.NativeProfileRoot != global {
		t.Fatalf("selected original Devin root was not stored on creation: %q", created.NativeProfileRoot)
	}
	for _, prior := range []domain.ClientBinding{
		{ClientID: string(domain.ClientWindsurf)},
		{ClientID: string(domain.ClientWindsurf), NativeProfileRoot: cascade},
	} {
		got := stagedClientBinding(prior, true, input, plan, "old", "now")
		if got.NativeProfileRoot != prior.NativeProfileRoot {
			t.Fatalf("legacy Cascade receipt was rebound to a current Devin profile: prior=%q new=%q", prior.NativeProfileRoot, got.NativeProfileRoot)
		}
	}
	plan.NativeRegistryRoot = cascade
	if got := stagedClientBinding(domain.ClientBinding{}, false, input, plan, "legacy", "now"); got.NativeProfileRoot != "" {
		t.Fatalf("legacy user creation unexpectedly adopted an unrelated native root: %q", got.NativeProfileRoot)
	}
}
