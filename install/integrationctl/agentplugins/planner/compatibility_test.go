package planner

import (
	"encoding/json"
	"errors"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCompatibilityRegistryMatrix(t *testing.T) {
	e := domain.PackageEnvelope{Skills: map[string]domain.Skill{"skill": {}}, MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"http": {Type: "streamable-http"}, "sse": {Type: "sse"}, "stdio": {Type: "stdio"}, "unknown": {Type: "future"}}}, Manifest: domain.PluginManifest{Extensions: map[string]json.RawMessage{"future.namespace": json.RawMessage(`{}`)}}}
	for _, def := range domain.ClientDefinitions() {
		t.Run(string(def.ID), func(t *testing.T) {
			got, err := Compatibility(e, []domain.ClientID{def.ID})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got[0].Capabilities, def.Capabilities) {
				t.Fatal("registry metadata differs")
			}
			base := componentDecisions(e, def.Capabilities)
			for _, c := range got[0].Components {
				if c.Kind == domain.ComponentExtension {
					if c.Support != domain.SupportUnsupported {
						t.Fatal("opaque extension claimed supported")
					}
					continue
				}
				var same []domain.ComponentDecision
				for _, b := range base {
					if b.Kind == c.Kind {
						same = append(same, b)
					}
				}
				if c.Support != same[c.Index-1].Support {
					t.Fatalf("shared decisions diverged: %+v", c)
				}
			}
			if def.Capabilities.ActivationMode == domain.ActivationByUser && !compatContains(got[0].Limitations, "manual_activation_required") {
				t.Fatal("manual limitation missing")
			}
		})
	}
}

func TestCompatibilityChatGPT(t *testing.T) {
	for _, tc := range []struct {
		name, transport string
		enabled, mapped bool
		want            domain.SupportLevel
	}{
		{"unmapped", "streamable-http", true, false, domain.SupportUnsupported},
		{"disabled", "streamable-http", false, true, domain.SupportUnsupported},
		{"remote", "streamable-http", true, true, domain.SupportProjected},
		{"sse", "sse", true, true, domain.SupportProjected},
		{"stdio", "stdio", true, true, domain.SupportUnsupported},
		{"unknown", "future", true, true, domain.SupportUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"server": {Type: tc.transport}}}, App: domain.AppComponent{Enabled: tc.enabled, Bindings: map[string]domain.AppBinding{}}}
			if tc.mapped {
				e.App.Bindings["server"] = domain.AppBinding{ID: "registered-reference"}
			}
			got, err := Compatibility(e, []domain.ClientID{domain.ClientChatGPT})
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range got[0].Components {
				if c.Kind == domain.ComponentMCPServer && c.Support != tc.want {
					t.Fatalf("wrong support: %+v", c)
				}
			}
			for _, code := range []string{"chatgpt_manual_preparation_only", "remote_app_registration_not_checked"} {
				if !compatContains(got[0].Limitations, code) {
					t.Fatal(code)
				}
			}
		})
	}
}

func TestCompatibilityInvalidSiblings(t *testing.T) {
	e := domain.PackageEnvelope{Skills: map[string]domain.Skill{"good": {}}, Inventory: domain.ComponentInventory{InvalidSkills: []string{"bad"}}, MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"good": {Type: "stdio"}}, InvalidServer: map[string]domain.Diagnostic{"bad": {Message: "fixture-diagnostic"}}}, Diagnostics: []domain.Diagnostic{{Severity: domain.SeverityError, Boundary: domain.BoundarySkill, Item: "bad", Message: "fixture-diagnostic"}}}
	got, err := Compatibility(e, []domain.ClientID{domain.ClientCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Components) != 4 {
		t.Fatal("invalid inventory lost")
	}
	for _, c := range got[0].Components {
		want := domain.SupportNative
		if c.Index == 1 {
			want = domain.SupportUnsupported
		}
		if c.Support != want {
			t.Fatalf("invalid sibling isolation: %+v", c)
		}
	}
}

func TestCompatibilityTargetsAndOwnership(t *testing.T) {
	e := domain.PackageEnvelope{Skills: map[string]domain.Skill{"only": {}}}
	a, err := Compatibility(e, []domain.ClientID{domain.ClientCursor, domain.ClientCodex, domain.ClientCursor})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Compatibility(e, []domain.ClientID{domain.ClientCodex, domain.ClientCursor})
	if !reflect.DeepEqual(a, b) {
		t.Fatal("duplicates/order affect output")
	}
	a[0].Capabilities.MCPTransports["stdio"] = domain.SupportUnsupported
	a[0].Capabilities.Scopes[0] = domain.ScopeProject
	c, _ := Compatibility(e, []domain.ClientID{domain.ClientCodex, domain.ClientCursor})
	if !reflect.DeepEqual(b, c) {
		t.Fatal("registry mutated")
	}
	for _, id := range []domain.ClientID{"", "CODEX", domain.ClientID(strings.Repeat("fixture-client\n/", 10000))} {
		got, err := Compatibility(e, []domain.ClientID{domain.ClientCursor, id})
		if got != nil || !errors.Is(err, ErrUnknownCompatibilityClient) || err.Error() != "unknown compatibility client" {
			t.Fatal("unbounded or partial error")
		}
	}
	empty, err := Compatibility(e, nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal("empty targets")
	}
}

func TestCompatibilityEmptyHostAndNoSensitiveEvidence(t *testing.T) {
	const fixtureMarker = "fixture-private-value"
	const fixturePath = "fixtures/client/config.json"
	e := domain.PackageEnvelope{
		SnapshotRoot: fixturePath,
		Source:       domain.SourceIdentity{RequestedSource: fixturePath},
		Manifest: domain.PluginManifest{Extensions: map[string]json.RawMessage{
			fixtureMarker: json.RawMessage(`{"opaque_note":"fixture-private-value"}`),
		}},
		Skills: map[string]domain.Skill{
			fixtureMarker: {RelativePath: fixturePath, Raw: []byte(fixtureMarker)},
		},
		MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
			fixtureMarker: {
				Type: "stdio",
				Decoded: map[string]any{
					"env":     map[string]string{"FIXTURE_NOTE": fixtureMarker},
					"headers": map[string]string{"X-Fixture-Note": fixtureMarker},
					"args":    []string{fixtureMarker, fixturePath},
				},
				StdioRequirement: &domain.StdioRequirement{Command: fixturePath},
			},
		}},
		Diagnostics: []domain.Diagnostic{{
			Severity: domain.SeverityError, Boundary: domain.BoundaryApp,
			Item: fixtureMarker, Code: fixtureMarker, Path: fixturePath, Message: fixtureMarker,
		}},
		CatalogEvidence: &domain.CatalogEvidence{},
	}
	before, _ := json.Marshal(e)
	a, err := Compatibility(e, domain.SupportedClientIDs())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("USERPROFILE", home)
	b, err := Compatibility(e, domain.SupportedClientIDs())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("host affected output")
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatal("host write")
	}
	after, _ := json.Marshal(e)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
	raw, _ := json.Marshal(b)
	if strings.Contains(string(raw), fixtureMarker) || strings.Contains(string(raw), fixturePath) {
		t.Fatalf("sensitive output: %s", raw)
	}
	e.CatalogEvidence = nil
	without, _ := Compatibility(e, domain.SupportedClientIDs())
	if !reflect.DeepEqual(b, without) {
		t.Fatal("catalog influenced static output")
	}
	for _, client := range b {
		for _, code := range []string{"installation_not_checked", "authentication_not_checked", "runtime_not_checked", "client_version_not_checked", "catalog_publication_not_checked"} {
			if !compatContains(client.Limitations, code) {
				t.Fatal(code)
			}
		}
	}
}

func compatContains(items []string, want string) bool {
	for _, s := range items {
		if s == want {
			return true
		}
	}
	return false
}

func TestCompatibilityDisabledAndComponentOnly(t *testing.T) {
	for _, tc := range []struct {
		name     string
		envelope domain.PackageEnvelope
		want     domain.SupportLevel
	}{
		{"skill-only", domain.PackageEnvelope{Skills: map[string]domain.Skill{"one": {}}}, domain.SupportNative},
		{"mcp-only", domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"one": {Type: "stdio"}}}}, domain.SupportNative},
		{"disabled-mcp", domain.PackageEnvelope{MCP: domain.MCPComponent{Present: true, Servers: map[string]domain.MCPServer{"one": {Type: "stdio"}}}}, domain.SupportUnsupported},
		{"invalid-root", domain.PackageEnvelope{Skills: map[string]domain.Skill{"one": {}}, Inventory: domain.ComponentInventory{InvalidSkillsRoot: true}}, domain.SupportUnsupported},
		{"app-other-client", domain.PackageEnvelope{App: domain.AppComponent{Enabled: true, Bindings: map[string]domain.AppBinding{"one": {ID: "reference"}}}}, domain.SupportUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compatibility(tc.envelope, []domain.ClientID{domain.ClientCursor})
			if err != nil || len(got[0].Components) != 1 || got[0].Components[0].Support != tc.want {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
	got, err := Compatibility(domain.PackageEnvelope{}, []domain.ClientID{domain.ClientChatGPT})
	if err != nil || len(got[0].Components) != 0 {
		t.Fatal("empty package invented components")
	}
}

// Exact keys, even empty ones, are identities in every item-level source.
func TestCompatibilityEmptyInvalidIdentity(t *testing.T) {
	for _, source := range []string{"map", "inventory", "diagnostic", "overlap"} {
		for _, name := range []string{"", "a-private-invalid-name", "z-private-invalid-name"} {
			t.Run(source+"/"+name, func(t *testing.T) {
				e := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"good": {Type: "stdio"}}}}
				switch source {
				case "map":
					e.MCP.InvalidServer = map[string]domain.Diagnostic{name: {}}
				case "inventory":
					e.Inventory.InvalidMCPServer = []string{name}
				case "overlap":
					e.MCP.Servers[name] = domain.MCPServer{Type: "stdio"}
					fallthrough
				case "diagnostic":
					e.Diagnostics = []domain.Diagnostic{{Severity: domain.SeverityError, Boundary: domain.BoundaryMCPServer, Item: name}}
				}
				var first []byte
				for repeat := 0; repeat < 10; repeat++ {
					got, err := Compatibility(e, []domain.ClientID{domain.ClientCursor})
					if err != nil || len(got[0].Components) != 2 {
						t.Fatalf("inventory: %+v, %v", got, err)
					}
					for i, c := range got[0].Components {
						invalid := (i == 0) == (name < "good")
						want := domain.SupportNative
						if invalid {
							want = domain.SupportUnsupported
						}
						if c.Kind != domain.ComponentMCPServer || c.Index != i+1 || c.Support != want || compatContains(c.Limitations, "invalid_component") != invalid {
							t.Fatalf("identity boundary: %+v", c)
						}
					}
					raw, _ := json.Marshal(got)
					if strings.Contains(string(raw), "private-invalid-name") || strings.Contains(string(raw), `"good"`) {
						t.Fatal("name disclosure")
					}
					if repeat > 0 && string(raw) != string(first) {
						t.Fatal("unstable indexes")
					}
					first = raw
				}
			})
		}
	}
}

func TestCompatibilityDocumentBoundary(t *testing.T) {
	for _, item := range []string{"", "document-private-marker"} {
		e := domain.PackageEnvelope{
			Skills:      map[string]domain.Skill{"good": {}},
			MCP:         domain.MCPComponent{Servers: map[string]domain.MCPServer{"": {Type: "stdio"}, "good": {Type: "stdio"}}},
			Diagnostics: []domain.Diagnostic{{Severity: domain.SeverityError, Boundary: domain.BoundaryMCP, Item: item}},
		}
		got, err := Compatibility(e, []domain.ClientID{domain.ClientCursor})
		if err != nil || len(got[0].Components) != 3 {
			t.Fatalf("document boundary invented inventory: %+v, %v", got, err)
		}
		for _, c := range got[0].Components {
			want := domain.SupportNative
			if c.Kind == domain.ComponentMCPServer {
				want = domain.SupportUnsupported
			}
			if c.Support != want {
				t.Fatalf("document boundary: %+v", c)
			}
		}
	}
}
