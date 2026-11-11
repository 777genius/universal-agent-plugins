package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type projectKimiDetector struct{ user, executable string }

func (d projectKimiDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientKimi, ConfigRoot: d.user,
		Version: "2.5.2", ExecutablePath: d.executable, Status: domain.DetectionDetected}}, nil
}

func configureProjectKimiJourney(app agentpluginscli.App, user, executable string) agentpluginscli.App {
	app.Detector = projectKimiDetector{user: user, executable: executable}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectKimiJourney(selected, user, executable), nil
	}
	return app
}

func projectKimiOwned(t *testing.T, journey projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := journey.owned(t, project)
	if binding.ClientID != "kimi" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(project, ".kimi-code") || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("Kimi binding lost project authority or conservative native status: %+v", binding)
	}
	for _, path := range []string{filepath.Join(project, ".kimi-code", "skills", "docs", "SKILL.md"), filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")} {
		if projectJourneyRead(t, path) != expected {
			t.Fatalf("Kimi managed/native skill differs at %s", path)
		}
	}
	// The source-backed lifecycle covers MCP as well as Skills: a successful
	// Skills receipt must not hide a missing/mismatched project MCP projection.
	// This checks installed bytes, never vendor trust, MCP startup or tools/call.
	managed := projectKimiMCPServers(t, filepath.Join(binding.TargetLocator, "mcp.json"))
	native := projectKimiMCPServers(t, filepath.Join(project, ".kimi-code", "mcp.json"))
	if len(managed) != 1 || managed["docs"] == nil || native["docs"] == nil || !reflect.DeepEqual(managed["docs"], native["docs"]) {
		t.Fatalf("Kimi project MCP does not match the single owned source projection: managed=%v native=%v", managed, native["docs"])
	}
	server := native["docs"]
	if server["transport"] != "stdio" {
		t.Fatalf("Kimi project MCP transport is not native stdio: %v", server)
	}
	command, ok := server["command"].(string)
	if !ok || !filepath.IsAbs(command) {
		t.Fatalf("Kimi project MCP did not retain its absolute managed launcher: %v", server["command"])
	}
	relative, err := filepath.Rel(binding.TargetLocator, command)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatalf("Kimi project MCP launcher escaped selected package: %q (%v)", command, err)
	}
	if args, ok := server["args"].([]any); !ok || len(args) < 2 {
		t.Fatalf("Kimi project MCP did not retain source command arguments: %v", server["args"])
	}
	return installation, binding
}

func projectKimiMCPServers(t *testing.T, path string) map[string]map[string]any {
	t.Helper()
	var document struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(projectJourneyRead(t, path)), &document); err != nil {
		t.Fatalf("invalid Kimi MCP JSON at %s: %v", path, err)
	}
	if document.MCPServers == nil {
		t.Fatalf("Kimi MCP collection missing at %s", path)
	}
	return document.MCPServers
}
