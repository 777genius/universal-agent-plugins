package installer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/hermes"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type hermesSessionSwapRunner struct {
	t                 *testing.T
	root              string
	at, calls         int
	parentReplacement bool
}

func (runner *hermesSessionSwapRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	var expected []string
	switch runner.calls {
	case 1:
		expected = []string{"config", "path"}
	case 2:
		expected = []string{"config", "get", "mcp_servers", "--json"}
	default:
		runner.t.Fatal("native observation exceeded its command budget")
	}
	if !reflect.DeepEqual(command.Argv[1:], expected) {
		runner.t.Fatal("unexpected native observation", command.Argv)
	}
	body, err := os.ReadFile(filepath.Join(runner.root, "config.yaml"))
	if err != nil {
		runner.t.Fatal(err)
	}
	if runner.calls == runner.at {
		runner.replaceRoot()
	}
	output := []byte(filepath.Join(runner.root, "config.yaml") + "\n")
	if runner.calls == 2 {
		var source struct {
			Servers map[string]any `yaml:"mcp_servers"`
		}
		if err := yaml.Unmarshal(body, &source); err != nil {
			runner.t.Fatal(err)
		}
		output, err = json.Marshal(source.Servers)
	}
	return legacyports.CommandResult{Stdout: output, CleanupComplete: true}, err
}

func (runner *hermesSessionSwapRunner) replaceRoot() {
	runner.t.Helper()
	if runner.parentReplacement {
		parent := filepath.Dir(runner.root)
		if err := os.Rename(parent, parent+"-original"); err != nil {
			runner.t.Fatal(err)
		}
		if err := os.Mkdir(parent, 0o700); err != nil {
			runner.t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(parent+"-original", filepath.Base(runner.root)), runner.root); err != nil {
			runner.t.Fatal(err)
		}
		return // Preserve the root inode while replacing its owned parent.
	}
	entries, err := os.ReadDir(runner.root)
	if err != nil {
		runner.t.Fatal(err)
	}
	if err := os.Rename(runner.root, runner.root+"-original"); err != nil {
		runner.t.Fatal(err)
	}
	if err := os.Mkdir(runner.root, 0o700); err != nil {
		runner.t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			runner.t.Fatal("unexpected session fixture directory")
		}
		body, err := os.ReadFile(filepath.Join(runner.root+"-original", entry.Name()))
		if err != nil {
			runner.t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runner.root, entry.Name()), body, 0o600); err != nil {
			runner.t.Fatal(err)
		}
	}
}

// The facade creates real root/package receipts. Byte-identical replacement
// cannot become new physical authority after either native callback returns.
func TestProjectHermesSDKSessionRejoinsRootAndParentAfterEachNativeObservation(t *testing.T) {
	for _, parent := range []bool{false, true} {
		for _, boundary := range []int{1, 2} {
			t.Run(fmt.Sprintf("parent=%t/call=%d", parent, boundary), func(t *testing.T) {
				base, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				user, pkg := hermesConflictPackage(t, base)
				fixture := newHermesSDKFixture(t, filepath.Join(base, "project"), user, pkg)
				fixture.apply(t, installer.OpInstall, "native-boundary-install")
				binding, before := fixture.binding(t), fixture.state(t)
				store := statev2.Store{Path: filepath.Join(fixture.project, ".uap", "state-v2.json"), ProjectRoot: fixture.project}
				publisher := transaction.ProjectRootPublisher{Kernel: transaction.Kernel{StateStore: store}}
				request := clients.SessionRequest{ProjectRoot: fixture.project, Inspect: true, Bindings: []domain.ClientBinding{binding},
					Client: domain.DetectedClient{ClientID: domain.ClientHermes, ConfigRoot: fixture.user, ExecutablePath: fixture.request.ClientExecutable,
						Status: domain.DetectionDetected, Surfaces: []domain.ClientSurface{{ID: "hermes_cli", Detected: true}}}}
				adapter, env := hermes.NewProject(), clients.Env{ProjectRoots: publisher}
				plan, err := adapter.PlanSession(t.Context(), env, request)
				if err != nil {
					t.Fatal(err)
				}
				root := filepath.Join(fixture.project, ".hermes", "uap-session")
				runner := &hermesSessionSwapRunner{t: t, root: root, at: boundary, parentReplacement: parent}
				env.Runner = runner
				if err := adapter.VerifySession(t.Context(), env, plan); err == nil || runner.calls != boundary {
					t.Fatalf("native root replacement was accepted or followed by another command: %d %v", runner.calls, err)
				}
				var owner domain.NativeObjectOwnership
				for _, object := range binding.NativeObjects {
					if object.Kind == "hermes_project_session_root" {
						owner = object
					}
				}
				if publisher.VerifyProjectRoot(t.Context(), root, owner) == nil || !reflect.DeepEqual(before, fixture.state(t)) {
					t.Fatal("replaced root changed durable publication authority")
				}
			})
		}
	}
}
