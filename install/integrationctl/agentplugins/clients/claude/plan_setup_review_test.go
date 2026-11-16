package claude

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestClaudeReviewDisclosesOnlyAutomaticSelectedSetup(t *testing.T) {
	for _, test := range []struct {
		name                  string
		project, mcp, prepare bool
		count                 int
	}{
		{"project skill", true, false, false, 1},
		{"project MCP", true, true, false, 2},
		{"user skill", false, false, false, 0},
		{"user MCP", false, true, false, 1},
		{"preparation", false, true, true, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := newClaudeMarketplaceRequest(t)
			plan := r.Plan
			if !test.project {
				plan.Scope = domain.ScopeUser
				plan.TargetContext = domain.TargetContext{Scope: domain.ScopeUser, UserConfigRoot: r.Client.ConfigRoot}
			}
			if test.mcp {
				plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "selected-server", Support: domain.SupportProjected}, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "withheld-server", Support: domain.SupportUnsupported})
			}
			in := clients.PlanInput{Client: r.Client, TargetContext: plan.TargetContext}
			if test.prepare {
				in.Intent = domain.InstallIntentPrepare
			}
			if err := New().RefinePlan(t.Context(), in, &plan); err != nil {
				t.Fatal(err)
			}
			if len(plan.Warnings) != test.count {
				t.Fatalf("wrong setup disclosure: %q", plan.Warnings)
			}
			joined := strings.Join(plan.Warnings, "\n")
			if test.project && !strings.Contains(joined, fmt.Sprintf("selected project %q in Claude Code", plan.TargetContext.ProjectRoot)) {
				t.Fatalf("exact project missing from review: %q", joined)
			}
			if test.mcp && !test.prepare && (!strings.Contains(joined, "selected-server") || strings.Contains(joined, "withheld-server") || !strings.Contains(joined, "Existing rejections are preserved")) {
				t.Fatalf("MCP review exceeded selected supported servers: %q", joined)
			}
			before := append([]string(nil), plan.Warnings...)
			if err := New().RefinePlan(t.Context(), in, &plan); err != nil || !reflect.DeepEqual(plan.Warnings, before) {
				t.Fatalf("repeated planning duplicated disclosure: %q %v", plan.Warnings, err)
			}
		})
	}
}
