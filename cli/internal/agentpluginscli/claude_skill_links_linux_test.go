//go:build linux

package agentpluginscli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestLinuxGroupedClaudeAddDoctorRemoveWithRealSkillLinks(t *testing.T) {
	t.Parallel()
	claude := fixtureClient(t, domain.ClientClaude)
	claude.ExecutablePath = "/test/bin/claude"
	root := filepath.Join(claude.ConfigRoot, "skills")
	seedRealClaudeHomeSkills(t, root)

	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), claude})
	attachClaudeListingHarness(t, &fixture, root)
	plugin := writeCLIPlugin(t)

	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "cursor,claude", "--dry-run")
	if err != nil {
		t.Fatalf("grouped dry-run rejected a realistic Claude skills directory: %v", err)
	}
	if !strings.Contains(stdout, "Targets: cursor,claude") || !strings.Contains(stdout, "No changes made (dry run).") {
		t.Fatalf("grouped dry-run output = %s", stdout)
	}

	stdout, _, err = fixture.execute(false, "add", plugin, "--target", "cursor,claude")
	if err != nil {
		t.Fatalf("grouped add rejected a realistic Claude skills directory: %v\n%s", err, stdout)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("grouped add state = %+v, %v; output=%s", state, loadErr, stdout)
	}

	doctor, _, err := fixture.execute(false, "doctor", "demo", "--format", "json")
	if err != nil {
		t.Fatalf("doctor after grouped add: %v\n%s", err, doctor)
	}
	if !strings.Contains(doctor, `"command":"doctor"`) {
		t.Fatalf("doctor output = %s", doctor)
	}

	removed, _, err := fixture.execute(false, "remove", "demo", "--target", "cursor,claude")
	if err != nil {
		t.Fatalf("remove after grouped add: %v\n%s", err, removed)
	}
	state, loadErr = fixture.store.Load()
	if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 {
		t.Fatalf("remove left client bindings = %+v, %v", state, loadErr)
	}
	if _, err := os.Lstat(filepath.Join(root, "stale-relative")); err != nil {
		t.Fatalf("remove deleted an unrelated dangling skill link: %v", err)
	}
}

func TestLinuxDoctorFailsClosedWhenManagedClaudePathBecomesDangling(t *testing.T) {
	t.Parallel()
	claude := fixtureClient(t, domain.ClientClaude)
	claude.ExecutablePath = "/test/bin/claude"
	root := filepath.Join(claude.ConfigRoot, "skills")
	seedRealClaudeHomeSkills(t, root)

	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), claude})
	attachClaudeListingHarness(t, &fixture, root)
	plugin := writeCLIPlugin(t)
	if out, _, err := fixture.execute(false, "add", plugin, "--target", "cursor,claude"); err != nil {
		t.Fatalf("grouped add: %v\n%s", err, out)
	}
	active := claudeTargetLocator(t, fixture)
	if err := os.RemoveAll(active); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "removed-managed-package"), active); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "doctor", "demo", "--format", "json")
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, stdout)
	}
	var output struct {
		Data struct {
			Findings []doctorFinding `json:"findings"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range output.Data.Findings {
		if finding.ClientID != string(domain.ClientClaude) {
			continue
		}
		switch finding.Code {
		case "managed_directory_changed", "managed_integrity_check_failed", "managed_target_mismatch", "native_projection_changed":
			found = true
		}
	}
	if !found {
		t.Fatalf("doctor did not diagnose dangling managed Claude path: %+v", output.Data.Findings)
	}
}

func TestLinuxGroupedClaudeDryRunRejectsCircularSkillLink(t *testing.T) {
	t.Parallel()
	claude := fixtureClient(t, domain.ClientClaude)
	claude.ExecutablePath = "/test/bin/claude"
	root := filepath.Join(claude.ConfigRoot, "skills")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("loop", filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), claude})
	fixture.app.Lifecycle.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{
		Stager: providerstest.NewStager(providers.Stager{}),
	})
	_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "cursor,claude", "--dry-run")
	if err == nil {
		t.Fatal("grouped dry-run accepted a circular Claude skill link")
	}
	if !strings.Contains(err.Error(), "no target was changed") {
		t.Fatalf("fail-closed error = %v", err)
	}
}

func TestLinuxGroupedAddPreparesKiroWithoutAbortingCursor(t *testing.T) {
	t.Parallel()
	kiro := fixtureClient(t, domain.ClientKiro)
	kiro.ExecutablePath = "/fixture/kiro-cli"
	cursor := fixtureClient(t, domain.ClientCursor)
	fixture := newCLIFixture(t, []domain.DetectedClient{kiro, cursor})
	fixture.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: &cliRunOnlyRunner{}})
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	out, _, err := fixture.execute(false, "add", plugin, "--target", "kiro,cursor")
	if err != nil {
		t.Fatalf("grouped add aborted every target: %v\n%s", err, out)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("state = %+v, %v; output=%s", state, loadErr, out)
	}
	var sawKiro, sawCursor bool
	for _, binding := range state.Installations[0].Clients {
		switch domain.ClientID(binding.ClientID) {
		case domain.ClientKiro:
			sawKiro = true
			if binding.InstallIntent != domain.InstallIntentPrepare {
				t.Fatalf("kiro intent = %q", binding.InstallIntent)
			}
		case domain.ClientCursor:
			sawCursor = true
			if binding.InstallIntent != "" {
				t.Fatalf("cursor intent changed: %q", binding.InstallIntent)
			}
		}
	}
	if !sawKiro || !sawCursor {
		t.Fatalf("missing grouped binding: %+v", state.Installations[0].Clients)
	}
}

func attachClaudeListingHarness(t *testing.T, fixture *cliFixture, skills string) {
	t.Helper()
	runner := &cliCommandRunner{run: func(int, legacyports.Command) legacyports.CommandResult {
		return claudeSkillsListing(skills)
	}}
	fixture.app.Lifecycle.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{
		Stager: providerstest.NewStager(providers.Stager{}),
		Runner: runner,
	})
	fixture.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
}

func claudeSkillsListing(skills string) legacyports.CommandResult {
	entries, err := os.ReadDir(skills)
	if err != nil {
		return legacyports.CommandResult{Stdout: []byte("[]")}
	}
	var items []string
	for _, entry := range entries {
		path := filepath.Join(skills, entry.Name())
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(path, ".claude-plugin", "plugin.json")); err != nil {
			continue
		}
		items = append(items, fmt.Sprintf(`{"id":"demo@skills-dir","version":"1.0.0","scope":"user","enabled":true,"installPath":%q,"mcpServers":{}}`, path))
	}
	body := "[]"
	if len(items) > 0 {
		body = "[" + strings.Join(items, ",") + "]"
	}
	return legacyports.CommandResult{Stdout: []byte(body)}
}

func claudeTargetLocator(t *testing.T, fixture cliFixture) string {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("state = %+v, %v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID == string(domain.ClientClaude) && binding.TargetLocator != "" {
			return binding.TargetLocator
		}
	}
	t.Fatalf("missing Claude target locator: %+v", state.Installations[0].Clients)
	return ""
}

func seedRealClaudeHomeSkills(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte("finder"), 0o600); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(root, "personal-notes")
	if err := os.MkdirAll(plain, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain, "SKILL.md"), []byte("# notes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "shared-review")
	if err := os.MkdirAll(linked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linked, "SKILL.md"), []byte("# linked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(linked, filepath.Join(root, "shared-review")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("removed-skill", filepath.Join(root, "stale-relative")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "removed-abs"), filepath.Join(root, "stale-absolute")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "queue.fifo"), 0o600); err != nil {
		t.Fatal(err)
	}
}
