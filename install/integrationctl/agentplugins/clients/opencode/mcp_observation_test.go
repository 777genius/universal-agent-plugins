package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Native mcp list exits zero for disabled/failed servers. A same-name
// connected server from higher-priority config is also insufficient.
func TestLocalMCPObservationRequiresExactConnectedServer(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			root := t.TempDir()
			active := filepath.Join(root, "managed")
			if err := os.Mkdir(active, 0700); err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(root, "opencode.json")
			owned := `{"mcp":{"probe":{"type":"local","command":["/TEST/probe"],"environment":{"MARKER":"owned"}}}}`
			if err := os.WriteFile(configPath, []byte(owned), 0600); err != nil {
				t.Fatal(err)
			}
			projection := OpenCodeProjection{Version: 1, ConfigPath: configPath, ConfigJSON: configPath, ConfigJSONC: filepath.Join(root, "opencode.jsonc"), PackageRoot: active}
			body, _ := json.Marshal(projection)
			if err := os.WriteFile(filepath.Join(active, OpenCodeProjectionFile), body, 0600); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name, config, list string
				active             bool
				auth               domain.AuthenticationState
				catalog            bool
			}{
				{"connected", owned, "┌  MCP Servers\n│\n●  ✓ probe \x1b[90mconnected\n│\n└  1 server(s)\n", true, "", false},
				{"pending authentication", owned, "┌  MCP Servers\n●  ✓ probe connected\n└  1 server(s)\n", true, domain.AuthenticationPending, false},
				{"failed authentication", owned, "┌  MCP Servers\n●  ✓ probe connected\n└  1 server(s)\n", true, domain.AuthenticationFailed, false},
				{"authenticated", owned, "┌  MCP Servers\n●  ✓ probe connected\n└  1 server(s)\n", true, domain.AuthenticationComplete, false},
				{"catalog unchecked", owned, "┌  MCP Servers\n●  ✓ probe connected\n└  1 server(s)\n", true, domain.AuthenticationNotChecked, true},
				{"failed", owned, "┌  MCP Servers\n│\n●  ✗ probe failed\n└  1 server(s)\n", false, "", false},
				{"disabled", owned, "┌  MCP Servers\n│\n●  ○ probe disabled\n└  1 server(s)\n", false, "", false},
				{"foreign command", `{"mcp":{"probe":{"type":"local","command":["/FOREIGN/probe"],"environment":{"MARKER":"owned"}}}}`, "┌  MCP Servers\n●  ✓ probe connected\n└  1 server(s)\n", false, "", false},
				{"foreign env", `{"mcp":{"probe":{"type":"local","command":["/TEST/probe"],"environment":{"MARKER":"foreign"}}}}`, "┌  MCP Servers\n●  ✓ probe connected\n└  1 server(s)\n", false, "", false},
				{"denied", `{"permission":{"probe_*":"deny"},"mcp":{"probe":{"type":"local","command":["/TEST/probe"],"environment":{"MARKER":"owned"}}}}`, "┌  MCP Servers\n●  ✓ probe connected\n└  1 server(s)\n", false, "", false},
				{"unrecognized", owned, "probe connected", false, "", false},
				{"conflicting status", owned, "┌  MCP Servers\n●  ✓ probe connected\n●  ✗ probe failed\n└  2 server(s)\n", false, "", false},
				{"duplicate", owned, "┌  MCP Servers\n●  ✓ probe connected\n●  ✓ probe connected\n└  2 server(s)\n", false, "", false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					target := domain.TargetContext{Scope: scope, UserConfigRoot: root}
					if scope == domain.ScopeProject {
						target.ProjectRoot = root
					}
					auth := tc.auth
					if auth == "" {
						auth = domain.AuthenticationNotChecked
					}
					warnings := []string{}
					if !tc.catalog {
						warnings = append(warnings, "authentication_not_catalog_verified")
					}
					request := domain.ActivationRequest{Client: domain.DetectedClient{ExecutablePath: "/TEST/opencode", ConfigRoot: root}, Plan: domain.DeliveryPlan{Authentication: auth, Warnings: warnings, Scope: scope, TargetContext: target, NativeRegistryRoot: root, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "probe"}}}, Delivery: domain.StagedDelivery{ActivePath: active}}
					qualifyOpenCodeObservation(&request, "1.18.34")
					runner := skillObservationRunner(func(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
						if c.Dir != root || len(c.Argv) != 3 || c.StdoutLimitBytes == 0 {
							t.Fatalf("unbounded native probe: %+v", c)
						}
						if scope == domain.ScopeUser && !slices.Contains(c.Env, "OPENCODE_DISABLE_PROJECT_CONFIG=1") {
							t.Fatal("user observation may load unrelated project config")
						}
						if c.Argv[1] == "debug" && c.Argv[2] == "config" {
							return legacyports.CommandResult{Stdout: []byte(tc.config)}, nil
						}
						if c.Argv[1] != "mcp" || c.Argv[2] != "list" {
							t.Fatalf("unexpected native command: %v", c.Argv)
						}
						return legacyports.CommandResult{Stdout: []byte(tc.list)}, nil
					})
					actual, err := observeOpenCodeNativeComponents(t.Context(), clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}, request, domain.ActivationOutcome{Authentication: auth, Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled})
					if err != nil {
						t.Fatal(err)
					}
					if tc.active {
						wantAuth := auth
						if auth == domain.AuthenticationNotChecked && !tc.catalog {
							wantAuth = domain.AuthenticationNotRequired
						}
						if actual.Authentication != wantAuth {
							t.Fatalf("authentication=%s want %s", actual.Authentication, wantAuth)
						}
					}
					if (actual.Activation == domain.ActivationActive) != tc.active {
						t.Fatalf("activation=%s want active=%v", actual.Activation, tc.active)
					}
				})
			}
		})
	}
}
