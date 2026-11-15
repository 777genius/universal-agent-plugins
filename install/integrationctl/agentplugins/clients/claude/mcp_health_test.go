package claude

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type claudeMCPTestRunner struct {
	t      *testing.T
	status string
	calls  int
}

func (r *claudeMCPTestRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	if reflect.DeepEqual(command.Argv, []string{"/trusted/claude", "plugin", "list", "--json"}) {
		return legacyports.CommandResult{Stdout: []byte(`[{"id":"demo@skills-dir","enabled":true}]`), NoChildStarted: true}, nil
	}
	if !reflect.DeepEqual(command.Argv, []string{"/trusted/claude", "mcp", "get", "plugin:demo:local"}) {
		r.t.Fatalf("wrong native MCP command: %v", command.Argv)
	}
	body := "plugin:demo:local:\n  Scope: Dynamic config (from command line)\n  Status: " + r.status + "\n  Type: stdio\n  Command: stdio\n"
	return legacyports.CommandResult{Stdout: []byte(body), NoChildStarted: true}, nil
}

func TestClaudeMCPActivationApprovesOnlyConfirmedSelectionAndPreservesRejection(t *testing.T) {
	for _, mode := range []string{"confirmed", "verify-only", "rejected"} {
		t.Run(mode, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			home, config := filepath.Join(base, "home"), filepath.Join(base, "config")
			for _, dir := range []string{home, config} {
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			path := filepath.Join(config, "settings.json")
			original := []byte(`{"disabledMcpjsonServers":["plugin:demo:local"],"permissions":{"deny":["Bash"]}}`)
			status := "✔ Connected"
			if mode == "verify-only" {
				status = "⏸ Pending approval (run `claude` to approve)"
			}
			if mode == "rejected" {
				status = "✘ Rejected (see disabledMcpjsonServers in settings)"
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
			}
			runner := &claudeMCPTestRunner{t: t, status: status}
			request := domain.ActivationRequest{VerifyOnly: mode == "verify-only", BackendExecutable: "/trusted/claude", DeclaredName: "demo", Client: domain.DetectedClient{ConfigRoot: config}, Plan: domain.DeliveryPlan{Warnings: []string{"authentication_not_catalog_verified"}, Scope: domain.ScopeUser, TargetContext: domain.TargetContext{Scope: domain.ScopeUser}, TargetAnchor: config, TargetRoot: filepath.Join(config, "skills"), ActivePath: filepath.Join(config, "skills", "demo"), Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportProjected}}}, Delivery: domain.StagedDelivery{ArtifactDigest: "sha256:verified"}}
			initial := domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotChecked}
			outcome, err := finishClaudeMCPActivation(t.Context(), clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}, request, initial)
			if err != nil {
				t.Fatal(err)
			}
			if runner.calls != 2 {
				t.Fatalf("native health not observed once: %d", runner.calls)
			}
			body, readErr := os.ReadFile(path)
			switch mode {
			case "confirmed":
				if readErr != nil || outcome.Activation != domain.ActivationActive {
					t.Fatalf("confirmed setup failed: %+v %v", outcome, readErr)
				}
			case "verify-only":
				if !os.IsNotExist(readErr) || outcome.Activation != domain.ActivationPrepared {
					t.Fatalf("verification changed approval: %+v %v", outcome, readErr)
				}
			case "rejected":
				if string(body) != string(original) || outcome.Activation != domain.ActivationPrepared || len(outcome.UserActions) == 0 {
					t.Fatalf("explicit policy rejection was overwritten: %+v %s", outcome, body)
				}
			}
			wantAuth := domain.AuthenticationNotChecked
			if mode == "confirmed" {
				wantAuth = domain.AuthenticationNotRequired
			}
			if outcome.Authentication != wantAuth || outcome.Policy != domain.PolicyAllowed {
				t.Fatalf("MCP health invented account or policy evidence: %+v", outcome)
			}
		})
	}
}
