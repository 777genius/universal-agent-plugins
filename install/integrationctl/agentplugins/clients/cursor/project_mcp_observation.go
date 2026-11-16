package cursor

import (
	"context"
	"fmt"
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

// Cursor's supported MCP commands do not require a model or account login.
// A Skill still requires independent native discovery; MCP cannot qualify it.
type cursorProjectMCPObservation struct{ Initialized, Complete bool }

func observeCursorProjectMCP(ctx context.Context, env clients.Env, request domain.ActivationRequest) (cursorProjectMCPObservation, error) {
	if env.Runner == nil || env.LookupExecutable == nil {
		return cursorProjectMCPObservation{}, nil
	}
	names, allMCP := cursorSelectedMCP(request.Plan.Components)
	if len(names) == 0 {
		return cursorProjectMCPObservation{}, nil
	}
	servers, serverErr := cursorDesiredServers(request.Delivery.ActivePath, cursorNativeObjects(request.Delivery.NativeObjects))
	if serverErr != nil {
		return cursorProjectMCPObservation{}, serverErr
	}
	for _, name := range names {
		if servers[name].Type != "stdio" {
			return cursorProjectMCPObservation{}, nil
		}
	}
	if err := cursorMCPPolicyAvailable(env, request, names); err != nil {
		return cursorProjectMCPObservation{}, err
	}
	layout := cursorComponents(request.Plan.NativeRegistryRoot)
	if err := layout.Verify(cursorNativeObjects(request.Delivery.NativeObjects), false, env.NativeConfig); err != nil {
		return cursorProjectMCPObservation{}, err
	}
	result, err := runCursorInventory(ctx, env, request, "list")
	if err != nil {
		return cursorProjectMCPObservation{}, err
	}
	for _, name := range names {
		if ready, err := observeCursorServer(ctx, env, request, name, result.Stdout); err != nil || !ready {
			return cursorProjectMCPObservation{}, err
		}
	}

	if err := layout.Verify(cursorNativeObjects(request.Delivery.NativeObjects), false, env.NativeConfig); err != nil {
		return cursorProjectMCPObservation{}, err
	}
	if err := cursorMCPPolicyAvailable(env, request, names); err != nil {
		return cursorProjectMCPObservation{}, err
	}
	return cursorProjectMCPObservation{Initialized: true, Complete: allMCP}, nil
}

func cursorSelectedMCP(components []domain.ComponentDecision) ([]string, bool) {
	names := []string{}
	allMCP := len(components) > 0
	for _, component := range components {
		if component.Kind != domain.ComponentMCPServer || component.Support != domain.SupportNative {
			allMCP = false
			continue
		}
		if component.Name == "" || strings.ContainsAny(component.Name, "\x00\r\n\x1b") {
			return nil, false
		}
		names = append(names, component.Name)
	}
	return names, allMCP
}

func runCursorInventory(ctx context.Context, env clients.Env, request domain.ActivationRequest, args ...string) (legacyports.CommandResult, error) {
	return runCursorNative(ctx, env, request, append([]string{"mcp"}, args...)...)
}

func runCursorNative(ctx context.Context, env clients.Env, request domain.ActivationRequest, args ...string) (legacyports.CommandResult, error) {
	executable, err := env.LookupExecutable("cursor-agent")
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	root := request.Client.ConfigRoot
	if !cursorNativeProfileEnvironment(root) {
		return legacyports.CommandResult{}, fmt.Errorf("cursor CLI profile overrides differ from the selected native profile; existing policy remains unchanged")
	}
	project := request.Plan.TargetContext.ProjectRoot
	if filepath.Base(root) != ".cursor" || !filepath.IsAbs(executable) || filepath.Clean(executable) != executable || !cursorExactNativeProject(project) {
		return legacyports.CommandResult{}, fmt.Errorf("cursor native profile or project authority is unavailable")
	}
	environment := []string{"HOME=" + filepath.Dir(root), "USERPROFILE=" + filepath.Dir(root), "CURSOR_CONFIG_DIR=" + root, "CURSOR_DATA_DIR=" + root, "NO_COLOR=1", "TERM=dumb"}
	for _, key := range []string{"PATH", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	bounded, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	result, err := shared.RunNativeRegistry(bounded, env.Runner, legacyports.Command{Argv: append([]string{executable}, args...), Dir: project, Env: environment, StdoutLimitBytes: 1 << 20})
	if err != nil {
		return result, err
	}
	if result.ExitCode != 0 || len(result.Stdout) > 1<<20 || len(result.Stderr) != 0 {
		return result, fmt.Errorf("cursor native MCP command did not complete cleanly")
	}
	return result, nil
}

func cursorExactNativeProject(project string) bool {
	if !filepath.IsAbs(project) || filepath.Clean(project) != project {
		return false
	}
	for path := project; ; path = filepath.Dir(path) {
		if _, err := os.Lstat(filepath.Join(path, ".git")); err == nil {
			return path == project
		} else if !os.IsNotExist(err) {
			return false
		}
		if filepath.Dir(path) == path {
			return true
		}
	}
}

func cursorProjectSlug(project string) string {
	var result strings.Builder
	separator := true
	for _, char := range project {
		ascii := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
		if ascii {
			result.WriteRune(char)
			separator = false
		} else if !separator {
			result.WriteByte('-')
			separator = true
		}
	}
	return strings.TrimSuffix(result.String(), "-")
}

func cursorMCPStatus(body []byte, name string) (string, bool) {
	if len(body) > 1<<20 {
		return "", false
	}
	prefix := name + ": "
	status := ""
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
			status = strings.TrimPrefix(line, prefix)
		}
	}
	return status, count == 1
}

func cursorToolsListed(body []byte, name string) bool {
	if len(body) > 1<<20 {
		return false
	}
	text := string(body)
	return strings.HasPrefix(text, "Tools for "+name+" (") && strings.Contains(strings.SplitN(text, "\n", 2)[0], "):")
}

func cursorObservedMCPActions(actions []string) []string {
	result := actions[:0]
	for _, action := range actions {
		if action != "open the selected project in Cursor and inspect skills and MCP servers" && action != "Cursor project configuration is installed; native discovery, aliases, enablement, approvals, team policy and MCP initialization remain unobserved" {
			result = append(result, action)
		}
	}
	return result
}

func cursorObservedLocalMCPAuthentication(plan domain.DeliveryPlan) bool {
	return plan.Authentication == domain.AuthenticationNotChecked && slices.Contains(plan.Warnings, "authentication_not_catalog_verified")
}

func observeCursorServer(ctx context.Context, env clients.Env, request domain.ActivationRequest, name string, listing []byte) (bool, error) {
	status, ok := cursorMCPStatus(listing, name)
	if !ok {
		return false, fmt.Errorf("cursor did not list the exact selected MCP server")
	}
	if status == "disabled" {
		return false, fmt.Errorf("preserved explicit Cursor MCP disablement for %q", name)
	}
	if status == "not loaded (needs approval)" && !request.VerifyOnly {
		if err := approveCursorMCP(ctx, env, request, name); err != nil {
			return false, err
		}
	} else if status != "ready" {
		return false, nil
	}
	tools, err := runCursorInventory(ctx, env, request, "list-tools", name)
	if err != nil {
		return false, err
	}
	if !cursorToolsListed(tools.Stdout, name) {
		return false, fmt.Errorf("cursor MCP initialization remains unobserved for %q", name)
	}
	return true, nil
}

func cursorNativeProfileEnvironment(root string) bool {
	for _, key := range []string{"CURSOR_CONFIG_DIR", "CURSOR_DATA_DIR"} {
		if value := os.Getenv(key); strings.TrimSpace(value) != "" && value != root {
			return false
		}
	}
	if strings.TrimSpace(os.Getenv("CURSOR_CONFIG_DIR")) == "" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); strings.TrimSpace(xdg) != "" && filepath.Join(xdg, "cursor") != root {
			return false
		}
	}
	return true
}
