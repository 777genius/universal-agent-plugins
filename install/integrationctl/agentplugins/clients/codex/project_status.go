package codex

import (
	"fmt"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ActivationPreflighter = (*Adapter)(nil)

func (*Adapter) PreflightActivation(env clients.Env, request domain.ActivationRequest) error {
	if request.Plan.Scope != domain.ScopeProject && request.Plan.TargetContext.Scope != domain.ScopeProject {
		return nil
	}
	if _, err := codexProjectPlanRoot(request.Client, request.Plan); err != nil {
		return err
	}
	if _, err := codexProjectConditions(env.NativeConfig, request); err != nil {
		return err
	}
	if request.ActivationComplete {
		return fmt.Errorf("codex project native discovery cannot be established by activation attestation; inspect skills and MCP in a fresh trusted project session")
	}
	return nil
}

func refineCodexProjectPlan(input clients.PlanInput, plan *domain.DeliveryPlan) error {
	if _, err := codexProjectPlanRoot(input.Client, *plan); err != nil {
		return err
	}
	for i := range plan.Components {
		component := &plan.Components[i]
		if component.Kind == domain.ComponentMCPServer && input.Envelope.MCP.Servers[component.Name].Type == "sse" {
			component.Support = domain.SupportUnsupported
			component.Reason = "Codex project MCP supports stdio and streamable-http; SSE is unsupported"
		}
	}
	if !shared.OnlyNativeComponents(plan.Components) {
		plan.Status, plan.Activation = domain.PlanUnsupported, domain.ActivationNotRequired
		return nil
	}
	shared.PromoteNativeReady(plan, plan.NativeRegistryRoot, true)
	plan.UserActions = shared.AppendUnique(plan.UserActions, codexProjectDiscoveryAction)
	return nil
}

const codexProjectDiscoveryAction = "Codex project components are configured; native discovery, project trust, effective managed/session policy and MCP initialization remain unobserved; start Codex in the selected project and inspect skills and `codex mcp list --json`"

type codexProjectSettings struct {
	MCPServers map[string]struct {
		Enabled *bool `toml:"enabled"`
	} `toml:"mcp_servers"`
	Skills struct {
		IncludeInstructions *bool                `toml:"include_instructions"`
		Config              []codexSkillSelector `toml:"config"`
	} `toml:"skills"`
}

func readCodexProjectSettings(kernel nativeconfig.Kernel, path string) (codexProjectSettings, error) {
	var settings codexProjectSettings
	if _, _, err := kernel.Inspect(nativeconfig.Paths{JSON: path}, nativeconfig.CodecCodex, "agentplugins-policy-inspection", nil); err != nil {
		return settings, err
	}
	file, err := kernel.ReadExactFile(path)
	if err != nil || !file.Exists {
		return settings, err
	}
	if len(file.Body) > 1<<20 {
		return settings, fmt.Errorf("codex config exceeds component inspection limit")
	}
	if err := toml.Unmarshal(file.Body, &settings); err != nil {
		return settings, fmt.Errorf("codex component settings are invalid")
	}
	return settings, nil
}

// These preserve visible user/project restrictions. They deliberately do not
// reconstruct native profile, trust, managed, ancestor or session precedence.
func codexProjectConditions(kernel nativeconfig.Kernel, request domain.ActivationRequest) ([]string, error) {
	var conditions []string
	paths := []string{filepath.Join(request.Client.ConfigRoot, "config.toml"), codexProjectConfigPath(request.Plan.NativeRegistryRoot)}
	for i, path := range paths {
		settings, err := readCodexProjectSettings(kernel, path)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, codexConfiguredRestrictions(settings, request, i == 0)...)
	}
	return conditions, nil
}

func codexConfiguredRestrictions(settings codexProjectSettings, request domain.ActivationRequest, userLayer bool) []string {
	var conditions []string
	for _, component := range request.Plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if component.Kind == domain.ComponentMCPServer {
			if enabled := settings.MCPServers[component.Name].Enabled; enabled != nil && !*enabled {
				conditions = append(conditions, fmt.Sprintf("preserved Codex settings disable MCP server %q; configuration does not prove native discovery", component.Name))
			}
		}
		if component.Kind == domain.ComponentSkill {
			if settings.Skills.IncludeInstructions != nil && !*settings.Skills.IncludeInstructions {
				conditions = append(conditions, "preserved Codex settings disable automatic skill instructions; explicit native discovery remains unobserved")
			}
			if userLayer && codexUserSkillDisabled(settings.Skills.Config, component.Name, filepath.Join(codexProjectSkillPath(request.Plan.NativeRegistryRoot, component.Name), "SKILL.md")) {
				conditions = append(conditions, fmt.Sprintf("preserved Codex user settings disable skill %q; session overrides and native effective enablement remain unobserved", component.Name))
			}
		}
	}
	return conditions
}
