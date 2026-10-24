package grok

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
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func projectTestRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func projectTestWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func projectTestRequest(t *testing.T) domain.ActivationRequest {
	t.Helper()
	base := projectTestRoot(t)
	project, user := filepath.Join(base, "A"), filepath.Join(base, "home", ".grok")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	projectTestWrite(t, filepath.Join(user, "keep.txt"), "original user")
	target, err := domain.NewTargetContext(domain.ScopeProject, project, user)
	if err != nil {
		t.Fatal(err)
	}
	native := filepath.Join(project, ".grok")
	active := filepath.Join(native, "plugins", "demo-0123456789ab")
	client := domain.DetectedClient{ClientID: domain.ClientGrok, ConfigRoot: user, ExecutablePath: "/inert/grok-never-executed"}
	return domain.ActivationRequest{Client: client, DeclaredName: "demo", BackendExecutable: client.ExecutablePath, Plan: domain.DeliveryPlan{ClientID: domain.ClientGrok, Scope: domain.ScopeProject, TargetContext: target, Status: domain.PlanReady, DeclaredName: "demo", DeclaredVersion: "1.0.0", NativeRegistryRoot: native, NativeRegistryExecutable: client.ExecutablePath, TargetAnchor: native, TargetRoot: filepath.Dir(active), ActivePath: active, PhysicalArtifactID: filepath.Base(active), Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative}}}, Delivery: domain.StagedDelivery{ClientID: domain.ClientGrok, ActivePath: active, OwnedBase: filepath.Dir(active)}}
}

type projectTestRunner struct {
	request domain.ActivationRequest
	calls   int
	enabled bool
	bad     string
}

func (runner *projectTestRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	plan := runner.request.Plan
	if !reflect.DeepEqual(command.Argv, []string{runner.request.Client.ExecutablePath, "inspect", "--json"}) || command.Dir != plan.TargetContext.ProjectRoot || command.StdoutLimitBytes != 1<<20 {
		return legacyports.CommandResult{}, os.ErrPermission
	}
	body, _ := json.Marshal(map[string]any{"grokVersion": "1.0.67", "cwd": command.Dir, "projectRoot": command.Dir + string(filepath.Separator), "projectTrusted": false, "skills": []any{}, "plugins": []any{map[string]any{"name": "demo", "scope": "project", "path": plan.ActivePath, "enabled": runner.enabled}}})
	if runner.bad != "" {
		body = []byte(runner.bad)
	}
	return legacyports.CommandResult{Stdout: body}, nil
}
func TestProjectGrokPreparedForeverAndNoGlobalCommand(t *testing.T) {
	request := projectTestRequest(t)
	projectTestWrite(t, filepath.Join(request.Plan.ActivePath, ".grok-plugin", "plugin.json"), `{"name":"demo"}`)
	for _, enabled := range []bool{false, true} {
		runner := &projectTestRunner{request: request, enabled: enabled}
		env := clients.Env{Runner: runner}
		out, err := New().Activate(t.Context(), env, request)
		if err != nil || out.Activation != domain.ActivationPrepared || out.Verification != domain.VerificationPackageValid || out.AuthoritativeObservation || runner.calls != 1 {
			t.Fatalf("source evidence promoted enabled%t: %+v %v calls%d", enabled, out, err, runner.calls)
		}
		if New().AutomaticallyActivates(env, request) || New().VerifierAvailable(request.Client, request.Plan, request.BackendExecutable) {
			t.Fatal("prepared source claimed activation verifier")
		}
	}
	request.ActivationComplete = true
	runner := &projectTestRunner{request: request}
	if _, err := New().Activate(t.Context(), clients.Env{Runner: runner}, request); err == nil || runner.calls != 0 {
		t.Fatal("attestation reached native process or promoted prepared source")
	}
	early := request
	early.ActivationComplete = false
	early.Delivery = domain.StagedDelivery{}
	if err := New().PreflightActivation(clients.Env{}, early); err != nil {
		t.Fatal("unstaged preflight refused", err)
	}
	if body, _ := os.ReadFile(filepath.Join(request.Client.ConfigRoot, "keep.txt")); string(body) != "original user" {
		t.Fatal("original user changed")
	}
}
func TestProjectGrokStrictObservedInspectShape(t *testing.T) {
	request := projectTestRequest(t)
	runner := &projectTestRunner{request: request}
	command := legacyports.Command{Argv: []string{request.Client.ExecutablePath, "inspect", "--json"}, Dir: request.Plan.TargetContext.ProjectRoot, StdoutLimitBytes: 1 << 20}
	result, err := runner.Run(t.Context(), command)
	if err != nil {
		t.Fatal(err)
	}
	body := string(result.Stdout)
	project := request.Plan.TargetContext.ProjectRoot
	for _, candidate := range []string{body, strings.Replace(body, `"projectRoot":"`+project+string(filepath.Separator)+`"`, `"projectRoot":"`+project+`"`, 1)} {
		if _, err := parseProjectInspect([]byte(candidate), project); err != nil {
			t.Fatal("observed root spelling refused", err)
		}
	}
	invalid := []string{strings.Replace(body, `"grokVersion":"1.0.67"`, `"grokVersion":"1.0.68"`, 1), strings.Replace(body, `"cwd":`, `"Cwd":`, 1), strings.Replace(body, `"projectRoot":`, `"ProjectRoot":`, 1), strings.Replace(body, `"enabled":false`, `"enabled":null`, 1), body + ` {}`, strings.Replace(body, `"skills":[]`, `"skills":null`, 1), strings.Replace(body, `"name":"demo"`, `"name":"demo","Name":"demo"`, 1)}
	for _, suffix := range []string{"//", "/./", "/../A/"} {
		invalid = append(invalid, strings.Replace(body, `"projectRoot":"`+project+string(filepath.Separator)+`"`, `"projectRoot":"`+project+suffix+`"`, 1))
	}
	for _, candidate := range invalid {
		if _, err := parseProjectInspect([]byte(candidate), project); err == nil {
			t.Fatalf("ambiguous native source accepted: %s", candidate)
		}
	}
}
func TestProjectGrokMalformedContextCannotReachUserLifecycle(t *testing.T) {
	for _, name := range []string{"both user labels", "other native", "other artifact", "other active", "delivery client", "attestation", "custom home"} {
		t.Run(name, func(t *testing.T) {
			request := projectTestRequest(t)
			switch name {
			case "both user labels":
				request.Plan.Scope = domain.ScopeUser
				request.Plan.TargetContext.Scope = domain.ScopeUser
			case "other native":
				request.Plan.NativeRegistryRoot = request.Client.ConfigRoot
			case "other artifact":
				request.Plan.PhysicalArtifactID = "foreign"
			case "other active":
				request.Plan.ActivePath = filepath.Join(request.Client.ConfigRoot, "plugins", "foreign")
			case "delivery client":
				request.Delivery.ClientID = domain.ClientClaude
			case "attestation":
				request.ActivationComplete = true
			case "custom home":
				request.Client.Surfaces = []domain.ClientSurface{{ID: "grok_custom_home", Detected: true}}
			}
			runner := &projectTestRunner{request: request}
			if _, err := New().Activate(t.Context(), clients.Env{Runner: runner}, request); err == nil || runner.calls != 0 {
				t.Fatalf("malformed project reached lifecycle: %v calls%d", err, runner.calls)
			}
		})
	}
}
func TestProjectGrokFiniteCollisionSourcesAndBudgets(t *testing.T) {
	for _, name := range []string{"user grok", "user claude", "ancestor grok", "selected claude", "direct skill", "unknown layout", "folded name", "oversized", "aggregate", "entries"} {
		t.Run(name, func(t *testing.T) {
			request := projectTestRequest(t)
			root := request.Plan.NativeRegistryRoot
			switch name {
			case "user grok":
				root = request.Client.ConfigRoot
			case "user claude":
				root = filepath.Join(filepath.Dir(request.Client.ConfigRoot), ".claude")
			case "ancestor grok":
				root = filepath.Join(filepath.Dir(request.Plan.TargetContext.ProjectRoot), ".grok")
			case "selected claude":
				root = filepath.Join(request.Plan.TargetContext.ProjectRoot, ".claude")
			}
			manifest := filepath.Join(root, "plugins", "foreign", ".grok-plugin", "plugin.json")
			body := `{"name":"demo"}`
			switch name {
			case "direct skill":
				projectTestWrite(t, filepath.Join(root, "skills", "demo", "SKILL.md"), "inert")
			case "unknown layout":
				projectTestWrite(t, filepath.Join(root, "plugins", "foreign", "unknown.txt"), "inert")
			case "folded name":
				projectTestWrite(t, manifest, `{"Name":"demo"}`)
			case "oversized":
				projectTestWrite(t, manifest, strings.Repeat(" ", projectManifestLimit+1))
			case "aggregate":
				for i := 0; i < 17; i++ {
					projectTestWrite(t, filepath.Join(root, "plugins", string(rune('a'+i)), ".grok-plugin", "plugin.json"), `{"name":"other"}`+strings.Repeat(" ", projectManifestLimit-len(`{"name":"other"}`)))
				}
			case "entries":
				for i := 0; i < 129; i++ {
					projectTestWrite(t, filepath.Join(root, "plugins", string(rune(1000+i)), ".grok-plugin", "plugin.json"), `{"name":"other"}`)
				}
			default:
				projectTestWrite(t, manifest, body)
			}
			finding, err := inspectProjectFiles(request.Plan, false)
			collision := name == "user grok" || name == "user claude" || name == "ancestor grok" || name == "selected claude" || name == "direct skill"
			if collision {
				if err != nil || finding != clients.RegistryCollision {
					t.Fatalf("foreign source overlooked: %s %v %v", name, finding, err)
				}
			} else if err == nil || finding != clients.RegistryIndeterminate {
				t.Fatalf("incomplete bounded discovery became available: %s %v %v", name, finding, err)
			}
		})
	}
}
func TestProjectGrokProjectionRejectsForeignStagingBeforeWrites(t *testing.T) {
	for _, kind := range []string{"user", "other", "outside", "nonclean"} {
		t.Run(kind, func(t *testing.T) {
			request := projectTestRequest(t)
			if err := os.MkdirAll(request.Plan.TargetAnchor, 0700); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(request.Plan.TargetAnchor, ".agentplugins-staging-fixture")
			if kind == "nonclean" {
				stage = request.Plan.TargetAnchor + string(filepath.Separator) + "x" + string(filepath.Separator) + ".." + string(filepath.Separator) + ".agentplugins-staging-fixture"
			} else {
				target := projectTestRoot(t)
				if kind == "user" {
					target = request.Client.ConfigRoot
				}
				if err := os.Symlink(target, stage); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			if _, err := New().Project(t.Context(), clients.ProjectionInput{Plan: request.Plan, StagingPath: stage}); err == nil {
				t.Fatal("foreign staging admitted")
			}
			if body, _ := os.ReadFile(filepath.Join(request.Client.ConfigRoot, "keep.txt")); string(body) != "original user" {
				t.Fatal("user mutated")
			}
		})
	}
}

func TestProjectGrokMissingLeafCannotHideAncestorAlias(t *testing.T) {
	request := projectTestRequest(t)
	alias := filepath.Join(filepath.Dir(request.Plan.TargetContext.ProjectRoot), ".claude")
	if err := os.Symlink(projectTestRoot(t), alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if finding, err := inspectProjectFiles(request.Plan, false); err == nil || finding != clients.RegistryIndeterminate {
		t.Fatalf("missing leaf under alias became absence proof: %v %v", finding, err)
	}
}
func TestProjectGrokManifestSparseCapAndExactLimit(t *testing.T) {
	path := filepath.Join(projectTestRoot(t), "manifest.json")
	body := `{"name":"other"}`
	projectTestWrite(t, path, body+strings.Repeat(" ", projectManifestLimit-len(body)))
	scanner := projectFileScanner{remaining: projectDiscoveryLimit}
	if got, err := scanner.readProjectManifest(path, nil); err != nil || len(got) != projectManifestLimit {
		t.Fatal("exact limit refused", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(1 << 30); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := scanner.readProjectManifest(path, nil); err == nil {
		t.Fatal("sparse overlimit manifest admitted")
	}
}
