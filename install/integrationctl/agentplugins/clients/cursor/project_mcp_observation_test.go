package cursor

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type cursorApprovalRunner struct{ commands []legacyports.Command }

func (r *cursorApprovalRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.commands = append(r.commands, command)
	return legacyports.CommandResult{Stdout: []byte("2026.09.18-9a7762b\n")}, nil
}

func TestCursorNativeApprovalPreservesDisablementAndMalformedPolicy(t *testing.T) {
	project := t.TempDir()
	root := filepath.Join(t.TempDir(), ".cursor")
	request := domain.ActivationRequest{Client: domain.DetectedClient{ConfigRoot: root}, Plan: domain.DeliveryPlan{TargetContext: domain.TargetContext{ProjectRoot: project}}}
	for _, test := range []struct{ file, body string }{{"mcp-disabled.json", "[\"audit\"]"}, {"mcp-disabled.json", "{\"foreign\":true}"}, {"mcp-approvals.json", "{\"foreign\":true}"}} {
		t.Run(test.file+test.body, func(t *testing.T) {
			path := cursorMCPPolicyPath(request, test.file)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.body), 0600); err != nil {
				t.Fatal(err)
			}
			runner := &cursorApprovalRunner{}
			env := clients.Env{NativeConfig: nativeconfig.New(), Runner: runner, LookupExecutable: func(string) (string, error) { return "/fake/cursor-agent", nil }}
			if err := approveCursorMCP(context.Background(), env, request, "audit"); err == nil {
				t.Fatal("approved despite existing policy")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != test.body || len(runner.commands) != 0 {
				t.Fatal("refusal changed policy or started native command")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCursorNativeApprovalPinsProjectProfileAndExactServer(t *testing.T) {
	request, kernel := cursorApprovalFixture(t)
	project := request.Plan.TargetContext.ProjectRoot
	root := request.Client.ConfigRoot
	runner := &cursorApprovalRunner{}
	env := clients.Env{NativeConfig: kernel, Runner: runner, LookupExecutable: func(name string) (string, error) {
		if name != "cursor-agent" {
			t.Fatal(name)
		}
		return "/fake/cursor-agent", nil
	}}
	t.Setenv("CURSOR_API_KEY", "must-not-inherit")
	t.Setenv("CURSOR_DATA_DIR", root)
	t.Setenv("CURSOR_CONFIG_DIR", root)
	t.Setenv("XDG_CONFIG_HOME", "")
	if err := approveCursorMCP(context.Background(), env, request, "audit"); err != nil {
		t.Fatal(err)
	}
	if len(runner.commands) != 1 {
		t.Fatal("unexpected command count")
	}
	command := runner.commands[0]
	if !reflect.DeepEqual(command.Argv, []string{"/fake/cursor-agent", "--version"}) || command.Dir != project {
		t.Fatalf("wrong command %+v", command)
	}
	text := strings.Join(command.Env, "\n")
	if !strings.Contains(text, "CURSOR_DATA_DIR="+root) || !strings.Contains(text, "CURSOR_CONFIG_DIR="+root) || strings.Contains(text, "must-not-inherit") || strings.Contains(text, "/foreign-profile") {
		t.Fatalf("wrong native environment %s", text)
	}
}

func TestCursorPreparedSetupRejectsSkillAndUnknownAgent(t *testing.T) {
	project := t.TempDir()
	request := domain.ActivationRequest{Client: domain.DetectedClient{ConfigRoot: filepath.Join(t.TempDir(), ".cursor")}, Plan: domain.DeliveryPlan{Scope: domain.ScopeProject, InstallIntent: domain.InstallIntentAutomatic, NativeRegistryRoot: filepath.Join(project, ".cursor"), TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project}, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "audit", Support: domain.SupportNative}}}}
	outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed}
	adapter := New()
	if adapter.CanCompletePreparedSetup(request, outcome) {
		t.Fatal("unknown agent was qualified")
	}
	request.Client.Surfaces = []domain.ClientSurface{{ID: "cursor_agent", Detected: true}}
	if !adapter.CanCompletePreparedSetup(request, outcome) {
		t.Fatal("known MCP-only setup refused")
	}
	request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "skill", Support: domain.SupportNative})
	if adapter.CanCompletePreparedSetup(request, outcome) {
		t.Fatal("MCP approval qualified native Skill discovery")
	}
}

func TestCursorNativeProfileRefusesDifferentExplicitPolicyRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".cursor")
	t.Setenv("CURSOR_CONFIG_DIR", "/foreign-policy")
	t.Setenv("CURSOR_DATA_DIR", root)
	if cursorNativeProfileEnvironment(root) {
		t.Fatal("bypassed explicit Cursor policy namespace")
	}
	t.Setenv("CURSOR_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/foreign-xdg")
	if cursorNativeProfileEnvironment(root) {
		t.Fatal("bypassed XDG Cursor policy namespace")
	}
}
