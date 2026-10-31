package gemini

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/tailscale/hujson"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ActivationPreflighter = (*Adapter)(nil)

// PreflightActivation runs before the provider's generic attestation shortcut.
// Native configuration ownership cannot prove discovery in a Gemini session.
func (*Adapter) PreflightActivation(env clients.Env, request domain.ActivationRequest) error {
	if request.Plan.Scope != domain.ScopeProject {
		return nil
	}
	if _, err := geminiPlanRoot(request.Client, request.Plan); err != nil {
		return err
	}
	if _, err := geminiProjectConditions(env.NativeConfig, request); err != nil {
		return err
	}
	if request.ActivationComplete {
		return fmt.Errorf("gemini project native discovery cannot be established by activation attestation; inspect skills and MCP servers in a fresh trusted session")
	}
	return nil
}

func geminiProjectStatus(env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if request.Plan.Scope != domain.ScopeProject || outcome.Verification != domain.VerificationInstalled {
		return outcome, nil
	}
	outcome.Activation = domain.ActivationPrepared
	outcome.ActivationAttested = false
	outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Gemini project configuration is installed; native discovery, workspace trust, system policy and MCP initialization remain unobserved; start Gemini CLI in the selected project and inspect skills list and mcp list")
	conditions, err := geminiProjectConditions(env.NativeConfig, request)
	if err != nil {
		return outcome, fmt.Errorf("gemini project configuration was installed but effective settings could not be inspected: %w", err)
	}
	for _, condition := range conditions {
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, condition)
	}
	return outcome, nil
}

type geminiProjectSettings struct {
	Skills struct {
		Enabled  *bool    `json:"enabled"`
		Disabled []string `json:"disabled"`
	} `json:"skills"`
	MCP struct {
		Allowed  *[]string `json:"allowed"`
		Excluded *[]string `json:"excluded"`
	} `json:"mcp"`
	Admin struct {
		Skills struct {
			Enabled *bool `json:"enabled"`
		} `json:"skills"`
		MCP struct {
			Enabled *bool `json:"enabled"`
		} `json:"mcp"`
	} `json:"admin"`
}

func readGeminiProjectSettings(kernel nativeconfig.Kernel, root string) (geminiProjectSettings, error) {
	settings := geminiProjectSettings{}
	// Inspect applies the same comment-only parser and uniqueness rules as the
	// mutation path without creating a file or claiming ownership of this name.
	if _, _, err := kernel.Inspect(GeminiConfigPaths(root), nativeconfig.CodecGemini, "agentplugins-policy-inspection", nil); err != nil {
		return settings, err
	}
	file, err := kernel.ReadExactFile(filepath.Join(root, "settings.json"))
	if err != nil || !file.Exists {
		return settings, err
	}
	if len(file.Body) > 1<<20 {
		return settings, fmt.Errorf("gemini settings exceed the inspection limit")
	}
	standard, err := hujson.Standardize(file.Body)
	if err != nil {
		return settings, err
	}
	if _, err := shared.DecodeStrictJSONObject(standard); err != nil {
		return settings, err
	}
	if err := json.Unmarshal(standard, &settings); err != nil {
		return settings, fmt.Errorf("invalid Gemini component settings: %w", err)
	}
	return settings, nil
}

// These are configured user/workspace restrictions, not a second Gemini
// settings engine. Higher system policy and session trust remain unobserved.
func geminiProjectConditions(kernel nativeconfig.Kernel, request domain.ActivationRequest) ([]string, error) {
	user, err := readGeminiProjectSettings(kernel, request.Client.ConfigRoot)
	if err != nil {
		return nil, err
	}
	workspace, err := readGeminiProjectSettings(kernel, request.Plan.NativeRegistryRoot)
	if err != nil {
		return nil, err
	}
	conditions := geminiConfiguredRestrictions(user, workspace, request.Plan.Components)
	for _, component := range request.Plan.Components {
		if component.Kind != domain.ComponentSkill {
			continue
		}
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return nil, err
		}
		alias := filepath.Join(request.Plan.TargetContext.ProjectRoot, ".agents", "skills", component.Name, "SKILL.md")
		file, err := kernel.ReadExactFile(alias)
		if err != nil {
			return nil, fmt.Errorf("inspect Gemini workspace skill alias: %w", err)
		}
		if file.Exists {
			conditions = append(conditions, fmt.Sprintf("workspace .agents/skills/%s/SKILL.md may shadow the managed .gemini skill; native discovery remains pending", component.Name))
		}
	}
	return conditions, nil
}

func geminiConfiguredRestrictions(user, workspace geminiProjectSettings, components []domain.ComponentDecision) []string {
	conditions := []string{}
	skills := geminiConfiguredEnabled(user.Skills.Enabled, workspace.Skills.Enabled)
	adminSkills := geminiConfiguredEnabled(user.Admin.Skills.Enabled, workspace.Admin.Skills.Enabled)
	adminMCP := geminiConfiguredEnabled(user.Admin.MCP.Enabled, workspace.Admin.MCP.Enabled)
	allowed, excluded := user.MCP.Allowed, user.MCP.Excluded
	if workspace.MCP.Allowed != nil {
		allowed = workspace.MCP.Allowed
	}
	if workspace.MCP.Excluded != nil {
		excluded = workspace.MCP.Excluded
	}
	disabled := append(slices.Clone(user.Skills.Disabled), workspace.Skills.Disabled...)
	for _, component := range components {
		if component.Kind == domain.ComponentSkill && (!skills || !adminSkills || slices.Contains(disabled, component.Name)) {
			conditions = append(conditions, fmt.Sprintf("preserved Gemini user/workspace settings disable skill %q; it remains configured pending native discovery", component.Name))
		}
		if component.Kind == domain.ComponentMCPServer && (!adminMCP || allowed != nil && !slices.Contains(*allowed, component.Name) || excluded != nil && slices.Contains(*excluded, component.Name)) {
			conditions = append(conditions, fmt.Sprintf("preserved Gemini user/workspace settings restrict MCP server %q; it remains configured pending native discovery", component.Name))
		}
	}
	return conditions
}

func geminiConfiguredEnabled(user, workspace *bool) bool {
	if workspace != nil {
		return *workspace
	}
	return user == nil || *user
}
