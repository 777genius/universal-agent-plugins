package gemini

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Native inventory is observed from the selected project without a prompt.
// Unsupported components or remote transports cannot inherit stdio readiness.
func observeGeminiProjectComponents(ctx context.Context, env clients.Env, request domain.ActivationRequest) (bool, error) {
	if env.Runner == nil || len(request.Plan.Components) == 0 {
		return false, nil
	}
	if err := VerifyGeminiNativeObjects(request.Plan.NativeRegistryRoot, request.Delivery.NativeObjects, false, env.NativeConfig); err != nil {
		return false, err
	}
	skills := []domain.ComponentDecision{}
	for _, component := range request.Plan.Components {
		if component.Support != domain.SupportNative {
			return false, nil
		}
		switch component.Kind {
		case domain.ComponentSkill:
			skills = append(skills, component)
		case domain.ComponentMCPServer:
		default:
			return false, nil
		}
	}
	conditions, err := geminiProjectConditions(env.NativeConfig, request)
	if err != nil {
		return false, err
	}
	if len(conditions) != 0 {
		return false, nil
	}
	if len(skills) > 0 {
		result, probeErr := runGeminiInventory(ctx, env, request, "skills", "list", "--all")
		if probeErr != nil {
			return false, probeErr
		}
		if !geminiSkillsListed(result.Stdout, request.Plan.NativeRegistryRoot, skills) || !geminiSkillMetadataMatches(ctx, env.NativeConfig, result.Stdout) {
			return false, nil
		}
	}
	if observed, probeErr := observeGeminiProjectMCP(ctx, env, request); probeErr != nil || !observed {
		return false, probeErr
	}
	return VerifyGeminiNativeObjects(request.Plan.NativeRegistryRoot, request.Delivery.NativeObjects, false, env.NativeConfig) == nil, nil
}

func runGeminiInventory(ctx context.Context, env clients.Env, request domain.ActivationRequest, args ...string) (legacyports.CommandResult, error) {
	executable := request.Client.ExecutablePath
	if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable || strings.ContainsAny(executable, "\x00\r\n") {
		return legacyports.CommandResult{}, fmt.Errorf("gemini native inventory authority is unavailable")
	}
	environment, err := New().VersionProbeEnvironment(request.Client.ConfigRoot)
	if err != nil {
		return legacyports.CommandResult{}, fmt.Errorf("gemini native inventory authority is unavailable")
	}
	for _, key := range []string{"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR", "GEMINI_RESTRICTED_MODE", "GEMINI_CLI_TRUST_WORKSPACE"} {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	environment = append(environment, "NO_COLOR=1", "TERM=dumb")
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	result, err := shared.RunNativeRegistry(bounded, env.Runner, legacyports.Command{Argv: append([]string{executable}, args...), Dir: request.Plan.TargetContext.ProjectRoot, Env: environment, StdoutLimitBytes: 1 << 20})
	if err != nil {
		return result, err
	}
	if result.ExitCode != 0 || len(result.Stdout) > 1<<20 || len(result.Stderr) > 1<<20 {
		return result, fmt.Errorf("gemini native inventory did not complete cleanly")
	}
	return result, nil
}

// Gemini 0.65.0 emits three-line records. Reject unknown formats and duplicate
// identities rather than inferring discovery from a skill name substring.
func geminiSkillsListed(body []byte, root string, components []domain.ComponentDecision) bool {
	if len(body) > 1<<20 || !strings.HasPrefix(string(body), "Discovered Agent Skills:\n\n") {
		return false
	}
	records := strings.Split(strings.TrimSuffix(strings.TrimPrefix(string(body), "Discovered Agent Skills:\n\n"), "\n\n"), "\n\n")
	found := map[string]string{}
	for _, record := range records {
		lines := strings.Split(record, "\n")
		if len(lines) != 3 || !strings.HasPrefix(lines[1], "  Description: ") || !strings.HasPrefix(lines[2], "  Location:    ") {
			return false
		}
		name, status, ok := strings.Cut(lines[0], " [")
		if !ok || name == "" || strings.ContainsAny(name, "\x00\r\n\x1b") {
			return false
		}
		if _, duplicate := found[name]; duplicate {
			return false
		}
		switch status {
		case "Enabled]", "Disabled]", "Enabled] [Built-in]", "Disabled] [Built-in]":
		default:
			return false
		}
		location := strings.TrimPrefix(lines[2], "  Location:    ")
		if !filepath.IsAbs(location) || filepath.Clean(location) != location {
			return false
		}
		if status == "Enabled]" {
			found[name] = location
		} else {
			found[name] = ""
		}
	}
	for _, component := range components {
		if component.Kind != domain.ComponentSkill || found[component.Name] != filepath.Join(root, "skills", component.Name, "SKILL.md") {
			return false
		}
	}
	return len(components) > 0
}

// Human list descriptions are untrusted Skill metadata. Binding every rendered
// record to its on-disk metadata prevents multiline descriptions from injecting
// synthetic inventory records that imitate a managed skill.
func geminiSkillMetadataMatches(ctx context.Context, kernel nativeconfig.Kernel, body []byte) bool {
	records := strings.Split(strings.TrimSuffix(strings.TrimPrefix(string(body), "Discovered Agent Skills:\n\n"), "\n\n"), "\n\n")
	for _, record := range records {
		lines := strings.Split(record, "\n")
		if len(lines) != 3 {
			return false
		}
		name, _, ok := strings.Cut(lines[0], " [")
		if !ok {
			return false
		}
		path := strings.TrimPrefix(lines[2], "  Location:    ")
		if filepath.Base(path) != "SKILL.md" {
			return false
		}
		file, err := kernel.ReadExactFile(path)
		if err != nil || !file.Exists || len(file.Body) > 1<<20 {
			return false
		}
		skill, err := conformance.ParseSkill(ctx, name, file.Body)
		if err != nil || skill.Name != name || skill.Description != strings.TrimPrefix(lines[1], "  Description: ") {
			return false
		}
	}
	return true
}
