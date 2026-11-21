package installer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This exercises the shared SDK boundary with an already scoped adapter. The
// Devin lane separately tests interpretation of the selected host evidence.
func TestProjectPreparePreservesSelectedHostVersion(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project, pkg := filepath.Join(base, "project"), filepath.Join(base, "package")
	for _, root := range []string{project, pkg} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(pkg, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"host-evidence","version":"1.0.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(gemini.New())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := newTestEngine(t, Config{Registry: registry, ProjectRoot: project, StateRoot: filepath.Join(project, ".uap")})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Scope: domain.ScopeProject, ProjectRoot: project, Operation: OpInstall, PackageRoot: pkg,
		ClientID: "gemini", ClientConfigRoot: filepath.Join(base, "profile"), ClientExecutable: filepath.Join(base, "gemini"), ClientVersion: "host-only-version"}
	prepared, err := engine.Prepare(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = prepared.Close() }()
	req.ClientVersion = "caller-changed"
	if prepared.client.Version != "host-only-version" || prepared.req.ClientVersion != "host-only-version" || prepared.client.ConfigRoot != req.ClientConfigRoot || prepared.Plan().TargetContext.UserConfigRoot != req.ClientConfigRoot {
		t.Fatalf("host evidence or original profile changed: %+v", prepared.client)
	}
	body, err := json.Marshal(prepared.req)
	if err != nil || strings.Contains(string(body), "host-only-version") || strings.Contains(string(body), "ClientVersion") {
		t.Fatalf("operational version leaked into diagnostic JSON: %s %v", body, err)
	}
	var decoded Request
	if err := json.Unmarshal([]byte(`{"ClientVersion":"source-injected"}`), &decoded); err != nil || decoded.ClientVersion != "" {
		t.Fatalf("JSON injected operational host version: %+v %v", decoded, err)
	}
	for _, root := range []string{"relative", filepath.Join(base, "profile") + string(filepath.Separator) + "."} {
		req.ClientConfigRoot = root
		if _, err := engine.Prepare(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("invalid nonempty project profile %q accepted: %v", root, err)
		}
	}
}

func TestProjectHostEvidenceDoesNotRelaxUserProfileAdmission(t *testing.T) {
	engine, err := newTestEngine(t, Config{StateRoot: filepath.Join(t.TempDir(), "state")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.detectedClient(Request{Operation: OpRemove, ClientID: "codex", ClientVersion: "host-evidence"})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("user scope accepted absent original profile: %v", err)
	}
}

func TestProjectHostEvidenceAllowsAbsentGenericProfileOnlyInProject(t *testing.T) {
	registry, err := clients.NewRegistry(windsurf.New())
	if err != nil {
		t.Fatal(err)
	}
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := newTestEngine(t, Config{Registry: registry, ProjectRoot: project, StateRoot: filepath.Join(project, ".uap")})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Scope: domain.ScopeProject, ProjectRoot: project, Operation: OpInstall, ClientID: "windsurf", ClientVersion: "host-version"}
	// Context validation is always before reconstruction in public Prepare.
	// The adapter's new scoped lane has its own full public lifecycle coverage.
	if err := engine.CheckProjectNamespace(); err != nil {
		t.Fatal(err)
	}
	client, err := engine.detectedClient(req)
	if err != nil || client.ConfigRoot != "" || client.Version != req.ClientVersion {
		t.Fatalf("absent original profile: %+v %v", client, err)
	}
	strict, err := clients.NewRegistry(gemini.New())
	if err != nil {
		t.Fatal(err)
	}
	engine.cfg.Registry = strict
	req.ClientID = "gemini"
	if _, err := engine.detectedClient(req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("stricter profile resolver bypassed: %v", err)
	}
}

func TestProjectSelectedClientSurfacesAreOwnedAndNotJSON(t *testing.T) {
	registry, err := clients.NewRegistry(copilot.New())
	if err != nil {
		t.Fatal(err)
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(base, "project")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	engine, err := newTestEngine(t, Config{Registry: registry, ProjectRoot: project, StateRoot: filepath.Join(project, ".uap")})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{Scope: domain.ScopeProject, ProjectRoot: project, ClientID: "copilot", Operation: OpInstall,
		ClientConfigRoot: filepath.Join(base, "copilot"), ClientExecutable: filepath.Join(base, "copilot"),
		ClientSurfaces: []domain.ClientSurface{{ID: "copilot_cli", Detected: true, Evidence: "selected-host-only"}}}
	copied := clonePrepareRequest(request)
	request.ClientSurfaces[0].ID = "mutated-caller"
	client, err := engine.detectedClient(copied)
	if err != nil || len(client.Surfaces) != 1 || client.Surfaces[0].ID != "copilot_cli" || !client.Surfaces[0].Detected {
		t.Fatalf("selected operational surface lost across installer boundary: %+v %v", client, err)
	}
	client.Surfaces[0].ID = "mutated-result"
	if copied.ClientSurfaces[0].ID != "copilot_cli" {
		t.Fatal("detected surface aliases the copied request")
	}
	body, err := json.Marshal(copied)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["ClientSurfaces"]; exists || strings.Contains(string(body), "selected-host-only") {
		t.Fatalf("private host surface leaked into public request JSON: %s", body)
	}
	var injected Request
	if err := json.Unmarshal([]byte(`{"ClientSurfaces":[{"id":"copilot_cli","detected":true}]}`), &injected); err != nil || len(injected.ClientSurfaces) != 0 {
		t.Fatalf("JSON injected operational host surface: %+v %v", injected, err)
	}
}
