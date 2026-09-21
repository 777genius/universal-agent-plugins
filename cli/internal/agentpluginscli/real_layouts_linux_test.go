//go:build linux

package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

func TestLinuxListAndReaddAfterGroupedClaudeInstall(t *testing.T) {
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
	listed, _, err := fixture.execute(false, "list", "--format", "json")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, listed)
	}
	if !strings.Contains(listed, `"name":"demo"`) || !strings.Contains(listed, `"command":"list"`) {
		t.Fatalf("list after grouped add = %s", listed)
	}
	if out, _, err := fixture.execute(false, "remove", "demo", "--target", "cursor,claude"); err != nil {
		t.Fatalf("remove: %v\n%s", err, out)
	}
	if out, _, err := fixture.execute(false, "add", plugin, "--target", "cursor,claude"); err != nil {
		t.Fatalf("re-add: %v\n%s", err, out)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("re-add state = %+v, %v", state, loadErr)
	}
}

func TestLinuxGroupedCursorClaudeKiroAdd(t *testing.T) {
	t.Parallel()
	claude := fixtureClient(t, domain.ClientClaude)
	claude.ExecutablePath = "/test/bin/claude"
	kiro := fixtureClient(t, domain.ClientKiro)
	kiro.ExecutablePath = "/fixture/kiro-cli"
	cursor := fixtureClient(t, domain.ClientCursor)
	root := filepath.Join(claude.ConfigRoot, "skills")
	seedRealClaudeHomeSkills(t, root)
	fixture := newCLIFixture(t, []domain.DetectedClient{cursor, claude, kiro})
	attachClaudeListingHarness(t, &fixture, root)
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	out, _, err := fixture.execute(false, "add", plugin, "--target", "cursor,claude,kiro")
	if err != nil {
		t.Fatalf("three-way grouped add: %v\n%s", err, out)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 3 {
		t.Fatalf("state = %+v, %v; output=%s", state, loadErr, out)
	}
	var sawKiro, sawCursor, sawClaude bool
	for _, binding := range state.Installations[0].Clients {
		switch domain.ClientID(binding.ClientID) {
		case domain.ClientKiro:
			sawKiro = true
			if binding.InstallIntent != domain.InstallIntentPrepare {
				t.Fatalf("kiro intent = %q", binding.InstallIntent)
			}
		case domain.ClientCursor:
			sawCursor = true
		case domain.ClientClaude:
			sawClaude = true
		}
	}
	if !sawKiro || !sawCursor || !sawClaude {
		t.Fatalf("missing grouped binding: %+v", state.Installations[0].Clients)
	}
}

func TestLinuxDoctorFailsClosedWhenManagedClaudePathBecomesFile(t *testing.T) {
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
	if err := os.WriteFile(active, []byte("replaced-with-file"), 0o600); err != nil {
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
		t.Fatalf("doctor did not diagnose file-replaced managed Claude path: %+v", output.Data.Findings)
	}
}

func TestLinuxGroupedClaudeDryRunRejectsFifoSkillsRoot(t *testing.T) {
	t.Parallel()
	claude := fixtureClient(t, domain.ClientClaude)
	claude.ExecutablePath = "/test/bin/claude"
	root := filepath.Join(claude.ConfigRoot, "skills")
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(root, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), claude})
	fixture.app.Lifecycle.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{
		Stager: providerstest.NewStager(providers.Stager{}),
	})
	type outcome struct {
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "cursor,claude", "--dry-run")
		done <- outcome{err}
	}()
	select {
	case result := <-done:
		if result.err == nil {
			t.Fatal("grouped dry-run accepted a FIFO Claude skills root")
		}
		if !strings.Contains(result.err.Error(), "no target was changed") {
			t.Fatalf("fail-closed error = %v", result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("grouped dry-run hung on a FIFO Claude skills root")
	}
}
