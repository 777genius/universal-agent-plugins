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
			if _, err := approveCursorMCP(context.Background(), env, request, "audit"); err == nil {
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
	if _, err := approveCursorMCP(context.Background(), env, request, "audit"); err != nil {
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

func TestCursorPreparedSetupCompletesOnlyPendingApproval(t *testing.T) {
	project := t.TempDir()
	request := domain.ActivationRequest{Client: domain.DetectedClient{ConfigRoot: filepath.Join(t.TempDir(), ".cursor")}, Plan: domain.DeliveryPlan{Scope: domain.ScopeProject, InstallIntent: domain.InstallIntentAutomatic, NativeRegistryRoot: filepath.Join(project, ".cursor"), TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project}, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "audit", Support: domain.SupportNative}}}}
	pending := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, UserActions: []string{cursorMCPApprovalPendingAction}}
	adapter := New()
	if adapter.CanCompletePreparedSetup(request, pending) {
		t.Fatal("unknown agent was qualified")
	}
	request.Client.Surfaces = []domain.ClientSurface{{ID: "cursor_agent", Detected: true}}
	if !adapter.CanCompletePreparedSetup(request, pending) {
		t.Fatal("known MCP-only pending approval refused")
	}
	// Regression: an add interrupted before approval left a Skills+MCP package
	// unapproved, and a repeated add reported success without approving it.
	request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "skill", Support: domain.SupportNative})
	if !adapter.CanCompletePreparedSetup(request, pending) {
		t.Fatal("pending MCP approval of a Skills package cannot be completed")
	}
	// Initialized servers or a preserved refusal must not re-run native setup.
	for _, actions := range [][]string{nil, {"Cursor MCP servers initialized; native Skill discovery remains pending in a signed-in Cursor session"}, {`preserved explicit Cursor MCP disablement for "audit"`}} {
		settled := pending
		settled.UserActions = actions
		if adapter.CanCompletePreparedSetup(request, settled) {
			t.Fatalf("completion without pending approval: %q", actions)
		}
	}
	request.Plan.Components = request.Plan.Components[1:]
	if adapter.CanCompletePreparedSetup(request, pending) {
		t.Fatal("Skills-only package has no MCP setup to complete")
	}
}

type cursorListingRunner struct {
	listing  string
	commands [][]string
}

func (r *cursorListingRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	args := command.Argv[1:]
	r.commands = append(r.commands, args)
	switch {
	case reflect.DeepEqual(args, []string{"mcp", "list"}):
		return legacyports.CommandResult{Stdout: []byte(r.listing)}, nil
	case reflect.DeepEqual(args, []string{"--version"}):
		return legacyports.CommandResult{Stdout: []byte("2026.09.18-9a7762b\n")}, nil
	case reflect.DeepEqual(args, []string{"mcp", "list-tools", "audit"}):
		return legacyports.CommandResult{Stdout: []byte("Tools for audit (1):\n- marker ()\n")}, nil
	}
	return legacyports.CommandResult{ExitCode: 1}, nil
}

// A read-only pass reports pending approval without approving; a preserved
// disablement is a refusal, never pending approval.
func TestCursorReadOnlyObservationReportsPendingApprovalOnly(t *testing.T) {
	request, kernel := cursorApprovalFixture(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	runner := &cursorListingRunner{listing: "audit: " + cursorNeedsApprovalStatus + "\n"}
	env := clients.Env{NativeConfig: kernel, Runner: runner, LookupExecutable: func(string) (string, error) { return "/fake/cursor-agent", nil }}
	approvals := cursorMCPPolicyPath(request, "mcp-approvals.json")

	request.VerifyOnly = true
	observed, err := observeCursorProjectMCP(context.Background(), env, request)
	if err != nil || !observed.ApprovalPending || observed.Initialized || observed.Approved {
		t.Fatalf("read-only pending approval: %+v %v", observed, err)
	}
	if _, err := os.Stat(approvals); !os.IsNotExist(err) || len(runner.commands) != 1 {
		t.Fatalf("read-only observation approved or ran extra commands: %v %q", err, runner.commands)
	}

	request.VerifyOnly = false
	observed, err = observeCursorProjectMCP(context.Background(), env, request)
	// The written approval is a native effect; an intact setup reports no other.
	if err != nil || observed.ApprovalPending || !observed.Initialized || !observed.Approved {
		t.Fatalf("confirmed observation did not approve: %+v %v", observed, err)
	}
	if _, err := os.Stat(approvals); err != nil {
		t.Fatalf("approval not written: %v", err)
	}

	if err := os.WriteFile(cursorMCPPolicyPath(request, "mcp-disabled.json"), []byte(`["audit"]`), 0600); err != nil {
		t.Fatal(err)
	}
	request.VerifyOnly = true
	observed, err = observeCursorProjectMCP(context.Background(), env, request)
	if err == nil || observed.ApprovalPending {
		t.Fatalf("disablement reported as pending approval: %+v %v", observed, err)
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
