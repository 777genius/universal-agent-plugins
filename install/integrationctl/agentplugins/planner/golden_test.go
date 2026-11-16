package planner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/internal/goldentest"
)

// plannedOutcome records everything Plan produces, including the operational
// paths the public json tags hide.
type plannedOutcome struct {
	Plan  domain.DeliveryPlan
	Error string
}

type envelopeFixture struct {
	name     string
	envelope domain.PackageEnvelope
}

// TestPlanGoldenAcrossClientsAndEnvelopes freezes the planning decisions of all
// eleven clients before planning moves behind the client registry.
func TestPlanGoldenAcrossClientsAndEnvelopes(t *testing.T) {
	t.Parallel()
	for _, fixture := range goldenEnvelopes() {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			root := goldenRoot(t)
			planner := testPlanner(Planner{ManagedRoot: filepath.Join(root, "managed")})
			outcomes := make(map[string]plannedOutcome, len(domain.ClientDefinitions()))
			for _, definition := range domain.ClientDefinitions() {
				plan, err := planner.Plan(
					context.Background(), fixture.envelope,
					goldenClient(definition.ID, root), domain.ScopeUser, "demo-0123456789ab",
				)
				outcomes[string(definition.ID)] = plannedOutcome{Plan: plan, Error: errorText(err)}
			}
			goldenFor(t, root).Assert(t, "plan_"+fixture.name, outcomes)
		})
	}
}

// TestPlanGoldenPinsDetectedMapDivergence records that the planner built in
// cmd/agentplugins with an empty Detected map and the planner the CLI builds
// with a real one disagree about the native Copilot backend. Part 1 introduces
// domain.PlanRequest; this divergence is current behavior and must be changed
// deliberately, not leveled out on the way past.
func TestPlanGoldenPinsDetectedMapDivergence(t *testing.T) {
	t.Parallel()
	root := goldenRoot(t)
	managed := filepath.Join(root, "managed")
	envelope := goldenEnvelopes()[0].envelope
	client := goldenClient(domain.ClientVSCode, root)

	// A PlanRequest with no surface map is what the composition root sends when
	// detection has not run yet.
	fromCommand, err := testPlanner(Planner{ManagedRoot: managed}).Plan(
		context.Background(), envelope, client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	detected := map[domain.ClientID]domain.DetectedClient{
		domain.ClientCopilot: goldenClient(domain.ClientCopilot, root),
	}
	fromCLIPlanner := testPlanner(Planner{ManagedRoot: managed})
	fromCLIPlanner.Detected = detected
	fromCLI, err := fromCLIPlanner.Plan(
		context.Background(), envelope, client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(fromCommand, fromCLI) {
		t.Fatal("the two composition roots now agree; that is a behavior change, not a cleanup")
	}
	goldenFor(t, root).Assert(t, "plan_detected_divergence", map[string]domain.DeliveryPlan{
		"empty_detected_map": fromCommand,
		"real_detected_map":  fromCLI,
	})
}

// TestCompatibilityGolden freezes the public compatibility facade the authoring
// `compat --format json` command renders.
func TestCompatibilityGolden(t *testing.T) {
	t.Parallel()
	ids := make([]domain.ClientID, 0, len(domain.ClientDefinitions()))
	for _, definition := range domain.ClientDefinitions() {
		ids = append(ids, definition.ID)
	}
	for _, fixture := range goldenEnvelopes() {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			compatibility, err := testCompatibility(fixture.envelope, ids)
			if err != nil {
				t.Fatal(err)
			}
			goldenFor(t, goldenRoot(t)).Assert(t, "compat_"+fixture.name, compatibility)
		})
	}
}

// goldenEnvelopes covers the four shapes the plan calls representative:
// skills only, a stdio MCP server, a remote MCP server bound to an app, and a
// package whose components all failed validation.
func goldenEnvelopes() []envelopeFixture {
	return []envelopeFixture{
		{name: "skills_only", envelope: domain.PackageEnvelope{
			Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0"},
			Skills:   map[string]domain.Skill{"docs": {Name: "docs", Description: "Documentation skill"}},
		}},
		{name: "stdio_mcp", envelope: domain.PackageEnvelope{
			Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0"},
			MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
				"local": {Name: "local", Type: "stdio", Decoded: map[string]any{"command": "demo-server"},
					StdioRequirement: &domain.StdioRequirement{Command: "demo-server", Kind: domain.ExecutableBare}},
			}},
			Skills: map[string]domain.Skill{"docs": {Name: "docs"}},
		}},
		{name: "remote_mcp_and_app", envelope: domain.PackageEnvelope{
			Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0",
				Extensions: map[string]json.RawMessage{"cursor": json.RawMessage(`{"enabled":true}`)}},
			MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
				"remote": {Name: "remote", Type: "streamable-http", Decoded: map[string]any{"url": "https://example.test/mcp"}},
			}},
			App: domain.AppComponent{Present: true, Declared: true, Enabled: true, Bindings: map[string]domain.AppBinding{
				"remote": {Alias: "remote", ID: "asdk_app_demo_123", Required: true},
			}},
			Skills: map[string]domain.Skill{"docs": {Name: "docs"}},
		}},
		{name: "invalid_components", envelope: domain.PackageEnvelope{
			Manifest: domain.PluginManifest{Name: "broken", Version: "1.0.0"},
			Diagnostics: []domain.Diagnostic{{
				Severity: domain.SeverityError, Boundary: domain.BoundarySkill,
				Code: "skill_invalid", Message: "all skills are invalid",
			}},
		}},
	}
}

// goldenClient uses fixed synthetic locators: the planner only inspects them
// lexically, so no directory has to exist and the result is reproducible.
func goldenClient(id domain.ClientID, root string) domain.DetectedClient {
	client := domain.DetectedClient{
		ClientID: id, DisplayName: string(id), Status: domain.DetectionDetected,
		ConfigRoot:     filepath.Join(root, "clients", string(id)),
		ExecutablePath: filepath.Join(root, "bin", string(id)),
	}
	if id == domain.ClientOpenCode {
		client.OpenCodeHost = contracttest.OpenCodeV1Host{}
	}
	return client
}

func goldenRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join(string(filepath.Separator), "agentplugins-golden"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func goldenFor(t *testing.T, root string) goldentest.Golden {
	t.Helper()
	var replacements []goldentest.Replacement
	for _, definition := range domain.ClientDefinitions() {
		clientRoot := filepath.Join(root, "clients", string(definition.ID))
		normalized := "<root>/clients/" + string(definition.ID)
		replacements = append(replacements, goldentest.Replacement{From: strconv.Quote(clientRoot), To: strconv.Quote(normalized)})
	}
	replacements = append(replacements, goldentest.Replacement{From: root, To: "<root>"})
	return goldentest.Golden{Replace: replacements}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestPlanGoldenNormalizesRawAndQuotedClientPaths(t *testing.T) {
	t.Setenv(goldentest.UpdateEnv, "")
	root := goldenRoot(t)
	golden := goldenFor(t, root)
	golden.Dir = t.TempDir()
	expected := map[string]string{
		"raw":    "<root>/clients/opencode",
		"quoted": "Review \"<root>/clients/opencode\" before native startup.",
	}
	body, err := json.MarshalIndent(expected, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(golden.Dir, "quoted_client_path.json"), append(body, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	clientRoot := filepath.Join(root, "clients", "opencode")
	golden.Assert(t, "quoted_client_path", map[string]string{
		"raw":    clientRoot,
		"quoted": "Review " + strconv.Quote(clientRoot) + " before native startup.",
	})
}
