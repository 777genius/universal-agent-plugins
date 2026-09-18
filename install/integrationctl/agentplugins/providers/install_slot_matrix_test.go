package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cursor"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// TestInstallSlotMatrix pins the discussed Claude vs Codex installer cases in
// one place so later agents do not re-unify the two slots.
func TestInstallSlotMatrix(t *testing.T) {
	t.Parallel()
	t.Run("claude_target_is_skills_dir", testClaudeTargetIsSkillsDir)
	t.Run("claude_prepared_identity", testClaudePreparedIdentityMatrix)
	t.Run("claude_prepared_occupancy", testClaudePreparedOccupancyMatrix)
	t.Run("claude_prepared_path_alias", testClaudePreparedPathAlias)
	t.Run("claude_list_identity", testClaudeListIdentityMatrix)
	t.Run("claude_list_path_alias", testClaudeListPathAlias)
	t.Run("claude_native_ignores_marketplace_leftover", testClaudeNativeIgnoresMarketplaceLeftover)
	t.Run("claude_activate_is_list_only", testClaudeActivateIsListOnly)
	t.Run("claude_activate_failures", testClaudeActivateFailures)
	t.Run("claude_projection", testClaudeProjectionMatrix)
	t.Run("codex_target_is_managed_marketplace", testCodexTargetIsManagedMarketplace)
	t.Run("codex_prepared_identity", testCodexPreparedIdentityMatrix)
	t.Run("codex_prepared_occupancy", testCodexPreparedOccupancyMatrix)
	t.Run("codex_list_identity", testCodexListIdentityMatrix)
	t.Run("codex_activate_is_marketplace_add", testCodexActivateIsMarketplaceAdd)
	t.Run("codex_verify_only_is_list_only", testCodexVerifyOnlyIsListOnly)
	t.Run("codex_activate_failures", testCodexActivateFailures)
	t.Run("codex_projection", testCodexProjectionMatrix)
	t.Run("remaining_targets", testRemainingClientTargets)
	t.Run("vscode_uses_copilot_registry", testVSCodeUsesCopilotRegistry)
	t.Run("remaining_activate", testRemainingClientActivateMatrix)
	t.Run("copilot_list_has_no_json_flag", testCopilotListHasNoJSONFlag)
	t.Run("gemini_skills_not_extensions", testGeminiSkillsSlotIgnoresExtensions)
	t.Run("chatgpt_remote_registry_is_indeterminate", testChatGPTRemoteRegistryIndeterminate)
	t.Run("cline_opencode_defer_native_occupancy", testClineOpenCodeDeferNativeOccupancy)
	t.Run("windsurf_mcp_active_keeps_skills_prepared", testWindsurfMCPActiveKeepsSkillsPrepared)
	t.Run("copilot_live_path_is_lexical", testCopilotLivePathIsLexical)
	t.Run("kiro_verifier_rejects_chat_binary", testKiroVerifierRejectsChatBinary)
	t.Run("remaining_projection", testRemainingClientProjectionMatrix)
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
		{"file symlink", func(t *testing.T, root, _ string) {
			target := filepath.Join(root, "note.txt")
			if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, "alias.txt")); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"plain file", func(t *testing.T, root, _ string) {
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"other name plugin", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "neighbor", ".claude-plugin", "plugin.json"), `{"name":"neighbor"}`)
		}, domain.NativeIdentityManaged},
		{"root plugin.json is not a skills-dir claim", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "foreign-root", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityManaged},
		{"nested plugin is not scanned", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "neighbor", "nested", ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
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

func testClaudePreparedOccupancyMatrix(t *testing.T) {
	t.Helper()
	t.Run("missing ActivePath is Absent", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "skills")
		plan := identityPlan(root)
		observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}}
		if got := observePrepared(t, observer, domain.ClientClaude, plan, managedIdentityBinding()); got != domain.NativeIdentityAbsent {
			t.Fatalf("state=%s", got)
		}
	})
	t.Run("unowned occupancy is Unmanaged", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "skills")
		plan := identityPlan(root)
		if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
			t.Fatal(err)
		}
		observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}}
		if got := observePrepared(t, observer, domain.ClientClaude, plan, nil); got != domain.NativeIdentityUnmanaged {
			t.Fatalf("state=%s", got)
		}
	})
	t.Run("digest mismatch is Indeterminate", func(t *testing.T) {
		_, plan, _, managed := ownedClaudeSkills(t)
		observer := NativeIdentityObserver{Stager: mismatchPackageVerifier{}}
		if got := observePrepared(t, observer, domain.ClientClaude, plan, managed); got != domain.NativeIdentityIndeterminate {
			t.Fatalf("state=%s", got)
		}
	})
	t.Run("owned plugin.json name drift stays digest-managed", func(t *testing.T) {
		_, plan, observer, managed := ownedClaudeSkills(t)
		writeIdentityFile(t, filepath.Join(plan.ActivePath, ".claude-plugin", "plugin.json"), `{"name":"other"}`)
		if got := observePrepared(t, observer, domain.ClientClaude, plan, managed); got != domain.NativeIdentityManaged {
			t.Fatalf("state=%s", got)
		}
	})
}

func testClaudePreparedPathAlias(t *testing.T) {
	t.Helper()
	_, plan, observer, managed := ownedClaudeSkills(t)
	resolved, err := filepath.EvalSymlinks(plan.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == plan.ActivePath {
		t.Skip("filesystem has no /var vs /private/var alias")
	}
	plan.ActivePath = resolved
	if got := observePrepared(t, observer, domain.ClientClaude, plan, managed); got != domain.NativeIdentityManaged {
		t.Fatalf("aliased ActivePath state=%s want managed", got)
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
		{".DS_Store", func(t *testing.T, root, _ string) {
			if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte{0}, 0o600); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"dangling symlink", func(t *testing.T, root, _ string) {
			if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "ccc")); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"plain skill neighbor", func(t *testing.T, root, _ string) {
			dir := filepath.Join(root, "social-autoposter")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# leftover\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, domain.NativeIdentityManaged},
		{"foreign marketplace other namespace", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "foreign-market", ".agents", "plugins", "marketplace.json"),
				`{"name":"foreign-market","plugins":[{"name":"demo"}]}`)
		}, domain.NativeIdentityManaged},
		{"nested plugin is not scanned", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "neighbor", "nested", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityManaged},
		{"staging prefix skipped", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, ".agentplugins-staging-deadbeef", ".codex-plugin", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityManaged},
		{"same name unqualified plugin", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "foreign", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityUnmanaged},
		{"same name .codex-plugin sibling", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "foreign", ".codex-plugin", "plugin.json"), `{"name":"demo"}`)
		}, domain.NativeIdentityUnmanaged},
		{"same name .claude-plugin sibling", func(t *testing.T, root, _ string) {
			writeIdentityFile(t, filepath.Join(root, "foreign", ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
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

func testCodexPreparedOccupancyMatrix(t *testing.T) {
	t.Helper()
	t.Run("missing ActivePath is Absent", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "plugins")
		plan := identityPlan(root)
		observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}}
		if got := observePrepared(t, observer, domain.ClientCodex, plan, managedIdentityBinding()); got != domain.NativeIdentityAbsent {
			t.Fatalf("state=%s", got)
		}
	})
	t.Run("unowned occupancy is Unmanaged", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "plugins")
		plan := identityPlan(root)
		if err := os.MkdirAll(plan.ActivePath, 0o700); err != nil {
			t.Fatal(err)
		}
		observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}}
		if got := observePrepared(t, observer, domain.ClientCodex, plan, nil); got != domain.NativeIdentityUnmanaged {
			t.Fatalf("state=%s", got)
		}
	})
	t.Run("digest mismatch is Indeterminate", func(t *testing.T) {
		_, plan, _, managed := ownedCodexPlugins(t)
		observer := NativeIdentityObserver{Stager: mismatchPackageVerifier{}}
		if got := observePrepared(t, observer, domain.ClientCodex, plan, managed); got != domain.NativeIdentityIndeterminate {
			t.Fatalf("state=%s", got)
		}
	})
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
		"trailing slash path":      {claudeListing("demo", managed+string(filepath.Separator), true), claudeStatusInstalled},
		"nested clean path":        {claudeListing("demo", filepath.Join(managed, ".", "."), true), claudeStatusInstalled},
		"disabled":                 {claudeListing("demo", managed, false), claudeStatusAbsent},
		"empty":                    {`[]`, claudeStatusAbsent},
		"skills-dir wrong path":    {claudeListing("demo", foreign, true), claudeStatusCollision},
		"project scope at path":    {fmt.Sprintf(`[{"id":"demo@skills-dir","scope":"project","enabled":true,"installPath":%q}]`, managed), claudeStatusCollision},
		"enabled marketplace leftover occupies the name": {fmt.Sprintf(`[{"id":"demo@some-marketplace","scope":"user","enabled":true,"installPath":%q}]`, managed), claudeStatusCollision},
		"disabled marketplace leftover is absent": {
			fmt.Sprintf(`[{"id":"demo@some-marketplace","scope":"user","enabled":false,"installPath":%q}]`, managed),
			claudeStatusAbsent,
		},
		"neighbor other name": {claudeListing("neighbor", managed, true), claudeStatusAbsent},
		"marketplace plus skills-dir": {
			fmt.Sprintf(`[{"id":"demo@some-marketplace","scope":"user","enabled":true,"installPath":%q},{"id":"demo@skills-dir","version":"1.0.0","scope":"user","enabled":true,"installPath":%q,"mcpServers":{}}]`, managed, managed),
			claudeStatusInstalled,
		},
		"marketplace winner plus folder skills-dir loser": {
			fmt.Sprintf(`[{"id":"demo@uap-r3-mktplace","scope":"user","enabled":true,"installPath":%q},{"id":"managed-r3@skills-dir","version":"unknown","scope":"user","enabled":false,"installPath":"","errors":["Not loaded — the name \"demo\" is already taken"]}]`, filepath.Join(filepath.Dir(filepath.Dir(managed)), "plugins", "cache", "demo")),
			claudeStatusCollision,
		},
		"two skills-dir paths": {
			fmt.Sprintf(`[{"id":"demo@skills-dir","scope":"user","enabled":true,"installPath":%q},{"id":"demo@skills-dir","scope":"user","enabled":true,"installPath":%q}]`, managed, foreign),
			claudeStatusCollision,
		},
		"relative installPath": {`[{"id":"demo@skills-dir","scope":"user","enabled":true,"installPath":"skills/managed"}]`, claudeStatusUnknown},
		"malformed":            {`[{"id":"demo@skills-dir"}]`, claudeStatusUnknown},
		"list --json --available object is unknown": {
			fmt.Sprintf(`{"installed":[{"id":"demo@skills-dir","scope":"user","enabled":true,"installPath":%q}],"available":[]}`, managed),
			claudeStatusUnknown,
		},
		"project marketplace same name occupies the name": {
			fmt.Sprintf(`[{"id":"demo@uap-r4-same","scope":"project","enabled":true,"installPath":%q,"projectPath":"/tmp/proj"},{"id":"user-demo@skills-dir","version":"unknown","scope":"user","enabled":false,"installPath":"","errors":["Not loaded — the name is already taken"]}]`, filepath.Join(filepath.Dir(filepath.Dir(managed)), "plugins", "cache", "demo")),
			claudeStatusCollision,
		},
		"empty plugin.json name failed load is skipped": {
			`[{"id":"empty@skills-dir","version":"unknown","scope":"user","enabled":false,"installPath":"","errors":["name cannot be empty"]}]`,
			claudeStatusAbsent,
		},
		"at-sign plugin.json name failed load is skipped": {
			`[{"id":"atsign@skills-dir","version":"unknown","scope":"user","enabled":false,"installPath":"","errors":["must not contain @"]}]`,
			claudeStatusAbsent,
		},
		"installedAt extra field is ignored": {
			fmt.Sprintf(`[{"id":"demo@skills-dir","scope":"user","enabled":true,"installPath":%q,"installedAt":"2026-09-18T12:00:00.000Z"}]`, managed),
			claudeStatusInstalled,
		},
		"same-name loser empty path": {
			fmt.Sprintf(`[{"id":"demo@skills-dir","scope":"user","enabled":true,"installPath":%q},{"id":"foreign-demo@skills-dir","version":"unknown","scope":"user","enabled":false,"installPath":"","errors":["Not loaded — same plugin name"]}]`, managed),
			claudeStatusInstalled,
		},
		"enabled winner keeps errors warnings": {
			fmt.Sprintf(`[{"id":"demo@skills-dir","scope":"user","enabled":true,"installPath":%q,"errors":["warning"]}]`, managed),
			claudeStatusInstalled,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := claudePluginStatus([]byte(tc.body), "demo", managed); got != tc.want {
				t.Fatalf("status=%d want=%d", got, tc.want)
			}
		})
	}
	t.Run("dotted plugin.json name", func(t *testing.T) {
		if got := claudePluginStatus([]byte(claudeListing("uap.r4.dot", managed, true)), "uap.r4.dot", managed); got != claudeStatusInstalled {
			t.Fatalf("status=%d", got)
		}
	})
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
		"name@marketplace":                    {`{"installed":[` + entry("demo@"+marketplace, "demo", marketplace, "") + `]}`, codexStatusInstalled},
		"source.path is additive":             {`{"installed":[` + entry("demo@"+marketplace, "demo", marketplace, `"source":{"path":"/managed/source"}`) + `]}`, codexStatusInstalled},
		"installedPath cache is additive":     {`{"installed":[` + entry("demo@"+marketplace, "demo", marketplace, `"installedPath":"/cache/demo/1.0.0"`) + `]}`, codexStatusInstalled},
		"--available extra array is additive": {`{"installed":[` + entry("demo@"+marketplace, "demo", marketplace, "") + `],"available":[]}`, codexStatusInstalled},
		"skills-dir leftover":                 {`{"installed":[` + entry("demo@skills-dir", "demo", "skills-dir", "") + `]}`, codexStatusAbsent},
		"other marketplace":                   {`{"installed":[` + entry("demo@other", "demo", "other", "") + `]}`, codexStatusAbsent},
		"empty":                               {`{"installed":[]}`, codexStatusAbsent},
		"disabled":                            {`{"installed":[{"pluginId":"demo@` + marketplace + `","name":"demo","marketplaceName":"` + marketplace + `","installed":true,"enabled":false}]}`, codexStatusAbsent},
		"not installed flag":                  {`{"installed":[{"pluginId":"demo@` + marketplace + `","name":"demo","marketplaceName":"` + marketplace + `","installed":false,"enabled":true}]}`, codexStatusAbsent},
		"inconsistent pluginId":               {`{"installed":[` + entry("demo@other", "demo", marketplace, "") + `]}`, codexStatusUnknown},
		"malformed":                           {`{`, codexStatusUnknown},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := codexPluginStatus([]byte(tc.body), "demo", marketplace); got != tc.want {
				t.Fatalf("status=%d want=%d", got, tc.want)
			}
		})
	}
	t.Run("dotted plugin name", func(t *testing.T) {
		body := `{"installed":[` + entry("uap.r4.dot@"+marketplace, "uap.r4.dot", marketplace, "") + `]}`
		if got := codexPluginStatus([]byte(body), "uap.r4.dot", marketplace); got != codexStatusInstalled {
			t.Fatalf("status=%d", got)
		}
	})
}

func testClaudeListPathAlias(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "other")
	if err := os.MkdirAll(other, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := claudePluginStatus([]byte(claudeListing("demo", real, true)), "demo", alias); got != claudeStatusInstalled {
		t.Fatalf("list real / active alias = %d", got)
	}
	if got := claudePluginStatus([]byte(claudeListing("demo", alias, true)), "demo", real); got != claudeStatusInstalled {
		t.Fatalf("list alias / active real = %d", got)
	}
	if got := claudePluginStatus([]byte(claudeListing("demo", other, true)), "demo", real); got != claudeStatusCollision {
		t.Fatalf("distinct dirs = %d", got)
	}
}

func testClaudeNativeIgnoresMarketplaceLeftover(t *testing.T) {
	t.Helper()
	_, plan, _, managed := ownedClaudeSkills(t)
	plan.NativeRegistryExecutable = "/test/bin/claude"
	plan.TargetAnchor = filepath.Dir(plan.TargetRoot)
	body := fmt.Sprintf(
		`[{"id":"demo@old-market","scope":"user","enabled":true,"installPath":%q},{"id":"demo@skills-dir","version":"1.0.0","scope":"user","enabled":true,"installPath":%q,"mcpServers":{}}]`,
		filepath.Join(plan.TargetAnchor, "plugins", "cache", "demo"),
		plan.ActivePath,
	)
	runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{Stdout: []byte(body)}
	}}
	observation, err := (NativeIdentityObserver{Stager: acceptingPackageVerifier{}, Runner: runner}).ObserveNativeIdentity(
		context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude}, plan, managed,
	)
	if err != nil || observation.State != domain.NativeIdentityManaged {
		t.Fatalf("observation=%+v err=%v", observation, err)
	}

	t.Run("enabled marketplace same name is unmanaged", func(t *testing.T) {
		blocked := fmt.Sprintf(
			`[{"id":"demo@old-market","scope":"user","enabled":true,"installPath":%q},{"id":%q,"version":"unknown","scope":"user","enabled":false,"installPath":"","errors":["Not loaded — the name is already taken"]}]`,
			filepath.Join(plan.TargetAnchor, "plugins", "cache", "demo"),
			filepath.Base(plan.ActivePath)+"@skills-dir",
		)
		blockedRunner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
			return legacyports.CommandResult{Stdout: []byte(blocked)}
		}}
		observation, err := (NativeIdentityObserver{Stager: acceptingPackageVerifier{}, Runner: blockedRunner}).ObserveNativeIdentity(
			context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude}, plan, managed,
		)
		if err != nil || observation.State != domain.NativeIdentityUnmanaged {
			t.Fatalf("marketplace occupancy observation=%+v err=%v", observation, err)
		}
	})
}

func testClaudeActivateIsListOnly(t *testing.T) {
	t.Helper()
	request, runner := claudeActivation(t, "")
	runner.run = func(legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{Stdout: []byte(claudeListing("demo", request.Plan.ActivePath, true))}
	}
	if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	got := commandArgv(runner.commands)
	if len(got) != 1 || !reflect.DeepEqual(got[0], []string{"/test/bin/claude", "plugin", "list", "--json"}) {
		t.Fatalf("claude argv = %#v", got)
	}
}

func testClaudeActivateFailures(t *testing.T) {
	t.Helper()
	t.Run("disabled", func(t *testing.T) {
		request, runner := claudeActivation(t, "")
		runner.run = func(legacyports.Command) legacyports.CommandResult {
			return legacyports.CommandResult{Stdout: []byte(claudeListing("demo", request.Plan.ActivePath, false))}
		}
		_, err := (Activator{Runner: runner}).Activate(context.Background(), request)
		if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("marketplace leftover", func(t *testing.T) {
		request, runner := claudeActivation(t, "")
		runner.run = func(legacyports.Command) legacyports.CommandResult {
			return legacyports.CommandResult{Stdout: []byte(fmt.Sprintf(`[{"id":"demo@some-marketplace","scope":"user","enabled":true,"installPath":%q}]`, request.Plan.ActivePath))}
		}
		_, err := (Activator{Runner: runner}).Activate(context.Background(), request)
		if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("wrong path", func(t *testing.T) {
		request, runner := claudeActivation(t, "")
		foreign := filepath.Join(t.TempDir(), "foreign")
		runner.run = func(legacyports.Command) legacyports.CommandResult {
			return legacyports.CommandResult{Stdout: []byte(claudeListing("demo", foreign, true))}
		}
		_, err := (Activator{Runner: runner}).Activate(context.Background(), request)
		if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("symlink ActivePath refused", func(t *testing.T) {
		request := activationRequest(t, domain.ClientClaude)
		config := filepath.Join(t.TempDir(), "claude-config")
		realDir := filepath.Join(config, "skills", "real")
		if err := os.MkdirAll(realDir, 0o700); err != nil {
			t.Fatal(err)
		}
		active := filepath.Join(config, "skills", "linked")
		if err := os.Symlink(realDir, active); err != nil {
			t.Fatal(err)
		}
		request.BackendExecutable = "/test/bin/claude"
		request.Client.ConfigRoot = config
		request.Plan.TargetAnchor = config
		request.Plan.TargetRoot = filepath.Join(config, "skills")
		request.Plan.ActivePath = active
		request.Delivery.ActivePath = active
		request.Delivery.OwnedBase = filepath.Dir(active)
		_, err := (Activator{Runner: &recordingRunner{}}).Activate(context.Background(), request)
		if err == nil || (!strings.Contains(err.Error(), "symlink") && !strings.Contains(err.Error(), "real directory")) {
			t.Fatalf("err=%v", err)
		}
	})
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

func testCodexVerifyOnlyIsListOnly(t *testing.T) {
	t.Helper()
	request := activationRequest(t, domain.ClientCodex)
	request.BackendExecutable = "/test/bin/codex"
	request.VerifyOnly = true
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{Stdout: []byte(fmt.Sprintf(`{"installed":[{"pluginId":"demo@%s","name":"demo","marketplaceName":%q,"installed":true,"enabled":true}]}`, marketplace, marketplace))}
	}}
	if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	got := commandArgv(runner.commands)
	if !reflect.DeepEqual(got, [][]string{{"/test/bin/codex", "plugin", "list", "--json"}}) {
		t.Fatalf("codex verify-only argv = %#v", got)
	}
}

func testCodexActivateFailures(t *testing.T) {
	t.Helper()
	t.Run("skills-dir leftover", func(t *testing.T) {
		request := activationRequest(t, domain.ClientCodex)
		request.BackendExecutable = "/test/bin/codex"
		runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
			if strings.Contains(strings.Join(command.Argv, " "), "plugin list --json") {
				return legacyports.CommandResult{Stdout: []byte(`{"installed":[{"pluginId":"demo@skills-dir","name":"demo","marketplaceName":"skills-dir","installed":true,"enabled":true}]}`)}
			}
			return legacyports.CommandResult{}
		}}
		_, err := (Activator{Runner: runner}).Activate(context.Background(), request)
		if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("disabled", func(t *testing.T) {
		request := activationRequest(t, domain.ClientCodex)
		request.BackendExecutable = "/test/bin/codex"
		marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
		runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
			if strings.Contains(strings.Join(command.Argv, " "), "plugin list --json") {
				return legacyports.CommandResult{Stdout: []byte(fmt.Sprintf(`{"installed":[{"pluginId":"demo@%s","name":"demo","marketplaceName":%q,"installed":true,"enabled":false}]}`, marketplace, marketplace))}
			}
			return legacyports.CommandResult{}
		}}
		_, err := (Activator{Runner: runner}).Activate(context.Background(), request)
		if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
			t.Fatalf("err=%v", err)
		}
	})
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

func testRemainingClientTargets(t *testing.T) {
	t.Helper()
	managed := filepath.Join(t.TempDir(), "managed")
	config := filepath.Join(t.TempDir(), "config")
	t.Run("cursor_target_is_plugins_local", func(t *testing.T) {
		anchor, root, err := (&cursor.Adapter{}).TargetRoot(
			domain.DetectedClient{ClientID: domain.ClientCursor, ConfigRoot: config},
			domain.PackageNative,
			managed,
		)
		if err != nil {
			t.Fatal(err)
		}
		if anchor != config || root != filepath.Join(config, "plugins", "local") {
			t.Fatalf("Cursor target = %q %q", anchor, root)
		}
		if strings.Contains(root, string(filepath.Separator)+"skills") || strings.Contains(root, "clients"+string(filepath.Separator)+"cursor") {
			t.Fatalf("Cursor target left the in-place local plugin slot: %q", root)
		}
	})
	for _, client := range []domain.ClientID{
		domain.ClientCopilot, domain.ClientVSCode, domain.ClientGemini, domain.ClientOpenCode,
		domain.ClientCline, domain.ClientWindsurf, domain.ClientKiro, domain.ClientChatGPT,
	} {
		client := client
		t.Run(string(client)+"_target_is_managed", func(t *testing.T) {
			anchor, root, err := shared.ManagedTargetRoot(
				domain.DetectedClient{ClientID: client, ConfigRoot: config},
				domain.PackageNative,
				managed,
			)
			if err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(managed, "clients", string(client))
			if anchor != managed || root != want {
				t.Fatalf("%s target = %q %q, want %q %q", client, anchor, root, managed, want)
			}
			if strings.Contains(root, string(filepath.Separator)+"skills") || strings.Contains(root, "plugins"+string(filepath.Separator)+"local") {
				t.Fatalf("%s target leaked into an in-place discovery dir: %q", client, root)
			}
		})
	}
	chatgpt, ok := domain.ClientDefinitionFor(domain.ClientChatGPT)
	if !ok || !chatgpt.PlansWithoutHostPresence {
		t.Fatalf("ChatGPT must plan without host presence: %+v", chatgpt)
	}
}

func testVSCodeUsesCopilotRegistry(t *testing.T) {
	t.Helper()
	copilotRoot := filepath.Join(t.TempDir(), ".copilot")
	vscodeRoot := filepath.Join(t.TempDir(), ".vscode")
	root, executable := (&vscode.Adapter{}).NativeRegistry(clients.PlanInput{
		Client: domain.DetectedClient{ClientID: domain.ClientVSCode, ConfigRoot: vscodeRoot, ExecutablePath: "/bin/code"},
		Detected: map[domain.ClientID]domain.DetectedClient{
			domain.ClientCopilot: {ClientID: domain.ClientCopilot, ConfigRoot: copilotRoot, ExecutablePath: "/bin/copilot"},
		},
	})
	if root != copilotRoot || executable != "/bin/copilot" {
		t.Fatalf("VS Code native registry = %q %q", root, executable)
	}
}

func testRemainingClientActivateMatrix(t *testing.T) {
	t.Helper()
	nativeSkill := []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "demo", Support: domain.SupportNative}}
	t.Run("cursor_and_chatgpt_are_manual_with_no_cli", func(t *testing.T) {
		for _, client := range []domain.ClientID{domain.ClientCursor, domain.ClientChatGPT} {
			client := client
			t.Run(string(client), func(t *testing.T) {
				runner := &recordingRunner{}
				request := activationRequest(t, client)
				request.BackendExecutable = "/test/bin/" + string(client)
				outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				if outcome.Activation != domain.ActivationManual {
					t.Fatalf("activation=%s", outcome.Activation)
				}
				if got := commandArgv(runner.commands); len(got) != 0 {
					t.Fatalf("argv = %#v", got)
				}
			})
		}
	})
	t.Run("native_config_clients_never_call_plugin_clis", func(t *testing.T) {
		for _, client := range []domain.ClientID{domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline} {
			client := client
			t.Run(string(client), func(t *testing.T) {
				runner := &recordingRunner{}
				request := activationRequest(t, client)
				request.VerifyOnly = true
				request.Plan.Components = nativeSkill
				request.BackendExecutable = "/test/bin/" + string(client)
				outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				if outcome.Activation != domain.ActivationActive {
					t.Fatalf("activation=%s", outcome.Activation)
				}
				got := commandArgv(runner.commands)
				if len(got) != 0 {
					t.Fatalf("argv = %#v", got)
				}
				for _, argv := range got {
					joined := strings.Join(argv, " ")
					if strings.Contains(joined, "plugin") || strings.Contains(joined, "extensions") || strings.Contains(joined, "skills") {
						t.Fatalf("native-config client invoked a competing CLI: %#v", argv)
					}
				}
			})
		}
	})
	t.Run("copilot_activate_is_marketplace_install_and_list", func(t *testing.T) {
		request := activationRequest(t, domain.ClientCopilot)
		request.BackendExecutable = "/test/bin/copilot"
		spec := "demo@" + shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
		runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
			if strings.HasSuffix(strings.Join(command.Argv, " "), "plugin list") {
				return legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • " + spec + " (v1.0.0)")}
			}
			return legacyports.CommandResult{}
		}}
		if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err != nil {
			t.Fatal(err)
		}
		got := commandArgv(runner.commands)
		want := [][]string{
			{"/test/bin/copilot", "plugin", "marketplace", "add", request.Delivery.ActivePath},
			{"/test/bin/copilot", "plugin", "install", spec},
			{"/test/bin/copilot", "plugin", "list"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("copilot argv = %#v, want %#v", got, want)
		}
		for _, argv := range got {
			joined := strings.Join(argv, " ")
			if strings.Contains(joined, "--json") || strings.Contains(joined, "marketplace upgrade") {
				t.Fatalf("copilot invoked a Codex/Claude flag: %#v", argv)
			}
		}
	})
	t.Run("vscode_activate_uses_copilot_backend_executable", func(t *testing.T) {
		request := activationRequest(t, domain.ClientVSCode)
		request.BackendExecutable = "/test/bin/copilot"
		spec := "demo@" + shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
		runner := &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
			if strings.HasSuffix(strings.Join(command.Argv, " "), "plugin list") {
				return legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • " + spec + " (v1.0.0)")}
			}
			return legacyports.CommandResult{}
		}}
		if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err != nil {
			t.Fatal(err)
		}
		got := commandArgv(runner.commands)
		if len(got) == 0 || got[0][0] != "/test/bin/copilot" {
			t.Fatalf("vscode argv = %#v", got)
		}
		for _, argv := range got {
			if argv[0] == "/bin/code" || strings.Contains(strings.Join(argv, " "), "--json") {
				t.Fatalf("VS Code used the code CLI or a JSON list flag: %#v", argv)
			}
		}
	})
	t.Run("kiro_skills_only_does_not_call_acp", func(t *testing.T) {
		runner := &recordingRunner{}
		request := activationRequest(t, domain.ClientKiro)
		request.VerifyOnly = true
		request.BackendExecutable = "/test/bin/kiro-cli"
		request.Plan.Components = nativeSkill
		if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err != nil {
			t.Fatal(err)
		}
		if got := commandArgv(runner.commands); len(got) != 0 {
			t.Fatalf("kiro skills argv = %#v", got)
		}
	})
	t.Run("kiro_mcp_verify_is_acp_v3", func(t *testing.T) {
		request := activationRequest(t, domain.ClientKiro)
		request.VerifyOnly = true
		request.BackendExecutable = "/test/bin/kiro-cli"
		request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "alpha", Support: domain.SupportNative}}
		runner := &recordingRunner{
			duplexOutput: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) +
				acpStatus("s", "alpha", "connected", `[{"name":"a","disabled":false}]`),
			duplexLive: true,
		}
		if _, err := (Activator{Runner: runner}).Activate(context.Background(), request); err != nil {
			t.Fatal(err)
		}
		got := commandArgv(runner.commands)
		want := [][]string{{"/test/bin/kiro-cli", "acp", "--agent-engine", "v3", "--auth-method", "cli"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("kiro argv = %#v, want %#v", got, want)
		}
		for _, argv := range got {
			joined := strings.Join(argv, " ")
			if strings.Contains(joined, "plugin") || strings.Contains(joined, "mcp add") {
				t.Fatalf("Kiro invoked a plugin/mcp CLI: %#v", argv)
			}
		}
	})
	if !isKiroCLI("/opt/Kiro CLI.app/Contents/MacOS/kiro-cli") || !isKiroCLI("kiro") || isKiroCLI("kiro-cli-chat") {
		t.Fatal("Kiro CLI identity drifted")
	}
}

func testCopilotListHasNoJSONFlag(t *testing.T) {
	t.Helper()
	plan := identityPlan(filepath.Join(t.TempDir(), "copilot"))
	plan.NativeRegistryExecutable = "/test/bin/copilot"
	runner := &identityRunner{result: legacyports.CommandResult{
		Stdout: []byte("No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin."),
	}}
	observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}, Runner: runner}
	observation, err := observer.ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCopilot}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityAbsent {
		t.Fatalf("observation=%+v err=%v", observation, err)
	}
	if len(runner.commands) != 1 || !reflect.DeepEqual(runner.commands[0], []string{"/test/bin/copilot", "plugin", "list"}) {
		t.Fatalf("copilot identity argv = %#v", runner.commands)
	}
}

func testGeminiSkillsSlotIgnoresExtensions(t *testing.T) {
	t.Helper()
	config := filepath.Join(t.TempDir(), ".gemini")
	plan := identityPlan(filepath.Join(t.TempDir(), "managed", "clients", "gemini"))
	plan.NativeRegistryRoot = config
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "demo", Support: domain.SupportNative}}
	writeIdentityFile(t, filepath.Join(config, "extensions", "demo", "gemini-extension.json"), `{"name":"demo","version":"0.0.1"}`)
	observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}}
	observation, err := observer.ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: config}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityAbsent {
		t.Fatalf("extension leftover occupied the skills slot: observation=%+v err=%v", observation, err)
	}

	if err := os.MkdirAll(filepath.Join(config, "skills", "demo"), 0o700); err != nil {
		t.Fatal(err)
	}
	observation, err = observer.ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: config}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("unowned Gemini skill occupancy = %+v err=%v", observation, err)
	}
}

func testChatGPTRemoteRegistryIndeterminate(t *testing.T) {
	t.Helper()
	plan := identityPlan(filepath.Join(t.TempDir(), "chatgpt"))
	observation, err := (NativeIdentityObserver{}).ObserveNativeIdentity(
		context.Background(), domain.DetectedClient{ClientID: domain.ClientChatGPT}, plan, nil,
	)
	if err != nil || observation.State != domain.NativeIdentityIndeterminate {
		t.Fatalf("ChatGPT without a local receipt must stay indeterminate: observation=%+v err=%v", observation, err)
	}
}

func testClineOpenCodeDeferNativeOccupancy(t *testing.T) {
	t.Helper()
	for _, client := range []domain.ClientID{domain.ClientCline, domain.ClientOpenCode} {
		client := client
		t.Run(string(client), func(t *testing.T) {
			config := filepath.Join(t.TempDir(), "."+string(client))
			writeIdentityFile(t, filepath.Join(config, "skills", "demo", "SKILL.md"), "---\nname: demo\ndescription: foreign\n---\n")
			plan := identityPlan(filepath.Join(t.TempDir(), "managed", "clients", string(client)))
			plan.NativeRegistryRoot = config
			plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "demo", Support: domain.SupportNative}}
			observation, err := (NativeIdentityObserver{}).ObserveNativeIdentity(
				context.Background(), domain.DetectedClient{ClientID: client, ConfigRoot: config}, plan, nil,
			)
			if err != nil || observation.State != domain.NativeIdentityAbsent {
				t.Fatalf("%s deferred occupancy = %+v err=%v", client, observation, err)
			}
		})
	}
}

func testWindsurfMCPActiveKeepsSkillsPrepared(t *testing.T) {
	t.Helper()
	configRoot := filepath.Join(t.TempDir(), ".codeium", "windsurf")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	delivery := stagedWindsurfDelivery(t, configRoot, "skills-caveat", "windsurf-skills")
	request := windsurfActivationRequest(delivery, configRoot, nil)
	request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "good", Support: domain.SupportPrepared})
	outcome, err := (Activator{}).Activate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationActive {
		t.Fatalf("activation=%s", outcome.Activation)
	}
	joined := strings.Join(outcome.UserActions, "\n")
	if !strings.Contains(joined, "not claimed as activated") {
		t.Fatalf("Windsurf MCP active overclaimed skills: %v", outcome.UserActions)
	}
}

func testCopilotLivePathIsLexical(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved == root {
		t.Skip("filesystem has no lexical alias")
	}
	spec := "demo@agentplugins-8f97b00da374"
	body := []byte(copilotLiveHeader + "\n  • " + spec + " (v1.0.0) (enabled)\n      from " + resolved + "\n")
	status, recognized := copilotLivePluginStatus(body, spec, "1.0.0", root)
	if !recognized || status != copilotStatusUnknown {
		t.Fatalf("Copilot live path used SameFile matching: status=%d recognized=%v", status, recognized)
	}
}

func testKiroVerifierRejectsChatBinary(t *testing.T) {
	t.Helper()
	client := domain.DetectedClient{ClientID: domain.ClientKiro}
	plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "demo", Support: domain.SupportNative}}}
	if !(Activator{}).VerifierAvailable(client, plan, "/test/bin/kiro-cli") || !(Activator{}).VerifierAvailable(client, plan, "/test/bin/kiro") {
		t.Fatal("trusted Kiro CLI was rejected as a verifier")
	}
	if (Activator{}).VerifierAvailable(client, plan, "/test/bin/kiro-cli-chat") {
		t.Fatal("kiro-cli-chat must not count as a Kiro plugin verifier")
	}
}

func testRemainingClientProjectionMatrix(t *testing.T) {
	t.Helper()
	t.Run("cursor_stages_beside_plugins_local", func(t *testing.T) {
		plan := stagingPlan(t, domain.ClientCursor, domain.PackageNative)
		delivery, err := (Stager{}).Stage(context.Background(), stagingEnvelope(t), plan, "matrix-cursor", domain.CompatibilityHints{})
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(delivery.StagingPath) == plan.TargetRoot {
			t.Fatalf("Cursor staging leaked into plugins/local: %q", delivery.StagingPath)
		}
		if filepath.Dir(delivery.StagingPath) != filepath.Dir(plan.TargetRoot) {
			t.Fatalf("Cursor staging = %q, want beside %q", delivery.StagingPath, plan.TargetRoot)
		}
		if _, err := os.Stat(filepath.Join(delivery.StagingPath, ".cursor-plugin", "plugin.json")); err != nil {
			t.Fatal(err)
		}
		assertMissing(t, filepath.Join(delivery.StagingPath, ".agents", "plugins", "marketplace.json"))
	})
	t.Run("copilot_marketplace_not_skills_dir", func(t *testing.T) {
		plan := stagingPlan(t, domain.ClientCopilot, domain.PackageNative)
		delivery, err := (Stager{}).Stage(context.Background(), stagingEnvelope(t), plan, "matrix-copilot", domain.CompatibilityHints{})
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(delivery.StagingPath) != plan.TargetRoot {
			t.Fatalf("Copilot staging = %q", delivery.StagingPath)
		}
		marketplace := readObject(t, filepath.Join(delivery.StagingPath, ".github", "plugin", "marketplace.json"))
		if marketplace["name"] != shared.ManagedMarketplaceName(plan.PhysicalArtifactID) {
			t.Fatalf("Copilot marketplace = %+v", marketplace)
		}
		assertMissing(t, filepath.Join(delivery.StagingPath, ".claude-plugin", "plugin.json"))
	})
	t.Run("chatgpt_marketplace_is_manual_prepared", func(t *testing.T) {
		plan := stagingPlan(t, domain.ClientChatGPT, domain.PackageProjection)
		delivery, err := (Stager{}).Stage(context.Background(), stagingEnvelope(t), plan, "matrix-chatgpt", domain.CompatibilityHints{})
		if err != nil {
			t.Fatal(err)
		}
		marketplace := readObject(t, filepath.Join(delivery.StagingPath, ".agents", "plugins", "marketplace.json"))
		if marketplace["name"] != shared.ManagedMarketplaceName(plan.PhysicalArtifactID) {
			t.Fatalf("ChatGPT marketplace = %+v", marketplace)
		}
		if _, err := os.Stat(filepath.Join(delivery.StagingPath, ".codex-plugin", "plugin.json")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("gemini_descriptor_not_extension_manifest", func(t *testing.T) {
		plan := stagingPlan(t, domain.ClientGemini, domain.PackageNative)
		plan.NativeRegistryRoot = filepath.Join(t.TempDir(), ".gemini")
		delivery, err := (Stager{}).Stage(context.Background(), stagingEnvelope(t), plan, "matrix-gemini", domain.CompatibilityHints{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(delivery.StagingPath, geminiDescriptorName)); err != nil {
			t.Fatal(err)
		}
		assertMissing(t, filepath.Join(delivery.StagingPath, "gemini-extension.json"))
		assertMissing(t, filepath.Join(delivery.StagingPath, ".agents", "plugins", "marketplace.json"))
	})
	t.Run("opencode_projection_not_npm_plugin", func(t *testing.T) {
		plan := stagingPlan(t, domain.ClientOpenCode, domain.PackageNative)
		plan.NativeRegistryRoot = filepath.Join(t.TempDir(), "opencode")
		delivery, err := (Stager{}).Stage(context.Background(), stagingEnvelope(t), plan, "matrix-opencode", domain.CompatibilityHints{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(delivery.StagingPath, openCodeProjectionFile)); err != nil {
			t.Fatal(err)
		}
		assertMissing(t, filepath.Join(delivery.StagingPath, ".agents", "plugins", "marketplace.json"))
	})
}

func claudeActivation(t *testing.T, _ string) (domain.ActivationRequest, *recordingRunner) {
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
	return request, &recordingRunner{}
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

type mismatchPackageVerifier struct{}

func (mismatchPackageVerifier) Verify(context.Context, string, string) error {
	return &ports.VerificationError{Kind: ports.VerificationDigestMismatch, ActualDigest: "sha256:other"}
}
