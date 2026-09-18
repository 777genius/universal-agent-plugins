package providers

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// TestInstallSlotMatrix pins the discussed Claude vs Codex installer cases in
// one place so later agents do not re-unify the two slots.
func TestInstallSlotMatrix(t *testing.T) {
	t.Parallel()
	t.Run("claude_prepared_identity", testClaudePreparedIdentityMatrix)
	t.Run("claude_list_identity", testClaudeListIdentityMatrix)
	t.Run("claude_activate_is_list_only", testClaudeActivateIsListOnly)
	t.Run("codex_activate_is_marketplace_add", testCodexActivateIsMarketplaceAdd)
}

func testClaudePreparedIdentityMatrix(t *testing.T) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "skills")
	plan := identityPlan(root)
	if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeIdentityFile(t, filepath.Join(plan.ActivePath, ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
	managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:owned"}}}
	observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}}
	observe := func() domain.NativeIdentityState {
		t.Helper()
		observation, err := observer.ObservePreparedIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude}, plan, managed)
		if err != nil {
			t.Fatalf("observe: %v", err)
		}
		return observation.State
	}
	if got := observe(); got != domain.NativeIdentityManaged {
		t.Fatalf("owned plugin = %s", got)
	}

	cases := []struct {
		name string
		seed func()
		want domain.NativeIdentityState
	}{
		{"empty dir", func() { _ = os.Mkdir(filepath.Join(root, "empty"), 0o700) }, domain.NativeIdentityManaged},
		{"plain skill", func() {
			_ = os.Mkdir(filepath.Join(root, "social-autoposter"), 0o700)
			_ = os.WriteFile(filepath.Join(root, "social-autoposter", "SKILL.md"), []byte("# leftover\n"), 0o600)
		}, domain.NativeIdentityManaged},
		{"dangling symlink", func() { _ = os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "ccc")) }, domain.NativeIdentityManaged},
		{"other name plugin", func() {
			writeIdentityFile(t, filepath.Join(root, "neighbor", ".claude-plugin", "plugin.json"), `{"name":"neighbor"}`)
		}, domain.NativeIdentityManaged},
		{"same name plugin", func() {
			writeIdentityFile(t, filepath.Join(root, "foreign", ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityUnmanaged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.seed()
			if got := observe(); got != tc.want {
				t.Fatalf("state=%s want=%s", got, tc.want)
			}
		})
	}
}

func testClaudeListIdentityMatrix(t *testing.T) {
	t.Helper()
	managed := filepath.Join(t.TempDir(), "claude-config", "skills", "managed")
	foreign := filepath.Join(t.TempDir(), "other", "skills", "foreign")
	cases := map[string]struct {
		body string
		want claudeStatus
	}{
		"skills-dir at ActivePath": {claudeListing("demo", managed, true), claudeStatusInstalled},
		"skills-dir wrong path":    {claudeListing("demo", foreign, true), claudeStatusCollision},
		"marketplace leftover":     {`[{"id":"demo@some-marketplace","scope":"user","enabled":true,"installPath":"` + managed + `"}]`, claudeStatusAbsent},
		"neighbor other name":      {claudeListing("neighbor", managed, true), claudeStatusAbsent},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := claudePluginStatus([]byte(tc.body), "demo", managed); got != tc.want {
				t.Fatalf("status=%d want=%d", got, tc.want)
			}
		})
	}
}

func testClaudeActivateIsListOnly(t *testing.T) {
	t.Helper()
	request := activationRequest(t, domain.ClientClaude)
	config := filepath.Join(t.TempDir(), "claude-config")
	active := filepath.Join(config, "skills", "demo-managed")
	if err := os.MkdirAll(active, 0o700); err != nil {
		t.Fatal(err)
	}
	request.BackendExecutable = "/test/bin/claude"
	request.Client.ConfigRoot = config
	request.Plan.TargetAnchor = config
	request.Plan.TargetRoot = filepath.Join(config, "skills")
	request.Plan.ActivePath = active
	request.Delivery.ActivePath = active
	request.Delivery.OwnedBase = filepath.Dir(active)
	runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{Stdout: []byte(claudeListing("demo", active, true))}
	}}
	if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	got := commandArgv(runner.commands)
	if len(got) != 1 || !reflect.DeepEqual(got[0], []string{"/test/bin/claude", "plugin", "list", "--json"}) {
		t.Fatalf("claude argv = %#v", got)
	}
	joined := strings.Join(got[0], " ")
	if strings.Contains(joined, "marketplace") || strings.Contains(joined, "plugin install") || strings.Contains(joined, "plugin add") {
		t.Fatalf("claude used marketplace CLI: %#v", got)
	}
}

func testCodexActivateIsMarketplaceAdd(t *testing.T) {
	t.Helper()
	runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		if strings.Contains(strings.Join(command.Argv, " "), "plugin list --json") {
			return legacyports.CommandResult{Stdout: []byte(`{"installed":[{"pluginId":"demo@agentplugins-8f97b00da374","name":"demo","marketplaceName":"agentplugins-8f97b00da374","installed":true,"enabled":true}],"available":[]}`)}
		}
		return legacyports.CommandResult{}
	}}
	request := activationRequest(t, domain.ClientCodex)
	request.BackendExecutable = "/test/bin/codex"
	if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	got := commandArgv(runner.commands)
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	want := [][]string{
		{"/test/bin/codex", "plugin", "marketplace", "add", request.Delivery.ActivePath, "--json"},
		{"/test/bin/codex", "plugin", "add", "demo@" + marketplace, "--json"},
		{"/test/bin/codex", "plugin", "list", "--json"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("codex argv = %#v, want %#v", got, want)
	}
	for _, argv := range got {
		joined := strings.Join(argv, " ")
		if strings.Contains(joined, "marketplace upgrade") || strings.Contains(joined, "plugin update") {
			t.Fatalf("codex used local upgrade: %#v", got)
		}
	}
}
