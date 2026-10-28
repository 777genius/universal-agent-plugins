package all

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type removalPolicyCase struct {
	name        string
	client      domain.ClientID
	executable  bool
	runner      bool
	activation  domain.ActivationState
	objectKind  string
	missingRoot bool
	required    bool
}

// This covers every manual-uninstall branch reachable by currently admitted
// dependency clients. ChatGPT is manual-preparation-only; VS Code is frozen.
// The Copilot cases compare its new observation with the unchanged shared path.
func TestRemovalPoliciesMatchExistingDeactivationBranches(t *testing.T) {
	cases := []removalPolicyCase{
		{name: "codex active", client: domain.ClientCodex, executable: true, runner: true, activation: domain.ActivationActive, required: true},
		{name: "codex never activated", client: domain.ClientCodex, required: true},
		{name: "grok automatic", client: domain.ClientGrok, executable: true, runner: true},
		{name: "grok missing executable", client: domain.ClientGrok, runner: true, required: true},
		{name: "grok missing runner", client: domain.ClientGrok, executable: true, required: true},
		{name: "qoder automatic", client: domain.ClientQoder, executable: true, runner: true},
		{name: "qoder missing executable", client: domain.ClientQoder, runner: true, required: true},
		{name: "qoder missing runner", client: domain.ClientQoder, executable: true, required: true},
		{name: "copilot automatic", client: domain.ClientCopilot, executable: true, runner: true, activation: domain.ActivationActive},
		{name: "copilot not active", client: domain.ClientCopilot, executable: true, runner: true, activation: domain.ActivationManual, required: true},
		{name: "copilot missing executable", client: domain.ClientCopilot, runner: true, activation: domain.ActivationActive, required: true},
		{name: "copilot missing runner", client: domain.ClientCopilot, executable: true, activation: domain.ActivationActive, required: true},
		{name: "cline legacy", client: domain.ClientCline, required: true},
		{name: "cline foreign objects", client: domain.ClientCline, objectKind: "unrelated", required: true},
		{name: "cline owned skill", client: domain.ClientCline, objectKind: cline.ClineSkillObjectKind},
		{name: "cline owned mcp", client: domain.ClientCline, objectKind: cline.ClineMCPObjectKind},
		{name: "kiro legacy power", client: domain.ClientKiro, required: true},
		{name: "kiro foreign objects", client: domain.ClientKiro, objectKind: "unrelated", required: true},
		{name: "kiro owned skill", client: domain.ClientKiro, objectKind: kiro.SkillObjectKind},
		{name: "kiro owned mcp", client: domain.ClientKiro, objectKind: kiro.MCPObjectKind},
		{name: "windsurf legacy", client: domain.ClientWindsurf, required: true},
		{name: "windsurf foreign objects", client: domain.ClientWindsurf, objectKind: "unrelated", required: true},
		{name: "windsurf owned mcp", client: domain.ClientWindsurf, objectKind: windsurf.WindsurfMCPObjectKind},
		{name: "windsurf missing selected root", client: domain.ClientWindsurf, objectKind: windsurf.WindsurfMCPObjectKind, missingRoot: true, required: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) { assertRemovalPolicy(t, test) })
	}
}

func assertRemovalPolicy(t *testing.T, test removalPolicyCase) {
	t.Helper()
	policy, ok := clients.As[clients.RemovalAttestationPolicy](Default(), test.client)
	if !ok {
		t.Fatal("admitted manual-removal adapter omitted its policy", test.client)
	}
	lifecycle, ok := clients.As[clients.Lifecycle](Default(), test.client)
	if !ok {
		t.Fatal("missing lifecycle", test.client)
	}
	runner := &removalPolicyRunner{}
	env := clients.Env{}
	if test.runner {
		env.Runner = runner
	}
	request := domain.DeactivationRequest{
		Client:       domain.DetectedClient{ClientID: test.client, ConfigRoot: t.TempDir()},
		DeclaredName: "demo", CurrentActivation: test.activation, PhysicalArtifactID: "artifact", ManagedArtifactPath: t.TempDir(),
	}
	if test.missingRoot {
		request.Client.ConfigRoot = ""
	}
	if test.executable {
		request.BackendExecutable = "/fixture/native-client"
	}
	if test.objectKind != "" {
		request.NativeObjects = []domain.NativeObjectOwnership{{Kind: test.objectKind}}
	}
	for _, supplied := range []bool{false, true} {
		request.ExternalUninstalled = supplied
		if got := policy.RequiresExternalUninstalled(env, request); got != test.required {
			t.Fatalf("policy=%v want=%v attested=%v", got, test.required, supplied)
		}
	}
	request.ExternalUninstalled = false
	outcome, err := lifecycle.Deactivate(t.Context(), env, request)
	if err != nil || outcome.ArtifactRemovalAllowed == test.required {
		t.Fatal("policy diverged from the existing deactivation branch", outcome, err)
	}
	if runner.calls != 0 {
		t.Fatal("read-only removal policy or unconfirmed removal executed a native command")
	}
}

type removalPolicyRunner struct{ calls int }

func (runner *removalPolicyRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	return legacyports.CommandResult{}, errors.New("unexpected native command in removal preflight")
}
