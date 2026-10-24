package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

const clineForeignConfig = `{"mcpServers":{"foreign":{"transport":{"type":"streamableHttp","url":"https://foreign.test/mcp"},"disabled":true}},"preference":"keep"}`
const clineSkill = "---\nname: docs\ndescription: Inert Cline project fixture\n---\nKIMI-PROJECT-ONE\n"

type clineProjectFixture struct {
	base, project, user, other, source, data string
	config                                   string
	request                                  domain.ActivationRequest
	envelope                                 domain.PackageEnvelope
	registry                                 *clients.Registry
	stager                                   providers.Stager
}

func newClineProjectFixture(t *testing.T, stdio bool) *clineProjectFixture {
	t.Helper()
	f := &clineProjectFixture{base: projectGeminiTemp(t)}
	f.project, f.user, f.other, f.source = filepath.Join(f.base, "A"), filepath.Join(f.base, "user"), filepath.Join(f.base, "B"), filepath.Join(f.base, "source")
	for _, root := range []string{f.project, f.user, f.other} {
		native := filepath.Join(root, ".cline")
		if root == f.user {
			native = f.user
		}
		config := filepath.Join(root, ".uap", "clients", "cline", "mcp.json")
		if root == f.user {
			config = filepath.Join(f.user, "data", "settings", "cline_mcp_settings.json")
		}
		projectGeminiWrite(t, config, clineForeignConfig)
		projectGeminiWrite(t, filepath.Join(native, "skills", "foreign", "SKILL.md"), "foreign skill\n")
		if root != f.user {
			if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	f.config = filepath.Join(f.project, ".uap", "clients", "cline", "mcp.json")

	var err error
	f.registry, err = clients.NewRegistry(cline.New())
	if err != nil {
		t.Fatal(err)
	}
	f.request.Client = domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: f.user, ExecutablePath: filepath.Join(f.base, "cline"), Version: "3.0.73", Status: domain.DetectionDetected}
	f.stager = providers.Stager{Registry: f.registry, Paths: pathpolicy.Policy{}}
	f.data = filepath.Join(f.project, ".uap", "data", "cline-exact-receipt")
	projectGeminiWrite(t, filepath.Join(f.data, "keep.txt"), "persistent data\n")
	projectGeminiWrite(t, filepath.Join(f.source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0"}`)
	projectGeminiWrite(t, filepath.Join(f.source, "skills", "docs", "SKILL.md"), clineSkill)
	mcp := `{"type":"streamable-http","url":"https://managed.test/one"}`
	if stdio {
		mcp = `{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js","${PLUGIN_DATA}"],"cwd":"${PLUGIN_ROOT}","env":{"MARKER":"${PLUGIN_DATA}/marker"}}`
		projectGeminiWrite(t, filepath.Join(f.source, "server.js"), "// Inert; never executed.\n")
		launcher := filepath.Join(f.base, "TEST-inert-launcher")
		if err := os.WriteFile(launcher, []byte("TEST trusted launcher bytes; never executed"), 0700); err != nil {
			t.Fatal(err)
		}
		f.stager.LauncherSource, err = managedstdio.NewSource(launcher, "fixture")
		if err != nil {
			t.Fatal(err)
		}
	}
	projectGeminiWrite(t, filepath.Join(f.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":`+mcp+`}}`)
	f.stage(t, "initial")
	return f
}

func (f *clineProjectFixture) stage(t *testing.T, operation string) {
	t.Helper()
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	f.envelope, err = (loader.Loader{Registry: schemas}).Load(context.Background(), domain.LoadInput{SnapshotRoot: f.source})
	if err != nil {
		t.Fatal(err)
	}
	planning := planner.Planner{ProjectRoot: f.project, ManagedRoot: filepath.Join(f.project, ".uap", "managed"), Registry: f.registry, Paths: pathpolicy.Policy{}}
	plan, err := planning.Plan(context.Background(), domain.PlanRequest{Envelope: f.envelope, Client: f.request.Client, Scope: domain.ScopeProject, ProjectRoot: f.project, PhysicalArtifactID: "demo-0123456789ab"})
	if err != nil || plan.Status != domain.PlanReady || plan.NativeRegistryRoot != f.project || plan.Activation != domain.ActivationPrepared {
		t.Fatalf("Cline repository plan = %+v, %v", plan, err)
	}
	delivery, err := f.stager.StageWithPluginData(context.Background(), f.envelope, plan, operation, domain.CompatibilityHints{}, f.data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(plan.ActivePath); err == nil {
		if err := os.Rename(plan.ActivePath, plan.ActivePath+"-"+operation+"-old"); err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	delivery.StagingPath = ""
	f.request.Plan, f.request.Delivery, f.request.DeclaredName = plan, delivery, "demo"
}

func (f *clineProjectFixture) activator(kernel nativeconfig.Kernel) providers.Activator {
	return providers.Activator{Registry: f.registry, NativeConfig: &kernel}
}

func (f *clineProjectFixture) remove(objects []domain.NativeObjectOwnership) domain.DeactivationRequest {
	return domain.DeactivationRequest{Client: f.request.Client, TargetContext: f.request.Plan.TargetContext,
		NativeRegistryRoot: f.project, PhysicalArtifactID: f.request.Plan.PhysicalArtifactID, ManagedArtifactPath: f.request.Plan.ActivePath, Confirmed: true, NativeObjects: objects}
}

func clinePrepared(t *testing.T, outcome domain.ActivationOutcome, err error) {
	t.Helper()
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.ActivationAttested {
		t.Fatalf("Cline configured state = %+v, %v", outcome, err)
	}
}
