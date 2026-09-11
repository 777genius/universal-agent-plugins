package planner

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"testing"
)

func TestPreparationCannotOverrideUnsupportedPackage(t *testing.T) {
	plan := domain.DeliveryPlan{ClientID: domain.ClientKiro, Scope: domain.ScopeUser, Status: domain.PlanUnsupported}
	if err := ApplyInstallIntent(&plan, domain.InstallIntentPrepare); err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanUnsupported {
		t.Fatal("preparation bypassed compatibility")
	}
}

func TestPreparationRequiresSupportedNativeUserTarget(t *testing.T) {
	for _, tc := range []struct {
		client domain.ClientID
		scope  domain.InstallScope
		root   string
		kind   domain.ComponentKind
	}{
		{domain.ClientChatGPT, domain.ScopeUser, "native", domain.ComponentMCPServer},
		{domain.ClientKiro, domain.ScopeProject, "native", domain.ComponentMCPServer},
		{domain.ClientKiro, domain.ScopeUser, "", domain.ComponentMCPServer},
	} {
		plan := domain.DeliveryPlan{ClientID: tc.client, Scope: tc.scope, NativeRegistryRoot: tc.root, Components: []domain.ComponentDecision{{Kind: tc.kind, Support: domain.SupportPrepared}}}
		if err := ApplyInstallIntent(&plan, domain.InstallIntentPrepare); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}
