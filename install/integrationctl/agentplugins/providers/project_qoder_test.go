package providers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/qoder"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const qoderView = ".agentplugins-qoder-native"
const qoderProjectSkill = "---\nname: docs\ndescription: Inert Qoder project fixture\n---\nPinned original.\n"

type qoderForbiddenRunner struct{ calls int }

func (r *qoderForbiddenRunner) Run(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, fmt.Errorf("forbidden Qoder native command: %v", c.Argv)
}

type qoderProjectFixture struct {
	base, project, user, other, source string
	envelope                           domain.PackageEnvelope
	registry                           *clients.Registry
	stager                             providers.Stager
	request                            domain.ActivationRequest
}

func newQoderProjectFixture(t *testing.T, stage bool) *qoderProjectFixture {
	t.Helper()
	f := &qoderProjectFixture{base: projectGeminiTemp(t)}
	f.project, f.user, f.other, f.source = filepath.Join(f.base, "A"), filepath.Join(f.base, "home", ".qoder"), filepath.Join(f.base, "B"), filepath.Join(f.base, "source")
	for _, root := range []string{f.project, f.user, f.other} {
		projectGeminiWrite(t, filepath.Join(root, "keep"), "foreign preserved")
	}
	projectGeminiWrite(t, filepath.Join(f.source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0"}`)
	projectGeminiWrite(t, filepath.Join(f.source, "skills", "docs", "SKILL.md"), qoderProjectSkill)
	projectGeminiWrite(t, filepath.Join(f.source, "skills", "docs", "assets", "reference.txt"), "selected asset")
	projectGeminiWrite(t, filepath.Join(f.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"http://127.0.0.1:9/never"}}}`)
	for _, aux := range []string{".mcp.json", "hooks/hooks.json", "agents/raw.md", ".qoder-plugin/plugin.json"} {
		projectGeminiWrite(t, filepath.Join(f.source, filepath.FromSlash(aux)), "raw source; never import root")
	}
	projectGeminiWrite(t, filepath.Join(f.source, filepath.FromSlash(commandext.RelativeDirectory), "echo.md"), "---\ndescription: Echo text\ndisable-model-invocation: false\n---\nQODER-COMMAND\n")
	projectGeminiWrite(t, filepath.Join(f.source, filepath.FromSlash(commandext.RelativeDirectory), "wizard.md"), "---\ndescription: Explicit only\ndisable-model-invocation: true\n---\nWITHHELD\n")
	var err error
	f.registry, err = clients.NewRegistry(qoder.New())
	if err != nil {
		t.Fatal(err)
	}
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	f.envelope, err = (loader.Loader{Registry: schemas}).Load(t.Context(), domain.LoadInput{SnapshotRoot: f.source})
	if err != nil {
		t.Fatal(err)
	}
	f.request.Client = domain.DetectedClient{ClientID: domain.ClientQoder, ConfigRoot: f.user, ExecutablePath: "/inert/qoder-must-never-execute", Status: domain.DetectionDetected}
	planning := planner.Planner{ProjectRoot: f.project, ManagedRoot: filepath.Join(f.project, ".uap", "managed"), Paths: pathpolicy.Policy{}, Registry: f.registry}
	f.request.Plan, err = planning.Plan(t.Context(), domain.PlanRequest{Envelope: f.envelope, Client: f.request.Client, Scope: domain.ScopeProject, ProjectRoot: f.project, PhysicalArtifactID: "demo-0123456789ab"})
	if err != nil || f.request.Plan.Status != domain.PlanReady {
		t.Fatalf("Qoder project plan: %+v %v", f.request.Plan, err)
	}
	f.request.DeclaredName = "demo"
	f.stager = providers.Stager{Registry: f.registry, Paths: pathpolicy.Policy{}}
	if stage {
		f.commit(t)
	}
	return f
}
func (f *qoderProjectFixture) commit(t *testing.T) {
	t.Helper()
	delivery, err := f.stager.Stage(t.Context(), f.envelope, f.request.Plan, "qoder-stage", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(delivery.StagingPath) != f.request.Plan.TargetRoot {
		t.Fatal("wrong staging base")
	}
	if err := os.Rename(delivery.StagingPath, delivery.ActivePath); err != nil {
		t.Fatal(err)
	}
	delivery.StagingPath = ""
	f.request.Delivery = delivery
}
func qoderPrepared(t *testing.T, out domain.ActivationOutcome, err error) {
	t.Helper()
	if err != nil || out.Activation != domain.ActivationPrepared || out.Verification != domain.VerificationInstalled || out.ActivationAttested || out.NativeEffect != domain.NativeEffectUnchanged || len(out.NativeObjects) != 0 {
		t.Fatalf("Qoder Prepared file evidence: %+v %v", out, err)
	}
}
func TestProjectQoderClosedViewAndNeutralDiscovery(t *testing.T) {
	f := newQoderProjectFixture(t, true)
	plan := f.request.Plan
	if plan.TargetAnchor != filepath.Join(f.project, ".uap", "managed") || plan.TargetRoot != filepath.Join(plan.TargetAnchor, "clients", "qoder") || plan.NativeRegistryRoot != plan.TargetRoot || plan.NativeRegistryExecutable != "" {
		t.Fatalf("wrong exact layout %+v", plan)
	}
	for _, c := range plan.Components {
		if c.Kind == domain.ComponentMCPServer && c.Support != domain.SupportUnsupported {
			t.Fatal("MCP promoted")
		}
	}
	view := filepath.Join(plan.ActivePath, qoderView)
	entries, err := os.ReadDir(view)
	if err != nil || len(entries) != 2 {
		t.Fatalf("view root: %v %v", entries, err)
	}
	projectGeminiAssert(t, filepath.Join(view, "skills", "docs", "SKILL.md"), qoderProjectSkill)
	projectGeminiAssert(t, filepath.Join(view, "skills", "docs", "assets", "reference.txt"), "selected asset")
	if body, err := os.ReadFile(filepath.Join(view, "skills", "demo-echo", "SKILL.md")); err != nil || !strings.Contains(string(body), "QODER-COMMAND") {
		t.Fatalf("command skill %s %v", body, err)
	}
	for _, path := range []string{".mcp.json", "mcp.json", "hooks", "agents", "skills/demo-wizard"} {
		if _, err := os.Lstat(filepath.Join(view, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("unqualified native path %s: %v", path, err)
		}
	}
	projectGeminiAssert(t, filepath.Join(plan.ActivePath, ".mcp.json"), "raw source; never import root")
	projectGeminiAssert(t, filepath.Join(f.source, "hooks", "hooks.json"), "raw source; never import root")
	runner := &qoderForbiddenRunner{}
	actor := providers.Activator{Registry: f.registry, Runner: runner}
	out, err := actor.Activate(t.Context(), f.request)
	qoderPrepared(t, out, err)
	actions := strings.Join(out.UserActions, " ")
	for _, want := range []string{view, "--scope", "shared native registry/cache", "same-name/same-version", "independent", "does not unregister"} {
		if !strings.Contains(actions, want) {
			t.Fatalf("missing native limit %q: %s", want, actions)
		}
	}
	var command struct {
		WorkingDirectory string   `json:"working_directory"`
		Argv             []string `json:"argv"`
	}
	encoded := strings.TrimPrefix(out.UserActions[1], "Optional separate native import (UAP does not execute this command): ")
	if err := json.Unmarshal([]byte(encoded), &command); err != nil || command.WorkingDirectory != f.project || !reflect.DeepEqual(command.Argv, []string{"qoder", "plugins", "install", view, "--scope", "project", "--json"}) {
		t.Fatalf("manual argv changed: %+v %v", command, err)
	}
	binding := &domain.ClientBinding{ClientID: "qoder", Scope: "project", PhysicalArtifact: plan.PhysicalArtifactID, TargetLocator: plan.ActivePath, NativeProfileRoot: plan.NativeRegistryRoot, NativeObjects: f.request.Delivery.NativeObjects}
	for _, enabled := range []bool{false, true} {
		var nativeRunner *qoderForbiddenRunner
		if enabled {
			nativeRunner = runner
		}
		observer := providers.NativeIdentityObserver{Registry: f.registry, Stager: f.stager}
		if nativeRunner != nil {
			observer.Runner = nativeRunner
		}
		observed, err := observer.ObserveNativeIdentity(t.Context(), f.request.Client, plan, binding)
		if err != nil || observed.State != domain.NativeIdentityManaged || !observed.ReceiptReconciled || observed.NativeDiscoveryAttempted || observed.NativeDiscoveryReconciled || observed.NativeDiscoveryState != domain.NativeIdentityAbsent {
			t.Fatalf("enabled%t package became native discovery: %+v %v", enabled, observed, err)
		}
	}
	if qoder.New().AutomaticallyActivates(clients.Env{Runner: runner}, f.request) || !qoder.New().VerifierAvailable(f.request.Client, plan, "") || runner.calls != 0 {
		t.Fatal("project used native availability helper")
	}
}
func TestProjectQoderRejectsMalformedAuthorityBeforeNativeEffects(t *testing.T) {
	for _, bad := range []string{"attestation", "both-user", "client", "profile", "root", "native-executable", "active", "delivery-client", "empty-delivery", "missing-receipt", "foreign-receipt", "physical", "no-skills"} {
		t.Run(bad, func(t *testing.T) {
			f := newQoderProjectFixture(t, true)
			before := projectGeminiInventory(t, f.base)
			r := f.request
			switch bad {
			case "attestation":
				r.ActivationComplete = true
			case "both-user":
				r.Plan.Scope, r.Plan.TargetContext.Scope = domain.ScopeUser, domain.ScopeUser
			case "client":
				r.Client.ClientID = domain.ClientCodex
			case "profile":
				r.Plan.TargetContext.UserConfigRoot = f.other
			case "root":
				r.Plan.NativeRegistryRoot = f.user
			case "native-executable":
				r.Plan.NativeRegistryExecutable = r.Client.ExecutablePath
			case "active":
				r.Delivery.ActivePath = f.other
			case "delivery-client":
				r.Delivery.ClientID = domain.ClientClaude
			case "empty-delivery":
				r.Delivery = domain.StagedDelivery{}
			case "missing-receipt":
				r.Delivery.NativeObjects = nil
			case "foreign-receipt":
				r.Delivery.NativeObjects = append([]domain.NativeObjectOwnership(nil), r.Delivery.NativeObjects...)
				r.Delivery.NativeObjects[0].Path = f.user
			case "physical":
				r.Plan.PhysicalArtifactID = "other"
			case "no-skills":
				r.Plan.Components = nil
			}
			runner := &qoderForbiddenRunner{}
			if _, err := qoder.New().Activate(t.Context(), clients.Env{Runner: runner}, r); err == nil {
				t.Fatal("malformed direct adapter request admitted")
			}
			if runner.calls != 0 || !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
				t.Fatal("refused request caused an effect")
			}
		})
	}
	f := newQoderProjectFixture(t, false)
	if err := qoder.New().PreflightActivation(clients.Env{}, f.request); err != nil {
		t.Fatal("valid early unstaged request refused", err)
	}
	f.request.ActivationComplete = true
	if err := qoder.New().PreflightActivation(clients.Env{}, f.request); err == nil {
		t.Fatal("early attestation admitted")
	}
}
func TestProjectQoderProjectionCollisionAndTamperRefuse(t *testing.T) {
	f := newQoderProjectFixture(t, false)
	projectGeminiWrite(t, filepath.Join(f.source, qoderView, "keep"), "foreign reserved surface")
	before := projectGeminiInventory(t, f.user)
	if _, err := f.stager.Stage(t.Context(), f.envelope, f.request.Plan, "reserved", domain.CompatibilityHints{}); err == nil {
		t.Fatal("reserved source view adopted")
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, f.user)) {
		t.Fatal("reserved refusal touched user")
	}
	for _, tamper := range []string{"skill", "extra-root", "mcp-manifest", "receipt-digest"} {
		t.Run(tamper, func(t *testing.T) {
			f := newQoderProjectFixture(t, true)
			switch tamper {
			case "skill":
				projectGeminiWrite(t, filepath.Join(f.request.Plan.ActivePath, qoderView, "skills", "docs", "SKILL.md"), "foreign")
			case "extra-root":
				projectGeminiWrite(t, filepath.Join(f.request.Plan.ActivePath, qoderView, ".mcp.json"), "{}")
			case "mcp-manifest":
				projectGeminiWrite(t, filepath.Join(f.request.Plan.ActivePath, qoderView, ".qoder-plugin", "plugin.json"), `{"name":"demo","version":"1.0.0","skills":"./skills/","mcpServers":"../.mcp.json"}`)
			case "receipt-digest":
				f.request.Delivery.ArtifactDigest = "sha256:foreign"
			}
			// Independently exercise the closed-surface verifier even if a direct
			// caller supplies a digest for the modified package. SDK ownership
			// admission separately refuses any rewrite of its recorded digest.
			if tamper == "extra-root" || tamper == "mcp-manifest" {
				digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), f.request.Plan.ActivePath)
				if err != nil {
					t.Fatal(err)
				}
				f.request.Delivery.ArtifactDigest = digest
				f.request.Delivery.NativeObjects[0].ManagedDigest = digest
			}
			before := projectGeminiInventory(t, f.base)
			if _, err := qoder.New().Activate(t.Context(), clients.Env{}, f.request); err == nil {
				t.Fatal("tampered bundle verified")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
				t.Fatal("tampered bundle changed")
			}
		})
	}
}
