package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kimi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type projectKimiDetector struct{ user, executable string }

func (d projectKimiDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientKimi, ConfigRoot: d.user,
		Version: "2.5.2", ExecutablePath: d.executable, Status: domain.DetectionDetected}}, nil
}

func configureProjectKimiJourney(t *testing.T, app agentpluginscli.App, user, executable string) agentpluginscli.App {
	app.Detector = projectKimiDetector{user: user, executable: executable}
	mode, modePresent := os.LookupEnv("UAP_KIMI_GENUINE")
	selected, selectedPresent := os.LookupEnv("UAP_KIMI_STABLE_EXECUTABLE")
	genuine, err := projectKimiGenuineSelection(mode, selected, os.Getenv("GITHUB_ACTIONS"), os.Getenv("RUNNER_ENVIRONMENT"), modePresent || selectedPresent)
	if err != nil {
		t.Fatal(err)
	}
	if genuine {
		registry, err := clients.NewRegistry(kimi.New())
		if err != nil {
			t.Fatal(err)
		}
		detector := clientdetect.NewOS(filepath.Dir(user))
		detector.Registry = registry
		detector.Environment = map[string]string{"KIMI_CODE_HOME": user}
		detector.LookPath = func(name string) (string, error) {
			if name != "kimi" {
				return "", exec.ErrNotFound
			}
			return selected, nil
		}
		rows, err := detector.DetectTargetsWithVersionProbe(context.Background(), []domain.ClientID{domain.ClientKimi})
		if err != nil || len(rows) != 1 {
			t.Fatalf("genuine selected Kimi detection failed: %v (%v)", err, rows)
		}
		row := rows[0]
		if row.ClientID != domain.ClientKimi || row.Status != domain.DetectionDetected || row.ExecutablePath != selected || row.ConfigRoot != user || row.Version != "2.5.2" || row.DetectionError != nil {
			t.Fatalf("genuine selected Kimi identity/version differs: %+v", row)
		}
		// Assign the concrete detector after the shared composer; retain its targeted
		// probe interface instead of replacing it with a static observed version.
		app.Detector = detector
		if _, ok := app.Detector.(ports.TargetedVersionProbingClientDetector); !ok {
			t.Fatal("genuine detector lost targeted probing")
		}
		body, err := json.Marshal(map[string]string{"version": row.Version, "executable": selected, "config_root": user})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("KIMI_GENUINE_PROBE %s", body)
	}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectKimiJourney(t, selected, user, executable), nil
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
	projectKimiLogSource(t, installation, binding)
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

// Absence is the only inert mode. A partial opt-in must never pass as a fixture.
func projectKimiGenuineSelection(mode, executable, actions, environment string, present bool) (bool, error) {
	if !present {
		return false, nil
	}
	if mode != "1" || actions != "true" || environment != "github-hosted" || !filepath.IsAbs(executable) || filepath.Clean(executable) != executable || filepath.Base(executable) != "kimi" {
		return false, fmt.Errorf("genuine Kimi requires explicit hosted mode and absolute primary kimi executable")
	}
	// Official npm launchers may be symlinks; preserve the selected basename.
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return false, fmt.Errorf("genuine selected Kimi executable missing or nonregular")
	}
	return true, nil
}

func projectKimiLogSource(t *testing.T, installation domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	if os.Getenv("UAP_KIMI_GENUINE") != "1" {
		return
	}
	if installation.Source.SourceBindingID == "" || installation.Source.TreeDigest == "" || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatal("genuine preparation evidence lost source or conservative status")
	}
	body, err := json.Marshal(map[string]any{"source_binding_id": installation.Source.SourceBindingID, "tree_digest": installation.Source.TreeDigest, "source": installation.Source, "installation_id": installation.InstallationID, "client_binding_id": binding.ClientBindingID, "native_profile_root": binding.NativeProfileRoot, "activation": binding.Activation, "verification": binding.Verification})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("KIMI_PREPARED_SOURCE %s", body)
}

func TestProjectKimiGenuineSelectionRefusesPartialOrUnhostedOptIn(t *testing.T) {
	for _, row := range [][4]string{{"1", "", "true", "github-hosted"}, {"", "/missing/kimi", "true", "github-hosted"}, {"1", "/missing/kimi", "true", "github-hosted"}, {"1", "/missing/kimi", "true", "self-hosted"}, {"1", "relative/kimi", "true", "github-hosted"}, {"1", "/missing/entry.js", "true", "github-hosted"}} {
		if _, err := projectKimiGenuineSelection(row[0], row[1], row[2], row[3], true); err == nil {
			t.Fatal("invalid genuine selection accepted", row)
		}
	}
	if enabled, err := projectKimiGenuineSelection("", "", "", "", false); enabled || err != nil {
		t.Fatal("ordinary fixture mode changed")
	}
	for _, presence := range [][2]bool{{true, false}, {false, true}, {true, true}} {
		if _, err := projectKimiGenuineSelection("", "", "true", "github-hosted", presence[0] || presence[1]); err == nil {
			t.Fatal("present-empty genuine opt-in accepted", presence)
		}
	}
	target := filepath.Join(t.TempDir(), "kimi")
	if err := os.WriteFile(target, []byte("inert"), 0700); err != nil {
		t.Fatal(err)
	}
	if enabled, err := projectKimiGenuineSelection("1", target, "true", "github-hosted", true); !enabled || err != nil {
		t.Fatal("ordinary selected executable refused", err)
	}
	t.Run("official-npm-symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink privileges are not required by the ordinary Windows suite; genuine lanes are Linux/macOS")
		}
		launcher := filepath.Join(t.TempDir(), "kimi")
		if err := os.Symlink(target, launcher); err != nil {
			t.Fatal(err)
		}
		if enabled, err := projectKimiGenuineSelection("1", launcher, "true", "github-hosted", true); !enabled || err != nil {
			t.Fatal("official symlink launcher refused", err)
		}
	})
}
