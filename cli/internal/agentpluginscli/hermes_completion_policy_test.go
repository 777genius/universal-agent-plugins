package agentpluginscli

import (
	"encoding/json"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestHermesCompletionRepairConsentUsesWholeEnvelope(t *testing.T) {
	for name, mutate := range map[string]func(*domain.PackageEnvelope){
		"pure":             func(*domain.PackageEnvelope) {},
		"empty extensions": func(e *domain.PackageEnvelope) { e.Manifest.RawExtensions = json.RawMessage(`{}`) },
		"no skills":        func(e *domain.PackageEnvelope) { e.Skills = nil },
		"disabled mcp":     func(e *domain.PackageEnvelope) { e.MCP.Present = true },
		"inventory mcp":    func(e *domain.PackageEnvelope) { e.Inventory.MCPPresent = true },
		"invalid mcp": func(e *domain.PackageEnvelope) {
			e.MCP.InvalidServer = map[string]domain.Diagnostic{"invalid": {}}
		},
		"app":           func(e *domain.PackageEnvelope) { e.App.Declared = true },
		"inventory app": func(e *domain.PackageEnvelope) { e.Inventory.AppPresent = true },
		"extension": func(e *domain.PackageEnvelope) {
			e.Manifest.Extensions = map[string]json.RawMessage{"unknown": json.RawMessage(`{}`)}
		},
		"ignored invalid extension": func(e *domain.PackageEnvelope) {
			e.Diagnostics = []domain.Diagnostic{{Severity: domain.SeverityWarning, Code: "plugin_extensions_ignored"}}
		},
		"command": func(e *domain.PackageEnvelope) {
			e.Commands = map[string]domain.Command{"cmd": {Name: "cmd"}}
		},
		"invalid command": func(e *domain.PackageEnvelope) { e.InvalidCommands = []string{"cmd"} },
		"invalid skill":   func(e *domain.PackageEnvelope) { e.Inventory.InvalidSkills = []string{"bad"} },
		"invalid root":    func(e *domain.PackageEnvelope) { e.Inventory.InvalidSkillsRoot = true },
	} {
		t.Run(name, func(t *testing.T) {
			envelope := domain.PackageEnvelope{Skills: map[string]domain.Skill{"docs": {Name: "docs"}}, Inventory: domain.ComponentInventory{Skills: []string{"docs"}}}
			mutate(&envelope)
			request := installer.Request{Scope: domain.ScopeProject, ProjectTrustConsent: true}
			action := dependencyAction{Action: environment.DesiredAction(installer.OpRepair)}
			want := name == "pure" || name == "empty extensions"
			if got := completeEnvelopeTrustConsent(request, action, envelope); got != want {
				t.Fatalf("repair trust consent=%v want=%v", got, want)
			}
			action.Action = environment.ActionInstall
			if !completeEnvelopeTrustConsent(request, action, envelope) {
				t.Fatal("changed existing fresh-install consent policy")
			}
			action.Action, request.ProjectTrustConsent = environment.DesiredAction(installer.OpRepair), false
			if completeEnvelopeTrustConsent(request, action, envelope) {
				t.Fatal("ordinary repair inherited explicit-add consent")
			}
		})
	}
}

func TestHermesCompletionRetryRequiresExpandedDisclosureReview(t *testing.T) {
	if !skillsRetryDisclosureCovered([]string{"reviewed", "unrelated"}, []string{"reviewed"}) ||
		skillsRetryDisclosureCovered([]string{"reviewed"}, []string{"reviewed", "broader trust"}) {
		t.Fatal("retry coverage ignored expanded disclosure or rejected a subset")
	}
}
