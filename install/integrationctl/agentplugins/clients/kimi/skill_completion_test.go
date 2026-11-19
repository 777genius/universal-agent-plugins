package kimi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type kimiCompletionFixture struct {
	t             *testing.T
	request       domain.ActivationRequest
	env           clients.Env
	rows          []any
	calls         []string
	queries       int
	change        func(*http.Request, map[string]any)
	closure       error
	banner        string
	afterExchange func()
}

func newKimiCompletionFixture(t *testing.T, version string) *kimiCompletionFixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, project := filepath.Join(base, "home"), filepath.Join(base, "project")
	profile, native := filepath.Join(home, ".kimi-code"), filepath.Join(project, ".kimi-code")
	active := filepath.Join(project, ".uap", "managed", "clients", "kimi", "demo-0123456789ab")
	doc := "---\nname: docs\ndescription: Owned Kimi instructions\n---\nInert fixture.\n"
	write(t, filepath.Join(active, "skills", "docs", "SKILL.md"), doc)
	write(t, filepath.Join(profile, "config.toml"), "telemetry = false\n")
	write(t, filepath.Join(profile, "mcp.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.invalid/mcp"}}}`)
	write(t, filepath.Join(project, ".git", "marker"), "inert")
	executable := filepath.Join(base, "kimi")
	if err := os.WriteFile(executable, []byte("inert fixture, never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientKimi, Scope: domain.ScopeProject, Status: domain.PlanReady, InstallIntent: domain.InstallIntentAutomatic,
		TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: profile}, Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotChecked,
		Warnings: []string{"authentication_not_catalog_verified"}, PhysicalArtifactID: "demo-0123456789ab", DeclaredName: "demo", ActivePath: active, TargetRoot: filepath.Dir(active), NativeRegistryRoot: native,
		Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative}}, UserActions: []string{KimiProjectAction}}
	object, err := kimiComponents(native).SkillObject("docs", "skills/docs", active)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), active)
	if err != nil {
		t.Fatal(err)
	}
	packageObject := domain.NativeObjectOwnership{ObjectID: "package:kimi:" + plan.PhysicalArtifactID, Kind: "managed_package_directory", LogicalName: "demo", Path: active, ManagedDigest: digest, ProtectionClass: "managed"}
	f := &kimiCompletionFixture{t: t}
	f.request = domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientKimi, ConfigRoot: profile, ExecutablePath: executable, Version: version, Status: domain.DetectionDetected}, Plan: plan, DeclaredName: "demo",
		Delivery:      domain.StagedDelivery{ClientID: domain.ClientKimi, ActivePath: active, OwnedBase: plan.TargetRoot, ArtifactDigest: digest, NativeObjects: []domain.NativeObjectOwnership{packageObject, object}},
		NativeAttempt: domain.NativeAttemptIdentity{BindingID: "client_existing_stable_identity"}}
	f.env = clients.Env{UserHome: home, NativeConfig: nativeconfig.New(), Snapshots: packagesnapshot.Builder{}, ParseSkill: conformance.ParseSkill, Runner: f, NativeHTTP: f}
	return f
}

func (f *kimiCompletionFixture) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{NoChildStarted: true}, fmt.Errorf("ordinary command must not replace completion")
}

func (f *kimiCompletionFixture) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	f.queries++
	want := []string{f.request.Client.ExecutablePath, "web", "--no-open", "--host", "127.0.0.1", "--port", "0"}
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 30*time.Second || !reflect.DeepEqual(command.Argv, want) || command.Dir != f.request.Plan.TargetContext.ProjectRoot || command.StdoutLimitBytes != 1<<20 {
		f.t.Fatal("native completion command authority or bounds changed", command)
	}
	write(f.t, filepath.Join(f.request.Client.ConfigRoot, "server.token"), strings.Repeat("A", 43))
	banner := "  native  Kimi server ready  " + f.request.Client.Version + "\n  Local:    http://127.0.0.1:12345/#token=" + strings.Repeat("A", 43) + "\n  Token:    " + strings.Repeat("A", 43) + "\n"
	if f.banner != "" {
		banner = f.banner
	}
	if err := exchange(io.Discard, strings.NewReader(banner)); err != nil {
		return err
	}
	if f.afterExchange != nil {
		f.afterExchange()
	}
	return f.closure
}

func (f *kimiCompletionFixture) workspace() map[string]any {
	row := map[string]any{"id": "wd_project_0123456789ab", "root": f.request.Plan.TargetContext.ProjectRoot, "name": "project", "created_at": "2026-11-19T00:00:00Z", "last_opened_at": "2026-11-19T00:00:00Z", "session_count": 0}
	if f.request.Client.Version == "2.5.2" {
		row["id"], row["kind"], row["additional_dirs"] = "wm_01234567-0123-4567-89ab-0123456789ab", "managed", []any{}
	}
	return row
}

func (f *kimiCompletionFixture) RoundTrip(request *http.Request) (*http.Response, error) {
	f.calls = append(f.calls, request.Method+" "+request.URL.Path)
	if request.URL.Host != "127.0.0.1:12345" || request.Header.Get("Authorization") != "Bearer "+strings.Repeat("A", 43) {
		f.t.Fatal("native bearer origin mismatch")
	}
	var data map[string]any
	switch request.Method + " " + request.URL.Path {
	case "GET /api/v1/meta":
		data = map[string]any{"server_version": f.request.Client.Version, "dangerous_bypass_auth": false, "backend": "v2", "server_id": "inert-owned-service"}
	case "GET /api/v1/workspaces":
		data = map[string]any{"items": f.rows}
		if f.rows == nil {
			data["items"] = []any{}
		}
	case "POST /api/v1/workspaces":
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			f.t.Fatal(err)
		}
		if body["root"] != f.request.Plan.TargetContext.ProjectRoot || (f.request.Client.Version == "2.5.2" && body["idempotency_key"] != f.request.NativeAttempt.BindingID) || (f.request.Client.Version == "2.3.2" && len(body) != 1) {
			f.t.Fatal("native workspace protocol version/identity differs", body)
		}
		data = f.workspace()
		f.rows = []any{data}
	case "POST /api/v1/shutdown":
		data = map[string]any{"ok": true}
	default:
		if request.Method != "GET" || !strings.HasSuffix(request.URL.Path, "/skills") {
			f.t.Fatal("forbidden native endpoint", request.Method, request.URL.Path)
		}
		data = map[string]any{"skills": []any{map[string]any{"name": "docs", "description": "Owned Kimi instructions", "source": "project", "path": filepath.Join(f.request.Plan.NativeRegistryRoot, "skills", "docs", "SKILL.md")}}}
	}
	if f.change != nil {
		f.change(request, data)
	}
	body, err := json.Marshal(map[string]any{"code": 0, "msg": "success", "request_id": "inert-request", "data": data})
	if err != nil {
		f.t.Fatal(err)
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
}

func TestKimiProjectPureSkillsRequireNativeCompletionAndNaturalClosure(t *testing.T) {
	for _, version := range []string{"2.3.2", "2.5.2"} {
		t.Run(version, func(t *testing.T) {
			f := newKimiCompletionFixture(t, version)
			before, err := os.ReadFile(filepath.Join(f.request.Client.ConfigRoot, "mcp.json"))
			if err != nil {
				t.Fatal(err)
			}
			out, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || out.Activation != domain.ActivationActive || out.Authentication != domain.AuthenticationNotRequired || len(out.UserActions) != 0 || f.queries != 1 {
				t.Fatalf("native completion=%+v err=%v calls=%v", out, err, f.calls)
			}
			f.request.PreviousNativeObjects = out.NativeObjects
			f.request.VerifyOnly = true
			observed, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || observed.Activation != domain.ActivationPrepared || f.queries != 1 {
				t.Fatal("VerifyOnly started a native child", observed, err)
			}
			f.request.VerifyOnly = false
			f.request.Replacing = true
			if !New().CanCompletePreparedSetup(f.request, observed) {
				t.Fatal("confirmed completion classifier refused intact Skills")
			}
			out, err = New().Activate(t.Context(), f.env, f.request)
			if err != nil || out.Activation != domain.ActivationActive || f.queries != 2 {
				t.Fatal("repeat did not complete", out, err)
			}
			posts := 0
			for _, call := range f.calls {
				if call == "POST /api/v1/workspaces" {
					posts++
				}
			}
			if posts != 1 {
				t.Fatal("repeat touched existing workspace", f.calls)
			}
			after, _ := os.ReadFile(filepath.Join(f.request.Client.ConfigRoot, "mcp.json"))
			if string(before) != string(after) {
				t.Fatal("foreign global MCP changed")
			}
		})
	}
}

func TestKimiNativeCompletionRetainsUnavailableAndNegativeEvidence(t *testing.T) {
	for _, condition := range []string{"verify", "old-version", "no-http", "no-parser", "no-snapshot", "unknown-auth", "explicit-auth", "wrong-path", "duplicate-workspace", "additional-dirs", "local-policy", "shutdown-refused", "wrong-version", "unclosed"} {
		t.Run(condition, func(t *testing.T) {
			f := newKimiCompletionFixture(t, "2.5.2")
			switch condition {
			case "verify":
				f.request.VerifyOnly = true
			case "old-version":
				f.request.Client.Version = "2.3.1"
			case "no-http":
				f.env.NativeHTTP = nil
			case "no-parser":
				f.env.ParseSkill = nil
			case "no-snapshot":
				f.env.Snapshots = nil
			case "unknown-auth":
				f.request.Plan.Warnings = append(f.request.Plan.Warnings, "authentication_requirement_unknown")
			case "explicit-auth":
				f.request.Plan.Authentication = domain.AuthenticationPending
			case "duplicate-workspace":
				f.rows = []any{f.workspace(), f.workspace()}
			case "additional-dirs":
				row := f.workspace()
				row["additional_dirs"] = []any{t.TempDir()}
				f.rows = []any{row}
			case "local-policy":
				write(t, filepath.Join(f.request.Plan.NativeRegistryRoot, "local.toml"), "[workspace]\nadditional_dir = ['../B']\n")
			case "unclosed":
				f.closure = &legacyports.ProcessClosureError{Cause: fmt.Errorf("inert unsettled process")}
			}
			f.change = func(r *http.Request, data map[string]any) {
				if condition == "wrong-path" && strings.HasSuffix(r.URL.Path, "/skills") {
					data["skills"].([]any)[0].(map[string]any)["path"] = "/foreign/SKILL.md"
				}
				if condition == "wrong-version" && strings.HasSuffix(r.URL.Path, "/meta") {
					data["server_version"] = "unknown"
				}
				if condition == "shutdown-refused" && strings.HasSuffix(r.URL.Path, "/shutdown") {
					data["ok"] = false
				}
			}
			out, err := New().Activate(t.Context(), f.env, f.request)
			if condition == "unknown-auth" || condition == "explicit-auth" {
				if err != nil || out.Authentication != f.request.Plan.Authentication {
					t.Fatal("authentication boundary changed", out, err)
				}
				return
			}
			if condition == "unclosed" {
				if !domain.IsUnclosedClientProbe(err) || out.NativeEffect != domain.NativeEffectUncertain {
					t.Fatal("closure uncertainty lost", out, err)
				}
				return
			}
			if out.Activation == domain.ActivationActive {
				t.Fatal("unqualified native proof promoted", condition, out, err)
			}
		})
	}
}
