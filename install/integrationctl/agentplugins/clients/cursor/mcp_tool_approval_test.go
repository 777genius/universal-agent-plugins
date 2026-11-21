package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

const cursorTestApproval = "{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"inspect_runtime\"]}}"

func TestCursorScopedToolApprovalSchemaAndNameControls(t *testing.T) {
	for _, test := range []struct {
		name  string
		body  string
		valid bool
	}{
		{"exact", cursorTestApproval, true},
		{"two tools", "{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"a.b\",\"a-b\"]}}", true},
		{"version", "{\"version\":2,\"mcp_auto_approve\":{\"audit\":[\"read\"]}}", false},
		{"alias", "{\"Version\":1,\"mcp_auto_approve\":{\"audit\":[\"read\"]}}", false},
		{"unknown", "{\"version\":1,\"extra\":true,\"mcp_auto_approve\":{\"audit\":[\"read\"]}}", false},
		{"empty", "{\"version\":1,\"mcp_auto_approve\":{}}", false},
		{"wildcard tool", "{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"*\"]}}", false},
		{"wildcard server", "{\"version\":1,\"mcp_auto_approve\":{\"*\":[\"read\"]}}", false},
		{"selector injection", "{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"read):Mcp(*:*)\"]}}", false},
		{"duplicate tool", "{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"read\",\"read\"]}}", false},
		{"duplicate server", "{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"read\"],\"audit\":[\"write\"]}}", false},
		{"control", "{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"read\\nmore\"]}}", false},
		{"bidi", "{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"read\\u202e\"]}}", false},
		{"wrong value", "{\"version\":1,\"mcp_auto_approve\":{\"audit\":\"read\"}}", false},
		{"nonobject", "[]", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, _ := New().InterpretExtension(cursorToolApprovalNamespace, json.RawMessage(test.body))
			if (got == domain.SupportNative) != test.valid {
				t.Fatalf("Cursor approval admission differs: %v", got)
			}
		})
	}
	if got, _ := New().InterpretExtension("foreign", json.RawMessage(cursorTestApproval)); got != domain.SupportUnsupported {
		t.Fatal("foreign extension gained Cursor tool approval")
	}
}

func cursorApprovalEnvelope() domain.PackageEnvelope {
	return domain.PackageEnvelope{Manifest: domain.PluginManifest{
		Extensions: map[string]json.RawMessage{cursorToolApprovalNamespace: json.RawMessage(cursorTestApproval)},
	}}
}

func cursorApprovalPlan() domain.DeliveryPlan {
	return domain.DeliveryPlan{
		Scope: domain.ScopeProject, TargetContext: domain.TargetContext{Scope: domain.ScopeProject},
		Components: []domain.ComponentDecision{
			{Kind: domain.ComponentMCPServer, Name: "audit", Support: domain.SupportNative},
			{Kind: domain.ComponentExtension, Name: cursorToolApprovalNamespace, Support: domain.SupportNative},
		},
	}
}

func TestCursorPublisherApprovalsRequireSelectedNativeMCPAndProject(t *testing.T) {
	plan := cursorApprovalPlan()
	envelope := cursorApprovalEnvelope()
	approved, err := selectedCursorToolApprovals(envelope, plan)
	if err != nil || !reflect.DeepEqual(approved["audit"], []string{"inspect_runtime"}) {
		t.Fatalf("valid selection was refused: %v %v", approved, err)
	}
	if !strings.Contains(cursorToolApprovalDisclosure(approved), "Mcp(audit:inspect_runtime)") {
		t.Fatal("plan must disclose the exact newly trusted tool")
	}
	for _, alter := range []func(*domain.DeliveryPlan){
		func(p *domain.DeliveryPlan) { p.Scope = domain.ScopeUser },
		func(p *domain.DeliveryPlan) { p.Components = p.Components[1:] },
		func(p *domain.DeliveryPlan) { p.Components = p.Components[:1] },
		func(p *domain.DeliveryPlan) { p.Components[0].Support = domain.SupportUnsupported },
	} {
		copy := cursorApprovalPlan()
		alter(&copy)
		if _, err := selectedCursorToolApprovals(envelope, copy); err == nil {
			t.Fatalf("unselected permission was admitted: %+v", copy)
		}
	}
	path := filepath.Join(t.TempDir(), "managed")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := projectCursorToolApprovals(path, envelope, plan); err != nil {
		t.Fatal(err)
	}
	if err := verifyCursorToolApprovals(path, envelope, plan); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, cursorToolApprovalFile), []byte("{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"write_runtime\"]}}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCursorToolApprovals(path, envelope, plan); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
		t.Fatalf("changed projection was accepted: %v", err)
	}
}

func TestCursorToolPermissionsMergePreservesForeignAndRefusals(t *testing.T) {
	approved := cursorToolApprovals{"audit": {"inspect_runtime"}}
	initial := []byte("{\"editor\":\"foreign\",\"permissions\":{\"allow\":[\"Bash(ls)\"],\"deny\":[\"Write(secret)\"],\"ask\":[\"Run(elsewhere)\"]}}\n")
	merged, err := cursorMergeToolPermissions(initial, approved)
	if err != nil {
		t.Fatal(err)
	}
	document := map[string]any{}
	if err := json.Unmarshal(merged, &document); err != nil {
		t.Fatal(err)
	}
	permissions := document["permissions"].(map[string]any)
	if document["editor"] != "foreign" || !reflect.DeepEqual(permissions["allow"], []any{"Bash(ls)", "Mcp(audit:inspect_runtime)"}) ||
		!reflect.DeepEqual(permissions["deny"], []any{"Write(secret)"}) || !reflect.DeepEqual(permissions["ask"], []any{"Run(elsewhere)"}) {
		t.Fatalf("foreign Cursor settings changed or permission was too broad: %s", merged)
	}
	repeated, err := cursorMergeToolPermissions(merged, approved)
	if err != nil || !bytes.Equal(repeated, merged) {
		t.Fatalf("repeated approval changed bytes: %v", err)
	}
	for _, raw := range []string{
		"{\"permissions\":{\"deny\":[\"Mcp(audit:inspect_runtime)\"]}}",
		"{\"permissions\":{\"ask\":[\"Mcp(*:*)\"]}}",
		"{\"permissions\":{\"deny\":[\"Mcp(audit:inspect_runtim?)\"]}}",
		"{\"permissions\":{\"ask\":[\"Mcp\"]}}",
		"{\"permissions\":{\"deny\":[\"*\"]}}",
		"{\"permissions\":{\"allow\":\"all\"}}",
		"{\"permissions\":null}",
		"{\"permissions\":{\"deny\":false}}",
		"{\"permissions\":{\"allow\":[false]}}",
		"{\"permissions\":{},\"permissions\":{}}",
	} {
		if _, err := cursorMergeToolPermissions([]byte(raw), approved); err == nil {
			t.Fatalf("malformed or explicitly forbidden Cursor policy was overwritten: %s", raw)
		}
	}
}

func cursorToolTrustFixture(t *testing.T) (domain.ActivationRequest, clients.Env) {
	t.Helper()
	request, kernel := cursorApprovalFixture(t)
	request.Plan.InstallIntent = domain.InstallIntentAutomatic
	request.Plan.ActivePath = request.Delivery.ActivePath
	request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{
		Kind: domain.ComponentExtension, Name: cursorToolApprovalNamespace, Support: domain.SupportNative,
	})
	if err := os.WriteFile(filepath.Join(request.Delivery.ActivePath, cursorToolApprovalFile), []byte(cursorTestApproval), 0600); err != nil {
		t.Fatal(err)
	}
	snapshots := packagesnapshot.Builder{}
	digest, err := snapshots.Digest(t.Context(), request.Delivery.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.ArtifactDigest = digest
	return request, clients.Env{NativeConfig: kernel, Snapshots: snapshots}
}

func TestCursorToolPermissionsExplicitAddAndIdempotentE2E(t *testing.T) {
	request, env := cursorToolTrustFixture(t)
	config := filepath.Join(request.Plan.NativeRegistryRoot, "cli.json")
	original := []byte("{\"foreign\":true,\"permissions\":{\"allow\":[\"Bash(ls)\"]}}\n")
	if err := os.WriteFile(config, original, 0600); err != nil {
		t.Fatal(err)
	}
	outcome := domain.ActivationOutcome{
		Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled,
		Activation: domain.ActivationPrepared, NativeEffect: domain.NativeEffectUnchanged,
	}
	for _, test := range []struct {
		name    string
		consent bool
		verify  bool
		changed bool
	}{
		{"unconsented repair", false, false, false},
		{"verify only", true, true, false},
		{"confirmed add", true, false, true},
		{"repeat confirmed add", true, false, false},
		{"later repair", false, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request.ProjectTrustConsent, request.VerifyOnly = test.consent, test.verify
			before, err := os.ReadFile(config)
			if err != nil {
				t.Fatal(err)
			}
			got, err := completeCursorProjectToolTrust(context.Background(), env, request, outcome)
			if err != nil || (got.NativeEffect == domain.NativeEffectCommitted) != test.changed {
				t.Fatalf("Cursor project trust effect differs: %+v, %v", got, err)
			}
			after, err := os.ReadFile(config)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(before, after) == test.changed {
				t.Fatal("Cursor tool permissions not tied to the confirmed add")
			}
			if test.changed && !bytes.Contains(after, []byte("Mcp(audit:inspect_runtime)")) {
				t.Fatal("named Cursor MCP permission missing")
			}
		})
	}
	request.ProjectTrustConsent = true
	if err := os.WriteFile(config, []byte("{\"permissions\":{\"deny\":[\"Mcp(audit:inspect_runtime)\"]}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(config)
	if _, err := completeCursorProjectToolTrust(context.Background(), env, request, outcome); err == nil {
		t.Fatal("Cursor explicit denial was overwritten")
	}
	after, _ := os.ReadFile(config)
	if !bytes.Equal(before, after) {
		t.Fatal("explicit Cursor denial was changed")
	}
}

func TestCursorToolTrustRefusesSymlinkAndDifferentProject(t *testing.T) {
	for _, test := range []string{"foreign_symlink", "other_project"} {
		t.Run(test, func(t *testing.T) {
			request, env := cursorToolTrustFixture(t)
			request.ProjectTrustConsent = true
			policy := filepath.Join(request.Plan.NativeRegistryRoot, "cli.json")
			other := filepath.Join(t.TempDir(), "foreign.json")
			if err := os.WriteFile(other, []byte("{\"foreign\":true}"), 0600); err != nil {
				t.Fatal(err)
			}
			if test == "foreign_symlink" {
				if err := os.Symlink(other, policy); err != nil {
					t.Skipf("symlink creation unavailable on this platform: %v", err)
				}
			} else {
				request.Plan.TargetContext.ProjectRoot = t.TempDir()
			}
			outcome := domain.ActivationOutcome{Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}
			if _, err := completeCursorProjectToolTrust(context.Background(), env, request, outcome); err == nil {
				t.Fatal("foreign Cursor project or symlink gained MCP tool trust")
			}
			content, err := os.ReadFile(other)
			if err != nil || string(content) != "{\"foreign\":true}" {
				t.Fatalf("foreign config changed: %q %v", content, err)
			}
		})
	}
}

func TestCursorToolTrustRefusesChangedOwnedServerOrDisabledPolicy(t *testing.T) {
	request, env := cursorToolTrustFixture(t)
	request.ProjectTrustConsent = true
	outcome := domain.ActivationOutcome{Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}
	if err := os.WriteFile(filepath.Join(request.Plan.NativeRegistryRoot, "mcp.json"), []byte("{\"mcpServers\":{}}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := completeCursorProjectToolTrust(context.Background(), env, request, outcome); err == nil {
		t.Fatal("changed MCP server inherited approval")
	}
	config := filepath.Join(request.Plan.NativeRegistryRoot, "cli.json")
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatalf("changed native server created Cursor allowlist: %v", err)
	}
	original, err := os.ReadFile(filepath.Join(request.Delivery.ActivePath, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(request.Plan.NativeRegistryRoot, "mcp.json"), original, 0600); err != nil {
		t.Fatal(err)
	}
	policy := cursorMCPPolicyPath(request, "mcp-disabled.json")
	if err := os.MkdirAll(filepath.Dir(policy), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("[\"audit\"]"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := completeCursorProjectToolTrust(context.Background(), env, request, outcome); err == nil {
		t.Fatal("explicitly disabled MCP server inherited tool approval")
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatalf("disabled server changed Cursor permissions: %v", err)
	}
}

func TestCursorToolTrustRefusesPostReviewApprovalFileTamper(t *testing.T) {
	request, env := cursorToolTrustFixture(t)
	request.ProjectTrustConsent = true
	path := filepath.Join(request.Delivery.ActivePath, cursorToolApprovalFile)
	if err := os.WriteFile(path, []byte("{\"version\":1,\"mcp_auto_approve\":{\"audit\":[\"write_runtime\"]}}"), 0600); err != nil {
		t.Fatal(err)
	}
	outcome := domain.ActivationOutcome{Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}
	if _, err := completeCursorProjectToolTrust(t.Context(), env, request, outcome); err == nil {
		t.Fatal("mutated package granted newly injected MCP tool")
	}
	if _, err := os.Stat(filepath.Join(request.Plan.NativeRegistryRoot, "cli.json")); !os.IsNotExist(err) {
		t.Fatalf("tampered package created Cursor permissions: %v", err)
	}
}

// The generic native-component predicate intentionally rejects extensions.
// Cursor must remove only its own qualified extension before invoking it,
// otherwise its confirmed Project installer silently degrades to Manual.
func TestCursorScopedApprovalDoesNotDisableAutomaticProjectLifecycle(t *testing.T) {
	request, env := cursorToolTrustFixture(t)
	request.Plan.ClientID = domain.ClientCursor
	request.Client.ClientID = domain.ClientCursor
	request.Plan.InstallIntent = domain.InstallIntentAutomatic
	request.Plan.TargetContext.Scope = domain.ScopeProject
	if !New().AutomaticallyActivates(env, request) {
		t.Fatal("Cursor qualified approval extension disabled automatic native installation")
	}
	request.Plan.Components = request.Plan.Components[1:]
	if New().AutomaticallyActivates(env, request) {
		t.Fatal("Cursor extension without a native MCP/Skill bypassed component admission")
	}
}

func TestCursorServerApprovalVersionRetainsOnlyNativeQualifiedPins(t *testing.T) {
	for _, row := range []struct {
		version string
		qualified bool
	}{
		{"2026.09.18-9a7762b", true},
		{"2026.10.22-27dcf5f", true},
		{"2026.10.22-27dcf5f-extra", false},
		{"2026.10.23-unknown", false},
		{"", false},
	} {
		if cursorQualifiedApprovalVersion(row.version) != row.qualified {
			t.Fatalf("unexpected native approval admission for %q", row.version)
		}
	}
}
