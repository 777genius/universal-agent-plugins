package kiro

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const kiroApprovalTestServer = "docs"

func testKiroApprovedEnvelope(approvals string) domain.PackageEnvelope {
	return domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Extensions: map[string]json.RawMessage{
			MCPApprovalNamespace: json.RawMessage(approvals),
		}},
		MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			kiroApprovalTestServer: {
				Name: kiroApprovalTestServer, Type: "streamable-http",
				Decoded: map[string]any{"type": "streamable-http", "url": "https://docs.example.test/mcp"},
			},
		}},
	}
}

func testKiroApprovalPlan(root, active string, scope domain.InstallScope) domain.DeliveryPlan {
	return domain.DeliveryPlan{
		ClientID: domain.ClientKiro, Scope: scope,
		InstallIntent: domain.InstallIntentPrepare, Status: domain.PlanReady,
		NativeRegistryRoot: root, ActivePath: active,
		Components: []domain.ComponentDecision{
			{Kind: domain.ComponentMCPServer, Name: kiroApprovalTestServer, Support: domain.SupportNative},
			{Kind: domain.ComponentExtension, Name: MCPApprovalNamespace, Support: domain.SupportNative},
		},
	}
}

func TestKiroNamedMCPApprovalSchemaFailClosed(t *testing.T) {
	valid := `{"version":1,"mcp_auto_approve":{"docs":["read_docs","get-library.id"]}}`
	for name, raw := range map[string]string{
		"empty":              "",
		"root":               "[]",
		"unsupported-version": `{"version":2,"mcp_auto_approve":{"docs":["read"]}}`,
		"empty-list":         `{"version":1,"mcp_auto_approve":{"docs":[]}}`,
		"wildcard":           `{"version":1,"mcp_auto_approve":{"docs":["*"]}}`,
		"embedded-wildcard":  `{"version":1,"mcp_auto_approve":{"docs":["read*"]}}`,
		"path":               `{"version":1,"mcp_auto_approve":{"docs":["../secret"]}}`,
		"duplicate-tool":     `{"version":1,"mcp_auto_approve":{"docs":["read","read"]}}`,
		"duplicate-server":   `{"version":1,"mcp_auto_approve":{"docs":["read"],"docs":["write"]}}`,
		"unknown-field":      `{"version":1,"mcp_auto_approve":{"docs":["read"]},"all_tools":true}`,
		"wrong-shape":        `{"version":1,"mcp_auto_approve":{"docs":"read"}}`,
		"null":               `{"version":1,"mcp_auto_approve":null}`,
		"trailing-content":   valid + " {}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeMCPApprovals(json.RawMessage(raw)); err == nil {
				t.Fatal("accepted an unsafe Kiro tool approval manifest")
			}
		})
	}
	approved, err := decodeMCPApprovals(json.RawMessage(valid))
	if err != nil || !reflect.DeepEqual(approved.Servers[kiroApprovalTestServer], []string{"read_docs", "get-library.id"}) {
		t.Fatalf("valid exact approval: %+v %v", approved, err)
	}
	if support, _ := New().InterpretExtension(MCPApprovalNamespace, json.RawMessage(valid)); support != domain.SupportNative {
		t.Fatal("Kiro exact tool approvals not qualified as native")
	}
	if support, _ := New().InterpretExtension("foreign.extension", json.RawMessage(valid)); support != domain.SupportUnsupported {
		t.Fatal("Kiro accepted a foreign extension")
	}
}

func TestKiroApprovalPreviewAndNoUnselectedPrivilege(t *testing.T) {
	approved := testKiroApprovedEnvelope(`{"version":1,"mcp_auto_approve":{"docs":["read_docs"]}}`)
	root := filepath.Join(t.TempDir(), ".kiro")
	plan := testKiroApprovalPlan(root, filepath.Join(t.TempDir(), "active"), domain.ScopeUser)
	client := domain.DetectedClient{ClientID: domain.ClientKiro, ConfigRoot: root, ExecutablePath: "/fixture/kiro-cli"}
	if err := New().RefinePlan(context.Background(), clients.PlanInput{Envelope: approved, Client: client}, &plan); err != nil {
		t.Fatal(err)
	}
	preview := strings.Join(plan.Warnings, " ")
	if !strings.Contains(preview, "docs/read_docs") || !strings.Contains(preview, "workspace trust") {
		t.Fatalf("missing exact Kiro trust disclosure: %+v", plan)
	}
	if !kiroNativeComponents(plan.Components) {
		t.Fatal("known approval extension blocked otherwise-native Kiro delivery")
	}
	plan.Components[0].Name = "different-server"
	if _, err := selectedKiroApprovals(approved, plan); err == nil {
		t.Fatal("accepted publisher tool approval for an unselected server")
	}
	if kiroNativeComponents([]domain.ComponentDecision{{Kind: domain.ComponentExtension, Name: MCPApprovalNamespace, Support: domain.SupportNative}}) {
		t.Fatal("accepted approvals without any native component")
	}
}

func TestKiroExactMCPApprovalDisposableLifecycle(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kiro")
	config := filepath.Join(root, "settings", "mcp.json")
	writeTestFile(t, config, `{
  // foreign user preference must survive every operation
  "unrelated": 9007199254740993,
  "mcpServers": {"foreign": {"url": "https://foreign.test", "autoApprove": ["*"]}}
}`)
		var previous []domain.NativeObjectOwnership
	for _, trial := range []struct {
		name string
		list []string
	}{
		{"install", []string{"read_docs"}},
		{"repeat", []string{"read_docs"}},
		{"update", []string{"read_docs", "get_library"}},
		{"repair", []string{"read_docs", "get_library"}},
	} {
		t.Run(trial.name, func(t *testing.T) {
			stage := filepath.Join(t.TempDir(), "stage")
			if err := os.MkdirAll(stage, 0700); err != nil {
				t.Fatal(err)
			}
			policy, err := json.Marshal(map[string]any{"version": 1, "mcp_auto_approve": map[string]any{"docs": trial.list}})
			if err != nil {
				t.Fatal(err)
			}
			envelope := testKiroApprovedEnvelope(string(policy))
			plan := testKiroApprovalPlan(root, filepath.Join(t.TempDir(), "active"), domain.ScopeUser)
			objects, err := New().Project(t.Context(), clients.ProjectionInput{StagingPath: stage, Plan: plan, Envelope: envelope})
			if err != nil || len(objects) != 1 {
				t.Fatalf("projection: %v %+v", err, objects)
			}
			entry, err := projectedKiroMCPServer(stage, "docs")
			if err != nil || !reflect.DeepEqual(entry["autoApprove"], stringsToAny(trial.list)) {
				t.Fatalf("Kiro native approval missing: %v %+v", err, entry)
			}
			if err := applyKiroNativeMutation(root, stage, previous, objects); err != nil {
				t.Fatal(err)
			}
			if err := VerifyNativeObjects(root, objects, false); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(config)
			if err != nil || !strings.Contains(string(before), "// foreign user preference must survive") ||
				!strings.Contains(string(before), "9007199254740993") {
				t.Fatalf("foreign config was changed: %s %v", before, err)
			}
			servers, _, _, _, err := ReadMCPConfig(config)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(servers["docs"].(map[string]any)["autoApprove"], stringsToAny(trial.list)) {
				t.Fatalf("Kiro tool approvals not installed: %+v", servers["docs"])
			}
			previous = objects
		})
	}
	if err := applyKiroNativeMutation(root, "", previous, nil); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(config)
	if err != nil || !strings.Contains(string(after), "// foreign user preference must survive") {
		t.Fatalf("foreign bytes/comments removed: %s %v", after, err)
	}
	servers, _, _, _, err := ReadMCPConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := servers["docs"]; exists {
		t.Fatal("owned MCP tool grant persisted after selected server removal")
	}
	if !reflect.DeepEqual(servers["foreign"].(map[string]any)["autoApprove"], []any{"*"}) {
		t.Fatal("unrelated tool consent was changed")
	}
	if _, err := os.Stat(filepath.Join(root, "settings", "permissions.yaml")); !os.IsNotExist(err) {
		t.Fatalf("Kiro global workspace/tool permissions were unexpectedly created: %v", err)
	}
}

func stringsToAny(values []string) []any {
	items := make([]any, len(values))
	for i, value := range values {
		items[i] = value
	}
	return items
}

// Kiro project trust is separately mediated by the host. This test proves
// native consent is embedded in the selected project MCP config, not in the
// user's global registry or undocumented workspace-trust storage.
func TestKiroProjectApprovalDoesNotForgeWorkspaceTrust(t *testing.T) {
	base := t.TempDir()
	project, user := filepath.Join(base, "project"), filepath.Join(base, "user", ".kiro")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	target, err := domain.NewTargetContext(domain.ScopeProject, project, user)
	if err != nil {
		t.Fatal(err)
	}
	artifact := "demo-0123456789ab"
	active := filepath.Join(project, ".uap", "managed", "clients", "kiro", artifact)
	root := filepath.Join(project, ".kiro")
	plan := testKiroApprovalPlan(root, active, domain.ScopeProject)
	plan.TargetContext, plan.PhysicalArtifactID = target, artifact
	stage := filepath.Join(project, ".uap", "staged")
	if err := os.MkdirAll(stage, 0700); err != nil {
		t.Fatal(err)
	}
	envelope := testKiroApprovedEnvelope(`{"version":1,"mcp_auto_approve":{"docs":["read_docs"]}}`)
	objects, err := New().Project(t.Context(), clients.ProjectionInput{StagingPath: stage, Plan: plan, Envelope: envelope})
	if err != nil || len(objects) != 1 {
		t.Fatalf("project staging: %+v %v", objects, err)
	}
	entry, err := projectedKiroMCPServer(stage, "docs")
	if err != nil || !reflect.DeepEqual(entry["autoApprove"], []any{"read_docs"}) {
		t.Fatalf("project approval missing: %+v %v", entry, err)
	}
	if _, hasType := entry["type"]; hasType {
		t.Fatal("Kiro project native MCP transport was not normalized")
	}
	if _, err := os.Stat(filepath.Join(user, "settings", "permissions.yaml")); !os.IsNotExist(err) {
		t.Fatalf("Kiro global permission file unexpectedly written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(user, "workspace-roots")); !os.IsNotExist(err) {
		t.Fatalf("Kiro workspace trust unexpectedly fabricated: %v", err)
	}
	if got := kiroApprovalDisclosure(mcpApprovals{Servers: map[string][]string{"docs": {"read_docs"}}}); !strings.Contains(got, "workspace trust") {
		t.Fatalf("Kiro trust limit hidden: %s", got)
	}
}
