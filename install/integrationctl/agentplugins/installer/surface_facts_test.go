package installer

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestHostSurfaceFactsRemainPerClient(t *testing.T) {
	base := t.TempDir()
	registry, err := clients.NewRegistry(claude.New(), codex.New(), pi.NewProject(), grok.New())
	if err != nil {
		t.Fatal(err)
	}
	engine := &Engine{cfg: Config{ProjectRoot: filepath.Join(base, "project"), Registry: registry}}
	selected := []domain.ClientSurface{{ID: "claude_cli", Detected: true, Evidence: "selected"}}
	peer := []domain.ClientSurface{{ID: "codex_cli", Detected: false, Evidence: "unavailable"}}
	negative := []domain.ClientSurface{{ID: "pi_config", Detected: true, Evidence: "file-only"}}
	request := Request{Operation: OpInstall, Targets: []ClientTarget{
		{ClientID: "claude", ClientConfigRoot: filepath.Join(base, ".claude"), ClientExecutable: filepath.Join(base, "claude"), ClientSurfaces: selected},
		{ClientID: "codex", ClientConfigRoot: filepath.Join(base, ".codex"), ClientExecutable: filepath.Join(base, "codex"), ClientSurfaces: peer},
	}, KnownTargets: []TargetFacts{{ClientID: "pi", BindingID: "owned-pi", ConfigRoot: filepath.Join(base, "pi"), Surfaces: negative}}}
	frozen := clonePrepareRequest(request)
	selected[0].Detected = false
	peer[0].Detected = true
	negative[0].ID = "pi_cli"
	detected, err := engine.detectedClients(frozen)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[domain.ClientID][]domain.ClientSurface{
		domain.ClientClaude: {{ID: "claude_cli", Detected: true, Evidence: "selected"}},
		domain.ClientCodex:  {{ID: "codex_cli", Detected: false, Evidence: "unavailable"}},
		domain.ClientPi:     {{ID: "pi_config", Detected: true, Evidence: "file-only"}},
	}
	for id, want := range expected {
		if !reflect.DeepEqual(detected[id].Surfaces, want) {
			t.Fatalf("surface crossed client or ownership boundary: %s %+v", id, detected[id].Surfaces)
		}
	}
	frozen.Targets[0].ClientSurfaces[0].Detected = false
	if !detected[domain.ClientClaude].Surfaces[0].Detected {
		t.Fatal("domain client aliases Request surfaces")
	}
	profiles := profileClients(detected)
	for i := range profiles {
		if profiles[i].ClientID == domain.ClientPi {
			profiles[i].Surfaces[0].ID = "pi_cli"
		}
	}
	if detected[domain.ClientPi].Surfaces[0].ID != "pi_config" {
		t.Fatal("profile consumer mutated frozen negative authority")
	}
	physical := physicalClient(Request{physical: detected}, domain.DetectedClient{ClientID: domain.ClientPi})
	physical.Surfaces[0].ID = "pi_cli"
	if detected[domain.ClientPi].Surfaces[0].ID != "pi_config" {
		t.Fatal("physical consumer mutated frozen negative authority")
	}
}

func TestHostSurfaceFactsRefuseGroupLeakageAndConflicts(t *testing.T) {
	if err := validatePrepareInput(context.Background(), Request{ClientSurfaces: []domain.ClientSurface{{ID: "cline_cli", Detected: true}}, Targets: []ClientTarget{{ClientID: "claude"}}}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("global group surface admitted: %v", err)
	}
	original := domain.DetectedClient{ClientID: domain.ClientPi, ConfigRoot: "/selected", Surfaces: []domain.ClientSurface{{ID: "pi_config", Detected: true}}}
	detected := map[domain.ClientID]domain.DetectedClient{domain.ClientPi: original}
	changed := original
	changed.Surfaces = []domain.ClientSurface{{ID: "pi_cli", Detected: true}}
	if err := addDetectedClient(detected, changed); !errors.Is(err, ErrTargetFactsUnavailable) {
		t.Fatalf("contradictory surface silently retained: %v", err)
	}
	if !reflect.DeepEqual(detected[domain.ClientPi], original) {
		t.Fatal("conflicting evidence replaced original authority")
	}
	for _, value := range []any{Request{ClientSurfaces: original.Surfaces}, ClientTarget{ClientSurfaces: original.Surfaces}, TargetFacts{Surfaces: original.Surfaces}} {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if json.Unmarshal(body, &fields) != nil || fields["ClientSurfaces"] != nil || fields["Surfaces"] != nil {
			t.Fatalf("surface authority serialized: %s", body)
		}
	}
	var target ClientTarget
	var fact TargetFacts
	if json.Unmarshal([]byte(`{"ClientSurfaces":[{"id":"pi_cli","detected":true}]}`), &target) != nil || len(target.ClientSurfaces) != 0 {
		t.Fatal("JSON injected target surfaces")
	}
	if json.Unmarshal([]byte(`{"Surfaces":[{"id":"pi_cli","detected":true}]}`), &fact) != nil || len(fact.Surfaces) != 0 {
		t.Fatal("JSON injected sibling surfaces")
	}
}

func TestHostSurfaceFactsPreserveGrokCustomHomeRefusal(t *testing.T) {
	base := t.TempDir()
	registry, err := clients.NewRegistry(grok.New())
	if err != nil {
		t.Fatal(err)
	}
	engine := &Engine{cfg: Config{ProjectRoot: filepath.Join(base, "project"), Registry: registry}}
	request := Request{Operation: OpInstall, ClientID: "grok", ClientConfigRoot: filepath.Join(base, ".grok"), ClientExecutable: filepath.Join(base, "grok")}
	selected, err := engine.detectedClient(request)
	if err != nil {
		t.Fatal(err)
	}
	target := domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: engine.cfg.ProjectRoot, UserConfigRoot: selected.ConfigRoot}
	if _, _, err := grok.New().ProjectTargetRoot(selected, domain.PackageNative, "", target); err != nil {
		t.Fatalf("ordinary Grok control refused: %v", err)
	}
	request.ClientSurfaces = []domain.ClientSurface{{ID: "grok_custom_home", Detected: true}}
	selected, err = engine.detectedClient(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := grok.New().ProjectTargetRoot(selected, domain.PackageNative, "", target); err == nil {
		t.Fatal("SDK lost custom-home refusal")
	}
}
