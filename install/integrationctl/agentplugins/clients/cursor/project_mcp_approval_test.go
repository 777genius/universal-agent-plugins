package cursor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func cursorApprovalFixture(t *testing.T) (domain.ActivationRequest, nativeconfig.Kernel) {
	t.Helper()
	project := t.TempDir()
	root := filepath.Join(t.TempDir(), ".cursor")
	native := filepath.Join(project, ".cursor")
	active := filepath.Join(project, "managed")
	for _, path := range []string{root, native, active} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	kernel := nativeconfig.New()
	layout := cursorComponents(native)
	server := nativeconfig.Server{Type: "stdio", Command: "/TEST/native-server", StdioValuesResolved: true, CWDResolved: true}
	receipt, err := kernel.Apply(nativeconfig.Request{Paths: layout.ConfigPaths, Codec: layout.Codec, Action: nativeconfig.ActionAdd, Name: "audit", Server: server})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(layout.ConfigPaths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, "mcp.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	object := domain.NativeObjectOwnership{ObjectID: CursorProjectMCPKind + ":audit", Kind: CursorProjectMCPKind, LogicalName: "audit", Path: layout.ConfigPaths.JSON, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}
	request := domain.ActivationRequest{Client: domain.DetectedClient{ConfigRoot: root}, Plan: domain.DeliveryPlan{Scope: domain.ScopeProject, NativeRegistryRoot: native, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project}, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "audit", Support: domain.SupportNative}}}, Delivery: domain.StagedDelivery{ActivePath: active, NativeObjects: []domain.NativeObjectOwnership{object}}}
	return request, kernel
}

func TestCursorApprovalFingerprintMatchesNativeJSONStringify(t *testing.T) {
	source := `{"type":"stdio","command":"/usr/bin/python3","args":["/TEST/server.py","<>&","line\u2028break","literal\\u2028"],"env":{"Z_LAST":"literal <html> Ω \u2028","A_FIRST":"quote\"\\slash"},"ignored":"unknown"}`
	server, err := cursorNormalizeApprovalServer([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	body, err := cursorJSONStringify(struct {
		Path   string               `json:"path"`
		Server cursorApprovalServer `json:"server"`
	}{"/TEST/NativeProject", server})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	// Independent JSON.stringify recipe vector; live Cursor 2026.09.18 also
	// accepted the direct ordered-entry spike (approval-spike-c8bfd947bec1c462).
	if got := hex.EncodeToString(sum[:8]); got != "ebc3bfeab55b86f1" {
		t.Fatalf("native fingerprint differs: %s %s", got, body)
	}
}

func TestCursorApprovalCASNeverRemovesConcurrentDisable(t *testing.T) {
	request, _ := cursorApprovalFixture(t)
	disabled := cursorMCPPolicyPath(request, "mcp-disabled.json")
	approval := cursorMCPPolicyPath(request, "mcp-approvals.json")
	if err := os.MkdirAll(filepath.Dir(approval), 0700); err != nil {
		t.Fatal(err)
	}
	before := []byte("[\"foreign-approval\"]\n")
	if err := os.WriteFile(approval, before, 0600); err != nil {
		t.Fatal(err)
	}
	concurrent := []byte("[\"foreign-disabled\",\"audit\"]\n")
	kernel := nativeconfig.NewWithLockAcquirer(func(paths nativeconfig.Paths, _ nativeconfig.Codec) (func() error, error) {
		if paths.JSON == approval {
			if err := os.WriteFile(disabled, concurrent, 0600); err != nil {
				return nil, err
			}
		}
		return func() error { return nil }, nil
	})
	runner := &cursorApprovalRunner{}
	env := clients.Env{NativeConfig: kernel, Runner: runner, LookupExecutable: func(string) (string, error) { return "/fake/cursor-agent", nil }}
	if err := approveCursorMCP(context.Background(), env, request, "audit"); err == nil {
		t.Fatal("concurrent deny was not retained")
	}
	after, err := os.ReadFile(disabled)
	if err != nil || string(after) != string(concurrent) {
		t.Fatalf("concurrent disabled policy changed: %q %v", after, err)
	}
	after, err = os.ReadFile(approval)
	if err != nil || string(after) != string(before) {
		t.Fatalf("foreign approval state changed: %q %v", after, err)
	}
	for _, command := range runner.commands {
		if strings.Contains(strings.Join(command.Argv, " "), "mcp enable") {
			t.Fatal("unsafe native enable was invoked")
		}
	}
}

type cursorFailingProbe struct{ result legacyports.CommandResult }

func (r cursorFailingProbe) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return r.result, context.DeadlineExceeded
}

func TestCursorCompletionPropagatesUnclosedProbeMetadata(t *testing.T) {
	for _, test := range []struct {
		name   string
		result legacyports.CommandResult
		unsafe bool
	}{{"unknown", legacyports.CommandResult{}, true}, {"closed", legacyports.CommandResult{CleanupComplete: true}, false}, {"not-started", legacyports.CommandResult{NoChildStarted: true}, false}} {
		t.Run(test.name, func(t *testing.T) {
			request, kernel := cursorApprovalFixture(t)
			request.VerifyOnly = true
			env := clients.Env{NativeConfig: kernel, Runner: cursorFailingProbe{test.result}, LookupExecutable: func(string) (string, error) { return "/fake/cursor-agent", nil }}
			outcome, err := completeCursorProjectMCP(context.Background(), env, request, domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectUnchanged})
			if domain.IsUnclosedClientProbe(err) != test.unsafe {
				t.Fatalf("lost cleanup metadata: %v", err)
			}
			if test.unsafe && outcome.NativeEffect != domain.NativeEffectUncertain {
				t.Fatal("unclosed process did not retain uncertain effect")
			}
			if !test.unsafe && err != nil {
				t.Fatalf("closed unavailable probe should remain prepared: %v", err)
			}
		})
	}
	request, kernel := cursorApprovalFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := completeCursorProjectMCP(ctx, clients.Env{NativeConfig: kernel}, request, domain.ActivationOutcome{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("parent cancellation lost: %v", err)
	}
}

func TestCursorApprovalRefusesPolicyDirectoryAlias(t *testing.T) {
	request, kernel := cursorApprovalFixture(t)
	foreign := t.TempDir()
	if err := os.Symlink(foreign, filepath.Join(request.Client.ConfigRoot, "projects")); err != nil {
		t.Skip(err)
	}
	runner := &cursorApprovalRunner{}
	env := clients.Env{NativeConfig: kernel, Runner: runner, LookupExecutable: func(string) (string, error) { return "/fake/cursor-agent", nil }}
	if err := approveCursorMCP(context.Background(), env, request, "audit"); err == nil {
		t.Fatal("approval followed a foreign policy directory alias")
	}
	entries, err := os.ReadDir(foreign)
	if err != nil || len(entries) != 0 || len(runner.commands) != 0 {
		t.Fatal("policy refusal changed foreign data or started native code")
	}
}
