package providers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// TestInstallSlotMatrix pins the discussed Claude vs Codex installer cases in
// one place so later agents do not re-unify the two slots.
func TestInstallSlotMatrix(t *testing.T) {
	t.Parallel()
	t.Run("claude_target_is_skills_dir", testClaudeTargetIsSkillsDir)
	t.Run("claude_prepared_identity", testClaudePreparedIdentityMatrix)
	t.Run("claude_list_identity", testClaudeListIdentityMatrix)
	t.Run("claude_activate_is_list_only", testClaudeActivateIsListOnly)
	t.Run("claude_projection", testClaudeProjectionMatrix)
	t.Run("codex_target_is_managed_marketplace", testCodexTargetIsManagedMarketplace)
	t.Run("codex_prepared_identity", testCodexPreparedIdentityMatrix)
	t.Run("codex_list_identity", testCodexListIdentityMatrix)
	t.Run("codex_activate_is_marketplace_add", testCodexActivateIsMarketplaceAdd)
	t.Run("codex_projection", testCodexProjectionMatrix)
}

func testClaudeTargetIsSkillsDir(t *testing.T) {
	t.Helper()
	config := filepath.Join(t.TempDir(), ".claude")
	managed := filepath.Join(t.TempDir(), "managed")
	anchor, root, err := (&claude.Adapter{}).TargetRoot(
		domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: config},
		domain.PackageProjection,
		managed,
	)
	if err != nil {
		t.Fatal(err)
	}
	if anchor != config || root != filepath.Join(config, "skills") {
		t.Fatalf("Claude target = %q %q", anchor, root)
	}
}

func testCodexTargetIsManagedMarketplace(t *testing.T) {
	t.Helper()
	managed := filepath.Join(t.TempDir(), "managed")
	anchor, root, err := shared.ManagedTargetRoot(
		domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: filepath.Join(t.TempDir(), ".codex")},
		domain.PackageProjection,
		managed,
	)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(managed, "clients", string(domain.ClientCodex))
	if anchor != managed || root != want {
		t.Fatalf("Codex target = %q %q, want %q %q", anchor, root, managed, want)
	}
	if strings.Contains(root, string(filepath.Separator)+"skills") {
		t.Fatalf("Codex target leaked into skills: %q", root)
	}
}

func testClaudePreparedIdentityMatrix(t *testing.T) {
	t.Helper()
	cases := []struct {
		name string
		seed func(t *testing.T, root, active string)
		want domain.NativeIdentityState
	}{
		{"clean owned plugin", nil, domain.NativeIdentityManaged},
		{"empty dir", func(t *testing.T, root, _ string) {
			if err := os.Mkdir(filepath.Join(root, "empty"), 0o700); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{".DS_Store", func(t *testing.T, root, _ string) {
			if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte{0}, 0o600); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"plain skill", func(t *testing.T, root, _ string) {
			dir := filepath.Join(root, "social-autoposter")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# leftover\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"dangling symlink", func(t *testing.T, root, _ string) {
			if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "ccc")); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"other name plugin", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "neighbor", ".claude-plugin", "plugin.json"), `{"name":"neighbor"}`)
		}, domain.NativeIdentityManaged},
		{"malformed foreign plugin.json", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "broken", ".claude-plugin", "plugin.json"), `{"name":`)
		}, domain.NativeIdentityManaged},
		{"same name plugin", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "foreign", ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityUnmanaged},
		{"directory symlink to owned plugin", func(t *testing.T, root, active string) {
			if err := os.Symlink(active, filepath.Join(root, "alias-demo")); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityUnmanaged},
		{"staging dir leaked into skills", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, ".agentplugins-staging-deadbeef", ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityUnmanaged},
		{"damaged owned plugin.json", func(t *testing.T, _, active string) {
			writeIdentityFile(t, filepath.Join(active, ".claude-plugin", "plugin.json"), `{"name":`)
		}, domain.NativeIdentityIndeterminate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, plan, observer, managed := ownedClaudeSkills(t)
			if tc.seed != nil {
				tc.seed(t, root, plan.ActivePath)
			}
			if got := observePrepared(t, observer, domain.ClientClaude, plan, managed); got != tc.want {
				t.Fatalf("state=%s want=%s", got, tc.want)
			}
		})
	}
}

func testCodexPreparedIdentityMatrix(t *testing.T) {
	t.Helper()
	cases := []struct {
		name string
		seed func(t *testing.T, root, active string)
		want domain.NativeIdentityState
	}{
		{"clean owned plugin", nil, domain.NativeIdentityManaged},
		{"empty dir", func(t *testing.T, root, _ string) {
			if err := os.Mkdir(filepath.Join(root, "empty"), 0o700); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"dangling symlink", func(t *testing.T, root, _ string) {
			if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "ccc")); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"foreign marketplace other namespace", func(t *testing.T, root, active string) {
			writeIdentityFile(t, filepath.Join(root, "foreign-market", ".agents", "plugins", "marketplace.json"),
				`{"name":"foreign-market","plugins":[{"name":"demo"}]}`)
			_ = active
		}, domain.NativeIdentityManaged},
		{"staging prefix skipped", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, ".agentplugins-staging-deadbeef", ".codex-plugin", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityManaged},
		{"same name unqualified plugin", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "foreign", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityUnmanaged},
		{"damaged owned plugin.json", func(t *testing.T, _, active string) {
			writeIdentityFile(t, filepath.Join(active, ".codex-plugin", "plugin.json"), `{"name":`)
		}, domain.NativeIdentityIndeterminate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, plan, observer, managed := ownedCodexPlugins(t)
			if tc.seed != nil {
				tc.seed(t, root, plan.ActivePath)
			}
			if got := observePrepared(t, observer, domain.ClientCodex, plan, managed); got != tc.want {
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

func testCodexListIdentityMatrix(t *testing.T) {
	t.Helper()
	marketplace := "agentplugins-8f97b00da374"
	entry := func(pluginID, name, market string, extra string) string {
		body := fmt.Sprintf(`{"pluginId":%q,"name":%q,"marketplaceName":%q,"installed":true,"enabled":true`, pluginID, name, market)
		if extra != "" {
			body += "," + extra
		}
		return body + "}"
	}
	cases := map[string]struct {
		body string
		want codexStatus
	}{
		"name@marketplace":        {`{"installed":[` + entry("demo@"+marketplace, "demo", marketplace, "") + `]}`, codexStatusInstalled},
		"source.path is additive": {`{"installed":[` + entry("demo@"+marketplace, "demo", marketplace, `"source":{"path":"/managed/source"}`) + `]}`, codexStatusInstalled},
		"skills-dir leftover":     {`{"installed":[` + entry("demo@skills-dir", "demo", "skills-dir", "") + `]}`, codexStatusAbsent},
		"other marketplace":       {`{"installed":[` + entry("demo@other", "demo", "other", "") + `]}`, codexStatusAbsent},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := codexPluginStatus([]byte(tc.body), "demo", marketplace); got != tc.want {
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

func testClaudeProjectionMatrix(t *testing.T) {
	t.Helper()
	plan := stagingPlan(t, domain.ClientClaude, domain.PackageProjection)
	delivery, err := (Stager{}).Stage(context.Background(), stagingEnvelope(t), plan, "matrix-claude", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(delivery.StagingPath) != plan.TargetAnchor {
		t.Fatalf("Claude staging leaked into skills: %q", delivery.StagingPath)
	}
	if _, err := os.Stat(filepath.Join(delivery.StagingPath, ".claude-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Join(delivery.StagingPath, ".claude-plugin", "marketplace.json"))
	assertMissing(t, filepath.Join(delivery.StagingPath, ".agents", "plugins", "marketplace.json"))
}

func testCodexProjectionMatrix(t *testing.T) {
	t.Helper()
	plan := stagingPlan(t, domain.ClientCodex, domain.PackageProjection)
	delivery, err := (Stager{}).Stage(context.Background(), stagingEnvelope(t), plan, "matrix-codex", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(delivery.StagingPath) != plan.TargetRoot {
		t.Fatalf("Codex staging = %q, want under %q", delivery.StagingPath, plan.TargetRoot)
	}
	marketplace := readObject(t, filepath.Join(delivery.StagingPath, ".agents", "plugins", "marketplace.json"))
	if marketplace["name"] != shared.ManagedMarketplaceName(plan.PhysicalArtifactID) {
		t.Fatalf("Codex marketplace = %+v", marketplace)
	}
}

func ownedClaudeSkills(t *testing.T) (string, domain.DeliveryPlan, NativeIdentityObserver, *domain.ClientBinding) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "skills")
	plan := identityPlan(root)
	if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeIdentityFile(t, filepath.Join(plan.ActivePath, ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
	return root, plan, NativeIdentityObserver{Stager: acceptingPackageVerifier{}}, managedIdentityBinding()
}

func ownedCodexPlugins(t *testing.T) (string, domain.DeliveryPlan, NativeIdentityObserver, *domain.ClientBinding) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "plugins")
	plan := identityPlan(root)
	if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeIdentityFile(t, filepath.Join(plan.ActivePath, ".codex-plugin", "plugin.json"), `{"name":"demo"}`)
	return root, plan, NativeIdentityObserver{Stager: acceptingPackageVerifier{}}, managedIdentityBinding()
}

func managedIdentityBinding() *domain.ClientBinding {
	return &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:owned"}}}
}

func observePrepared(t *testing.T, observer NativeIdentityObserver, client domain.ClientID, plan domain.DeliveryPlan, managed *domain.ClientBinding) domain.NativeIdentityState {
	t.Helper()
	observation, err := observer.ObservePreparedIdentity(context.Background(), domain.DetectedClient{ClientID: client}, plan, managed)
	if err != nil && observation.State != domain.NativeIdentityIndeterminate {
		t.Fatalf("observe: %v", err)
	}
	return observation.State
}
