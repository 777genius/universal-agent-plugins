package providers

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestStagingRequiresInjectedDependencies(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	plan := stagingPlan(t, domain.ClientCursor, domain.PackageNative)
	ctx := context.Background()

	bare := Stager{}
	if _, err := bare.Stage(ctx, envelope, plan, "operation", domain.CompatibilityHints{}); !errors.Is(err, errPathPolicyRequired) {
		t.Fatalf("Stage without paths = %v, want errPathPolicyRequired", err)
	}
	if err := bare.Discard(ctx, domain.StagedDelivery{ClientID: plan.ClientID, OwnedBase: plan.TargetRoot, StagingPath: filepath.Join(plan.TargetRoot, ".agentplugins-staging-deadbeef")}); !errors.Is(err, errPathPolicyRequired) {
		t.Fatalf("Discard without paths = %v, want errPathPolicyRequired", err)
	}
	if _, err := bare.ManagedMCPNames(ctx, plan.ClientID, plan.TargetRoot, "sha256:x"); !errors.Is(err, errPathPolicyRequired) {
		t.Fatalf("ManagedMCPNames without paths = %v, want errPathPolicyRequired", err)
	}

	withoutRegistry := Stager{Paths: pathpolicy.Policy{}}
	if _, err := withoutRegistry.Stage(ctx, envelope, plan, "operation", domain.CompatibilityHints{}); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("Stage without a registry = %v, want ErrRegistryRequired", err)
	}
	if err := withoutRegistry.Discard(ctx, domain.StagedDelivery{ClientID: plan.ClientID, OwnedBase: plan.TargetRoot, StagingPath: filepath.Join(plan.TargetRoot, ".agentplugins-staging-deadbeef")}); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("Discard without a registry = %v, want ErrRegistryRequired", err)
	}
	if _, err := withoutRegistry.ManagedMCPNames(ctx, plan.ClientID, plan.TargetRoot, "sha256:x"); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("ManagedMCPNames without a registry = %v, want ErrRegistryRequired", err)
	}
}
