package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type skillObservationRunner func(context.Context, legacyports.Command) (legacyports.CommandResult, error)

func (f skillObservationRunner) Run(ctx context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	return f(ctx, c)
}

// A denied or shadowed skill still appears in OpenCode's catalog. Neither
// observation may upgrade installed bytes into an active selected skill.
func TestProjectSkillObservationRequiresExactUsableCatalog(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".opencode", "skills", "probe", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: probe\ndescription: probe\n---\nExpected body\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, config, location, content string
		active                          bool
		mixed                           bool
		alias                           bool
	}{
		{name: "exact", config: `{"agent":{},"mode":{}}`, location: path, content: "Expected body\n", active: true},
		{name: "denied", config: `{"permission":{"skill":{"*":"deny"}}}`, location: path, content: "Expected body\n"},
		{name: "agent denied", config: `{"agent":{"build":{"permission":{"skill":"deny"}}}}`, location: path, content: "Expected body\n"},
		{name: "shadowed", config: `{}`, location: filepath.Join(root, ".agents", "skills", "probe", "SKILL.md"), content: "Expected body\n"},
		{name: "wrong content", config: `{}`, location: path, content: "Other body"},
		{name: "unrecognized config", config: `[]`, location: path, content: "Expected body\n"},
		{name: "known alias", config: `{}`, location: path, content: "Expected body\n", alias: true},
		{name: "mixed MCP", config: `{}`, location: path, content: "Expected body\n", mixed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.alias {
				alias := filepath.Join(root, ".agents", "skills", "probe", "SKILL.md")
				if err := os.MkdirAll(filepath.Dir(alias), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(alias, []byte("shadow"), 0600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Remove(alias); err != nil {
						t.Errorf("remove test alias: %v", err)
					}
				})
			}
			components := []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "probe"}}
			if tc.mixed {
				components = append(components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "server"})
			}
			request := domain.ActivationRequest{Client: domain.DetectedClient{ExecutablePath: "/TEST/opencode", ConfigRoot: filepath.Join(root, "user-opencode")}, Plan: domain.DeliveryPlan{Scope: domain.ScopeProject, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: root}, NativeRegistryRoot: root, Components: components}}
			runner := skillObservationRunner(func(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
				if c.Dir != root || len(c.Argv) != 3 || c.Argv[0] != "/TEST/opencode" || c.Argv[1] != "debug" || c.StdoutLimitBytes == 0 {
					t.Fatalf("unbounded or misplaced native probe: %+v", c)
				}
				if c.Argv[2] == "config" {
					return legacyports.CommandResult{Stdout: []byte(tc.config)}, nil
				}
				body, _ := json.Marshal([]openCodeObservedSkill{{Name: "probe", Location: tc.location, Content: tc.content}})
				return legacyports.CommandResult{Stdout: body}, nil
			})
			outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, UserActions: []string{"restart OpenCode to load the installed plugin", "preserve cleanup warning"}}
			actual, err := observeOpenCodeNativeComponents(t.Context(), clients.Env{Runner: runner}, request, outcome)
			if err != nil {
				t.Fatal(err)
			}
			if (actual.Activation == domain.ActivationActive) != tc.active {
				t.Fatalf("activation=%s, want active=%v", actual.Activation, tc.active)
			}
			if tc.active && (len(actual.UserActions) != 1 || actual.UserActions[0] != "preserve cleanup warning") {
				t.Fatalf("lost unrelated warnings: %v", actual.UserActions)
			}

		})
	}
}

// Exact file receipts do not establish native readiness when discovery cannot run.
func TestNativeObservationWithoutClientRemainsPrepared(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		request := domain.ActivationRequest{Plan: domain.DeliveryPlan{Scope: scope, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "probe"}}}}
		outcome, err := observeOpenCodeNativeComponents(t.Context(), clients.Env{}, request, domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled})
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled {
			t.Fatalf("scope=%s: files claimed readiness: %+v", scope, outcome)
		}
	}
}
