package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Observe fresh native Skills and MCP catalogs without a model turn. Only
// locally initialized stdio servers qualify; remote authentication stays pending.
func observeOpenCodeNativeComponents(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if outcome.Verification != domain.VerificationInstalled || len(request.Plan.Components) == 0 {
		return outcome, nil
	}
	outcome = openCodeNativeStatus(request, outcome)
	if env.Runner == nil || request.Client.ExecutablePath == "" {
		return outcome, nil
	}
	skills, servers, ok := openCodeObservableComponents(request.Plan.Components)
	if !ok {
		return outcome, nil
	}
	layout, err := openCodeActivationLayout(request)
	if err != nil || (layout.Scope == domain.ScopeProject && openCodeProjectSkillAliases(layout, skills)) {
		return outcome, nil
	}
	observed, err := openCodeCatalogsObserved(ctx, env, request, layout, skills, servers)
	if err != nil {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return shared.FailedActivation(outcome, "confirm prior OpenCode discovery process cleanup before retrying", err)
	}
	if !observed {
		return outcome, nil
	}
	outcome.Activation = domain.ActivationActive
	outcome.ActivationAttested = false
	// Successful local initialization qualifies native setup only, not future
	// model access or authentication used inside a server's tools.
	if len(servers) > 0 && outcome.Authentication == domain.AuthenticationNotChecked && request.Plan.Authentication == domain.AuthenticationNotChecked && slices.Contains(request.Plan.Warnings, "authentication_not_catalog_verified") {
		outcome.Authentication = domain.AuthenticationNotRequired
	}
	// Keep unrelated cleanup warnings; remove only actions this probe superseded.
	actions := outcome.UserActions[:0]
	for _, action := range outcome.UserActions {
		if action == "restart OpenCode to load the installed plugin" || strings.HasPrefix(action, "OpenCode project configuration is installed;") || strings.HasPrefix(action, "OpenCode user configuration is installed;") || strings.HasPrefix(action, "OpenCode merges ancestor,") {
			continue
		}
		actions = append(actions, action)
	}
	outcome.UserActions = actions
	return outcome, nil
}

func openCodeObservableComponents(components []domain.ComponentDecision) ([]domain.ComponentDecision, []string, bool) {
	skills := []domain.ComponentDecision{}
	servers := []string{}
	for _, component := range components {
		if component.Support == domain.SupportUnsupported {
			return nil, nil, false
		}
		switch component.Kind {
		case domain.ComponentSkill:
			skills = append(skills, component)
		case domain.ComponentMCPServer:
			servers = append(servers, component.Name)
		default:
			return nil, nil, false
		}
	}
	return skills, servers, len(components) > 0
}

// Known same-name external Skills can win in different orders across fresh
// native directory scans. Their presence is not stable activation evidence.
func openCodeProjectSkillAliases(layout openCodeNativeLayout, skills []domain.ComponentDecision) bool {
	for _, component := range skills {
		for _, directory := range []string{".agents", ".claude"} {
			if _, err := os.Lstat(filepath.Join(layout.Root, directory, "skills", component.Name, "SKILL.md")); !os.IsNotExist(err) {
				return true
			}
		}
	}
	return false
}

func openCodeCatalogsObserved(ctx context.Context, env clients.Env, request domain.ActivationRequest, layout openCodeNativeLayout, skills []domain.ComponentDecision, servers []string) (bool, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	environment := append(os.Environ(), "OPENCODE_DISABLE_AUTOUPDATE=1", "OPENCODE_DISABLE_MODELS_FETCH=1", "NO_COLOR=1", "OPENCODE_CONFIG_DIR="+request.Client.ConfigRoot)
	if layout.Scope == domain.ScopeUser {
		environment = append(environment, "OPENCODE_DISABLE_PROJECT_CONFIG=1")
	}
	run := func(args ...string) ([]byte, bool, error) {
		result, err := shared.RunNativeRegistry(probeCtx, env.Runner, legacyports.Command{Argv: append([]string{request.Client.ExecutablePath}, args...), Env: environment, Dir: layout.Root, StdoutLimitBytes: 4 << 20})
		if domain.IsUnclosedClientProbe(err) {
			return nil, false, err
		}
		return result.Stdout, err == nil && result.ExitCode == 0 && len(result.Stdout) <= 4<<20, nil
	}
	config, ok, err := run("debug", "config")
	if err != nil || !ok || !openCodeSkillPermissionsUnrestricted(config) {
		return false, err
	}
	if len(skills) > 0 {
		body, ok, err := run("debug", "skill")
		if err != nil || !ok || !exactOpenCodeSkills(body, layout, skills) {
			return false, err
		}
	}
	if len(servers) > 0 {
		if !exactOpenCodeLocalMCPConfig(env, config, request, servers) {
			return false, nil
		}
		listed, ok, err := run("mcp", "list")
		if err != nil || !ok || !connectedOpenCodeMCP(listed, servers) {
			return false, err
		}
	}
	return true, nil
}

func openCodeSkillPermissionsUnrestricted(body []byte) bool {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(body, &config); err != nil || config == nil {
		return false
	}
	// Listing includes skills denied by permission policy. Until the native
	// permission evaluator is observable, retain prepared for explicit policy.
	if value, ok := config["permission"]; ok && string(value) != "null" {
		return false
	}
	for _, key := range []string{"agent", "mode"} {
		var agents map[string]map[string]json.RawMessage
		if value, ok := config[key]; ok {
			if json.Unmarshal(value, &agents) != nil {
				return false
			}
			for _, agent := range agents {
				if value, ok := agent["tools"]; ok && string(value) != "null" {
					return false
				}
				if value, ok := agent["permission"]; ok && string(value) != "null" {
					return false
				}
			}
		}
	}
	if value, ok := config["tools"]; ok && string(value) != "null" {
		return false
	}
	return true
}

type openCodeObservedSkill struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Content  string `json:"content"`
}

func exactOpenCodeSkills(body []byte, layout openCodeNativeLayout, components []domain.ComponentDecision) bool {
	var skills []openCodeObservedSkill
	if json.Unmarshal(body, &skills) != nil || skills == nil {
		return false
	}
	for _, component := range components {
		path := filepath.Join(layout.Skills, component.Name, "SKILL.md")
		expected, err := os.ReadFile(path)
		if err != nil || len(expected) > 1<<20 {
			return false
		}
		content, ok := openCodeSkillBody(string(expected))
		if !ok {
			return false
		}
		matches := 0
		for _, skill := range skills {
			if skill.Name == component.Name {
				matches++
				if filepath.Clean(skill.Location) != path || strings.ReplaceAll(skill.Content, "\r\n", "\n") != content {
					return false
				}
			}
		}
		if matches != 1 {
			return false
		}
	}
	return true
}
func openCodeSkillBody(document string) (string, bool) {
	document = strings.ReplaceAll(document, "\r\n", "\n")
	if !strings.HasPrefix(document, "---\n") {
		return "", false
	}
	_, content, ok := strings.Cut(document[4:], "\n---\n")
	return content, ok
}
