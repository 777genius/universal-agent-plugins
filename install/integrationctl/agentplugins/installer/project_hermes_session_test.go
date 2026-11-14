package installer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"syscall"
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
	renameDenied      bool
	original          func()
}

func (runner *hermesSessionSwapRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	var expected []string
	switch runner.calls {
	case 1:
		expected = []string{"--version"}
	case 2:
		expected = []string{"config", "path"}
	case 3:
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
	if runner.calls == 1 {
		output = []byte("Hermes Agent vunknown (2026.9.24)\nInstall directory: " + filepath.Join(filepath.Dir(filepath.Dir(command.Argv[0])), "hermes-agent") + "\n")
	}
	if runner.calls == 3 {
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
			runner.requireRenameDenial(err, parent, parent+"-original", syscall.Errno(5))
			return
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
		runner.requireRenameDenial(err, runner.root, runner.root+"-original", syscall.Errno(32))
		return
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

func (runner *hermesSessionSwapRunner) requireRenameDenial(err error, source, destination string, code syscall.Errno) {
	runner.t.Helper()
	var failure *os.LinkError
	if runtime.GOOS != "windows" || !errors.As(err, &failure) || failure.Op != "rename" ||
		failure.Old != source || failure.New != destination || reflect.TypeOf(failure.Err) != reflect.TypeFor[syscall.Errno]() ||
		!errors.Is(failure.Err, code) || runner.original == nil {
		runner.t.Fatal("unexpected native-boundary rename error", err)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		runner.t.Fatal("denied rename created its destination", err)
	}
	runner.original()
	runner.renameDenied = true
}

func hermesSessionOriginalAuthority(t *testing.T, root, statePath string) func() {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, 3+len(entries))
	paths = append(paths, filepath.Dir(root), root, statePath)
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatal("unexpected session fixture directory")
		}
		paths = append(paths, filepath.Join(root, entry.Name()))
	}
	infos, bodies := map[string]os.FileInfo{}, map[string][]byte{}
	for _, path := range paths {
		infos[path] = hermesSessionCapturedInfo(t, path)
		if !infos[path].IsDir() {
			bodies[path], err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return func() {
		t.Helper()
		for path, before := range infos {
			after := hermesSessionCapturedInfo(t, path)
			if !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatal("denied rename changed original identity or metadata", path)
			}
			if original, ok := bodies[path]; ok {
				current, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(original, current) {
					t.Fatal("denied rename changed original bytes", path, err)
				}
			}
		}
	}
}

func hermesSessionCapturedInfo(t *testing.T, path string) os.FileInfo {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if closeErr := file.Close(); err != nil || closeErr != nil {
		t.Fatal("capture original session identity", err, closeErr)
	}
	return info
}

// The facade creates real root/package receipts. Byte-identical replacement
// cannot become new physical authority after either native callback returns.
// An exact Windows kernel denial preserves the original and all observations;
// a successful replacement on any platform must still invalidate authority.
func TestProjectHermesSDKSessionRejoinsRootAndParentAfterEachNativeObservation(t *testing.T) {
	for _, parent := range []bool{false, true} {
		for _, boundary := range []int{1, 2, 3} {
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
				adapter, env := hermes.New(), clients.Env{ProjectRoots: publisher}
				plan, err := adapter.PlanSession(t.Context(), env, request)
				if err != nil {
					t.Fatal(err)
				}
				root := filepath.Join(fixture.project, ".hermes", "uap-session")
				runner := &hermesSessionSwapRunner{t: t, root: root, at: boundary, parentReplacement: parent,
					original: hermesSessionOriginalAuthority(t, root, store.Path)}
				env.Runner = runner
				verifyErr := adapter.VerifySession(t.Context(), env, plan)
				if runner.renameDenied {
					if verifyErr != nil || runner.calls != 3 {
						t.Fatalf("kernel-protected original did not complete its observations: %d %v", runner.calls, verifyErr)
					}
					runner.original()
				} else if verifyErr == nil || runner.calls != boundary {
					t.Fatalf("native root replacement was accepted or followed by another command: %d %v", runner.calls, verifyErr)
				}
				var owner domain.NativeObjectOwnership
				for _, object := range binding.NativeObjects {
					if object.Kind == "hermes_project_session_root" {
						owner = object
					}
				}
				if (publisher.VerifyProjectRoot(t.Context(), root, owner) == nil) != runner.renameDenied || !reflect.DeepEqual(before, fixture.state(t)) {
					t.Fatal("replaced root changed durable publication authority")
				}
			})
		}
	}
}
