package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kimi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type kimiRawNative struct {
	t                         *testing.T
	project, profile, version string
	queries, registrations    int
	registered                bool
}

func (n *kimiRawNative) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{NoChildStarted: true}, fmt.Errorf("unexpected ordinary native command")
}

func (n *kimiRawNative) RunDuplexToCompletion(_ context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	n.queries++
	if command.Dir != n.project || !reflect.DeepEqual(command.Argv[1:], []string{"web", "--no-open", "--host", "127.0.0.1", "--port", "0"}) {
		n.t.Fatal("native selected command changed", command)
	}
	if err := os.MkdirAll(n.profile, 0700); err != nil {
		n.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(n.profile, "server.token"), []byte(strings.Repeat("A", 43)), 0600); err != nil {
		n.t.Fatal(err)
	}
	banner := "  fixture  Kimi server ready  " + n.version + "\n  Local:    http://127.0.0.1:12345/#token=" + strings.Repeat("A", 43) + "\n  Token:    " + strings.Repeat("A", 43) + "\n"
	return exchange(io.Discard, strings.NewReader(banner))
}

func (n *kimiRawNative) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host != "127.0.0.1:12345" || request.Header.Get("Authorization") != "Bearer "+strings.Repeat("A", 43) {
		n.t.Fatal("native bearer escaped selected origin")
	}
	workspace := map[string]any{"id": "wd_project_0123456789ab", "root": n.project, "name": "project", "created_at": "2026-11-19T00:00:00Z", "last_opened_at": "2026-11-19T00:00:00Z", "session_count": 0}
	if n.version == "2.5.2" {
		workspace["id"], workspace["kind"], workspace["additional_dirs"] = "wm_01234567-0123-4567-89ab-0123456789ab", "managed", []any{}
	}
	var data map[string]any
	switch request.Method + " " + request.URL.Path {
	case "GET /api/v1/meta":
		data = map[string]any{"server_version": n.version, "dangerous_bypass_auth": false, "backend": "v2", "server_id": "inert-raw-service"}
	case "GET /api/v1/workspaces":
		rows := []any{}
		if n.registered {
			rows = append(rows, workspace)
		}
		data = map[string]any{"items": rows}
	case "POST /api/v1/workspaces":
		n.registrations++
		n.registered = true
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			n.t.Fatal(err)
		}
		if body["root"] != n.project || n.version == "2.5.2" && body["idempotency_key"] == "" {
			n.t.Fatal("workspace identity not bound", body)
		}
		data = workspace
	case "POST /api/v1/shutdown":
		data = map[string]any{"ok": true}
	default:
		if request.Method != "GET" || !strings.HasSuffix(request.URL.Path, "/skills") {
			n.t.Fatal("forbidden native operation", request.Method, request.URL.Path)
		}
		data = map[string]any{"skills": []any{map[string]any{"name": "pdf", "description": "Read PDFs", "source": "project", "path": filepath.Join(n.project, ".kimi-code", "skills", "pdf", "SKILL.md")}}}
	}
	body, err := json.Marshal(map[string]any{"code": 0, "msg": "success", "request_id": "raw-fixture", "data": data})
	if err != nil {
		n.t.Fatal(err)
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
}

func kimiRawJourney(t *testing.T, journey *savedJourney, version string, nativeHTTP bool) *kimiRawNative {
	t.Helper()
	base, err := filepath.EvalSymlinks(journey.cli.root)
	if err != nil {
		t.Fatal(err)
	}
	home, project := filepath.Join(base, "original-home"), filepath.Join(base, "selected-project")
	profile := filepath.Join(home, ".kimi-code")
	for path, body := range map[string]string{filepath.Join(project, ".git", "HEAD"): "inert marker\n", filepath.Join(profile, "config.toml"): "telemetry = false\n", filepath.Join(profile, "mcp.json"): `{"mcpServers":{"foreign":{"url":"https://foreign.invalid/mcp"}}}`} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(base, "kimi")
	if err := os.WriteFile(executable, []byte("inert selected CLI; never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientKimi, ConfigRoot: profile, ExecutablePath: executable, Version: version, Status: domain.DetectionDetected}
	app := &journey.cli.app
	app.Scope, app.ProjectRoot, app.UserHome = domain.ScopeProject, project, home
	stateRoot := filepath.Join(project, ".uap")
	journey.cli.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: project}
	journey.cli.operations = filepath.Join(stateRoot, "operations")
	app.StateStore, app.Detector = journey.cli.store, staticDetector{clients: []domain.DetectedClient{client}}
	app.ManagedRoot, app.DependenciesRoot = filepath.Join(stateRoot, "managed"), project
	app.EnvironmentLockPath, app.EnvironmentStoreRoot = filepath.Join(stateRoot, "environment.lock"), filepath.Join(stateRoot, "store")
	app.ClientRegistry, err = clients.NewRegistry(kimi.New())
	if err != nil {
		t.Fatal(err)
	}
	n := &kimiRawNative{t: t, project: project, profile: profile, version: version}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	config := installer.Config{ProjectRoot: project, UserHome: home, StateRoot: stateRoot, Registry: app.ClientRegistry, Runner: n, HelperExecutable: helper, EnableNativeObserver: true}
	if nativeHTTP {
		config.NativeHTTP = n
	}
	app.Installer, err = installer.New(config)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestKimiPublicRawProjectSkillsAddRepeatUsesConfirmedNativeCompletion(t *testing.T) {
	for _, version := range []string{"2.3.2", "2.5.2"} {
		t.Run(version, func(t *testing.T) {
			_, journey, source, _ := managementMutableWebsite(t)
			native := kimiRawJourney(t, &journey, version, true)
			foreignMCP := filepath.Join(native.profile, "mcp.json")
			foreignBefore, err := os.ReadFile(foreignMCP)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"skills", "add", source, "--skill", "pdf", "--target", "kimi", "--no-ui", "--yes", "--scope", "project", "--project-root", native.project, "--format", "json"}
			request := SkillsBatchRequest{Selections: []SkillsSelection{{Website: source, Name: "pdf", Alias: "pdf"}}, TargetIDs: []string{"kimi"}}
			var before domain.ClientBinding
			var lockedRequirements, locked any
			for attempt := range 2 {
				preview, err := journey.cli.app.PrepareSkills(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				if err := preview.Check(t.Context()); err != nil {
					t.Fatal(err)
				}
				preview.Close()
				if native.queries != attempt {
					t.Fatal("read-only review started native service", native.queries)
				}
				body := journey.run(t, args...)
				var result map[string]any
				if err := json.Unmarshal([]byte(body), &result); err != nil {
					t.Fatal(err)
				}
				items, ok := result["items"].([]any)
				if !ok || len(items) != 1 {
					t.Fatal("public selected Skill item missing", body)
				}
				item, ok := items[0].(map[string]any)
				// Explicit re-add completes reviewed native setup repair as succeeded.
				if !ok || item["targetId"] != "kimi" || item["status"] != "succeeded" {
					t.Fatal("public selected Skill item status differs", body)
				}
				binding := savedJourneyBinding(t, journey.state(t), "kimi")
				if binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationNotRequired || result["readiness"] != "ready" || native.queries != attempt+1 {
					t.Fatalf("public raw native completion failed: %+v queries=%d result=%s", binding, native.queries, body)
				}
				for _, installation := range journey.state(t).Installations {
					if installation.Source.StableSourceKey == "" || installation.DeclaredName != "skill-"+fmt.Sprintf("%x", sha256.Sum256([]byte(installation.Source.StableSourceKey)))[:56] || installation.DeclaredName == "pdf" {
						t.Fatal("raw source lost canonical synthetic package identity", installation)
					}
				}
				for _, name := range []string{"uap.toml", "uap.lock"} {
					info, err := os.Lstat(filepath.Join(native.project, name))
					if err != nil || !info.Mode().IsRegular() {
						t.Fatal("raw fixture lost production Project dependency path", name, err)
					}
					if _, err := os.Lstat(filepath.Join(native.project, ".uap", name)); !os.IsNotExist(err) {
						t.Fatal("raw fixture created shadow dependency path", name, err)
					}
				}
				requirements, lock := journey.intent(t)
				if attempt == 0 {
					before = binding
					lockedRequirements, locked = requirements, lock
				} else if before.ClientBindingID != binding.ClientBindingID || !reflect.DeepEqual(before.PackageRevision, binding.PackageRevision) || !reflect.DeepEqual(lockedRequirements, requirements) || !reflect.DeepEqual(locked, lock) {
					t.Fatal("repeat changed exact binding or source/lock")
				}
			}
			removed := journey.run(t, "remove", "pdf", "--target", "kimi", "--scope", "project", "--project-root", native.project, "--yes", "--format", "json")
			var removal map[string]any
			if err := json.Unmarshal([]byte(removed), &removal); err != nil {
				t.Fatal(err)
			}
			if removal["command"] != "agentplugins remove" || removal["result"] != "success" || removal["outcome"] != "completed" || native.queries != 2 {
				t.Fatal("raw saved removal changed its public contract or started native discovery", removed)
			}
			for _, path := range []string{before.TargetLocator, filepath.Join(native.project, ".kimi-code", "skills", "pdf")} {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatal("raw removal left selected owned/native path", path, err)
				}
			}
			foreignAfter, err := os.ReadFile(foreignMCP)
			if err != nil || string(foreignAfter) != string(foreignBefore) {
				t.Fatal("raw lifecycle changed foreign global MCP", err)
			}
			for _, installation := range journey.state(t).Installations {
				for _, binding := range installation.Clients {
					if binding.ClientID == "kimi" {
						t.Fatal("raw removal left active client")
					}
				}
			}
			if native.registrations != 1 {
				t.Fatal("repeat reregistered existing exact-root workspace")
			}
		})
	}
}

func TestKimiRawProjectSkillsWithoutHTTPAuthorityRemainPrepared(t *testing.T) {
	_, journey, source, _ := managementMutableWebsite(t)
	native := kimiRawJourney(t, &journey, "2.5.2", false)
	journey.run(t, "skills", "add", source, "--skill", "pdf", "--target", "kimi", "--no-ui", "--yes", "--scope", "project", "--project-root", native.project, "--format", "json")
	if native.queries != 0 || savedJourneyBinding(t, journey.state(t), "kimi").Activation != domain.ActivationPrepared {
		t.Fatal("missing HTTP authority fabricated native completion")
	}
}
