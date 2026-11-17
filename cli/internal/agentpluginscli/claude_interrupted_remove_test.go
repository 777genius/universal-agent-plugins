package agentpluginscli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

// A signal between the grouped removal's state commit and its directory
// finalization left a hidden backup with the same Claude plugin name and an
// open journal. Rerunning remove reported "not installed" and add saw the
// backup as an unmanaged native identity, so neither command could recover.
func TestClaudeUserRemoveInterruptedAfterStateCommitResumesOnRerun(t *testing.T) {
	t.Parallel()
	for _, rerun := range []string{"remove", "prompted-remove", "add"} {
		t.Run(rerun, func(t *testing.T) {
			t.Parallel()
			f, skills, plugin := newInterruptedClaudeRemoveFixture(t)
			var out, stderr string
			var err error
			switch rerun {
			case "add":
				out, stderr, err = f.execute(false, "add", plugin, "--target", "claude", "--yes")
			case "prompted-remove":
				// The interactive target choice would otherwise find no bound target.
				out, stderr, err = f.execute(true, "remove", "demo")
			default:
				out, stderr, err = f.execute(false, "remove", "demo", "--target", "claude", "--yes")
			}
			if (rerun == "add") != (err == nil) {
				t.Fatalf("%s after the committed removal = %v\n%s\n%s", rerun, err, out, stderr)
			}
			if leftovers := interruptedClaudeLeftovers(t, f, skills); len(leftovers) != 0 {
				t.Fatalf("%s rerun left the interrupted transaction open: %v", rerun, leftovers)
			}
			if rerun == "add" {
				if out, stderr, err := f.execute(false, "remove", "demo", "--target", "claude", "--yes"); err != nil {
					t.Fatalf("remove after the recovered add: %v\n%s\n%s", err, out, stderr)
				}
			}
		})
	}
}

func newInterruptedClaudeRemoveFixture(t *testing.T) (cliFixture, string, string) {
	t.Helper()
	claude := fixtureClient(t, domain.ClientClaude)
	claude.ExecutablePath = filepath.Join(t.TempDir(), "claude")
	runner := &dependencyClaudeListing{root: claude.ConfigRoot}
	plugin := writeCLIPlugin(t)
	f, _ := newDependencyHarness(t, plugin, []domain.DetectedClient{claude}, installer.Config{Runner: runner, EnableNativeObserver: true})
	f.app.Lifecycle.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: providerstest.NewStager(providers.Stager{}), Runner: runner, Registry: f.app.ClientRegistry})
	f.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: runner, Registry: f.app.ClientRegistry})
	if out, stderr, err := f.execute(false, "add", plugin, "--target", "claude", "--yes"); err != nil {
		t.Fatalf("add: %v\n%s\n%s", err, out, stderr)
	}
	// Cancel exactly as SIGINT does once the backup rename is journaled: the
	// state commit still succeeds and only the context-checked finalization stops.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupted := f
	interrupted.app.Lifecycle.Kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseActivated {
			cancel()
		}
		return nil
	}
	_, _, err := executeCLIContext(ctx, interrupted, "remove", "demo", "--target", "claude", "--yes")
	if err == nil || !strings.Contains(err.Error(), "finalize grouped removal") {
		t.Fatalf("removal was not interrupted at finalization: %v", err)
	}
	skills := filepath.Join(claude.ConfigRoot, "skills")
	if leftovers := interruptedClaudeLeftovers(t, f, skills); len(leftovers) != 2 {
		t.Fatalf("fixture did not leave a backup and an open journal: %v", leftovers)
	}
	return f, skills, plugin
}

func interruptedClaudeLeftovers(t *testing.T, f cliFixture, skills string) []string {
	t.Helper()
	var leftovers []string
	entries, err := os.ReadDir(skills)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".agentplugins-") {
			leftovers = append(leftovers, entry.Name())
		}
	}
	open, err := f.app.Lifecycle.Kernel.Directory.ListOpen()
	if err != nil {
		t.Fatal(err)
	}
	for _, receipt := range open {
		leftovers = append(leftovers, receipt.OperationID)
	}
	return leftovers
}

func executeCLIContext(ctx context.Context, fixture cliFixture, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	app := fixture.app
	app.Input = strings.NewReader("")
	app.Output = &stdout
	app.ErrorOutput = &stderr
	command := NewRoot(app)
	command.SetArgs(args)
	err := command.ExecuteContext(ctx)
	return stdout.String(), stderr.String(), err
}
