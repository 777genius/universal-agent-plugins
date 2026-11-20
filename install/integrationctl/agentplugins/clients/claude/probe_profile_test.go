package claude

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A plain `claude` without CLAUDE_CONFIG_DIR reads project trust from
// HOME/.claude.json. Binding native commands to HOME/.claude would make them
// read HOME/.claude/.claude.json instead and report trust the user never sees.
func TestClaudeProfileMatchesTheUsersOwnGlobalConfig(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	defaultRoot, customRoot := filepath.Join(home, ".claude"), filepath.Join(home, "profile")
	for _, tc := range []struct {
		name, env, root, profile string
		bound                    bool
	}{
		{name: "default", root: defaultRoot, profile: filepath.Join(home, ".claude.json")},
		{name: "default beside another selected profile", env: customRoot, root: defaultRoot, profile: filepath.Join(home, ".claude.json")},
		{name: "explicit default folder", env: defaultRoot, root: defaultRoot, profile: filepath.Join(defaultRoot, ".claude.json"), bound: true},
		{name: "custom", env: customRoot, root: customRoot, profile: filepath.Join(customRoot, ".claude.json"), bound: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CONFIG_DIR", tc.env)
			command, err := ClaudeListCommand("claude", tc.root, filepath.Join(tc.root, "skills", "demo"))
			if err != nil {
				t.Fatal(err)
			}
			bound := slices.ContainsFunc(command.Env, func(entry string) bool { return strings.HasPrefix(entry, "CLAUDE_CONFIG_DIR=") })
			if bound != tc.bound || bound && !slices.Contains(command.Env, "CLAUDE_CONFIG_DIR="+tc.root) || !slices.Contains(command.Env, claudeProfileMarker+"="+tc.root) {
				t.Fatalf("native command environment = %v", command.Env)
			}
			profile, err := claudeGlobalConfigPath(tc.root)
			if err != nil || profile != tc.profile {
				t.Fatalf("global config = %q, %v; want %q", profile, err, tc.profile)
			}
		})
	}
}

// The default profile's trust lives in HOME/.claude.json, which a long-used
// Claude Code grows past the shared 1 MiB trust bound. Trust must be read and
// granted there instead of failing the install or using HOME/.claude.
func TestClaudeDefaultProfileTrustFitsALargeGlobalConfig(t *testing.T) {
	request := newClaudeMarketplaceRequest(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, ".claude")
	target, err := domain.NewTargetContext(domain.ScopeProject, request.Plan.TargetContext.ProjectRoot, root)
	if err != nil {
		t.Fatal(err)
	}
	request.Client.ConfigRoot, request.Plan.TargetContext = root, target
	profile := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(profile, []byte(`{"history":"`+strings.Repeat("x", 2<<20)+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	env := clients.Env{NativeConfig: nativeconfig.New()}
	if changed, err := grantClaudeProjectTrust(env, request); err != nil || !changed {
		t.Fatalf("grant in a large global config: changed=%v err=%v", changed, err)
	}
	if trusted, err := claudeProjectTrustRecorded(env, request); err != nil || !trusted {
		t.Fatalf("trust in a large global config: trusted=%v err=%v", trusted, err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude.json")); !os.IsNotExist(err) {
		t.Fatalf("default profile trust also reached the config folder: %v", err)
	}
}
