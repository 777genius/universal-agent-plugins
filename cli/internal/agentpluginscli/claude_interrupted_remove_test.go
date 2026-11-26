package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

const finishedInterruptedLine = "Finished the interrupted operation."

// A signal between the grouped removal's state commit and its directory
// finalization left a hidden backup with the same Claude plugin name and an
// open journal. Rerunning remove reported "not installed" and add saw the
// backup as an unmanaged native identity, so neither command could recover.
func TestClaudeUserRemoveInterruptedAfterStateCommitResumesOnRerun(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, wantErr string
		terminal      bool
		args          func(plugin string) []string
	}{
		{name: "remove", wantErr: `plugin is not installed for target "claude" in user scope`, args: func(string) []string {
			return []string{"remove", "demo", "--target", "claude", "--yes"}
		}},
		// The interactive target choice would otherwise find no bound target.
		{name: "prompted-remove", wantErr: "plugin has no materialized target in user scope", terminal: true, args: func(string) []string {
			return []string{"remove", "demo"}
		}},
		{name: "add", args: func(plugin string) []string { return []string{"add", plugin, "--target", "claude", "--yes"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, skills, plugin := newClaudeLifecycleFixture(t)
			interruptClaudeAfterStateCommit(t, f, skills, "finalize grouped removal", "remove", "demo", "--target", "claude", "--yes")
			out, stderr, err := f.execute(tc.terminal, tc.args(plugin)...)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("%s after the committed removal = %v, want %q\n%s\n%s", tc.name, err, tc.wantErr, out, stderr)
			}
			if err != nil && strings.Contains(err.Error(), "no target was changed") {
				t.Fatalf("%s claimed no change after finishing the interrupted removal: %v", tc.name, err)
			}
			if !strings.Contains(stderr, finishedInterruptedLine) {
				t.Fatalf("%s did not report the finished interrupted operation:\n%s", tc.name, stderr)
			}
			requireNoInterruptedLeftovers(t, f, skills, tc.name)
			if tc.name == "add" {
				if out, stderr, err := f.execute(false, "remove", "demo", "--target", "claude", "--yes"); err != nil {
					t.Fatalf("remove after the recovered add: %v\n%s\n%s", err, out, stderr)
				}
			}
		})
	}
	t.Run("dry-run", func(t *testing.T) {
		t.Parallel()
		f, skills, _ := newClaudeLifecycleFixture(t)
		interruptClaudeAfterStateCommit(t, f, skills, "finalize grouped removal", "remove", "demo", "--target", "claude", "--yes")
		_, stderr, _ := f.execute(false, "remove", "demo", "--target", "claude", "--dry-run")
		if leftovers := interruptedClaudeLeftovers(t, f, skills); len(leftovers) != 2 || strings.Contains(stderr, finishedInterruptedLine) {
			t.Fatalf("dry run changed the interrupted transaction: %v\n%s", leftovers, stderr)
		}
	})
}

// The same window in a swap leaves the previous version as the hidden backup.
// Update and repair plan against it before their mutation path could recover.
func TestClaudeUserUpdateInterruptedAfterStateCommitResumesOnRerun(t *testing.T) {
	t.Parallel()
	for _, rerun := range []string{"update", "repair"} {
		t.Run(rerun, func(t *testing.T) {
			t.Parallel()
			f, skills, plugin := newClaudeLifecycleFixture(t)
			writeCLIPluginVersion(t, plugin, "1.0.1")
			interruptClaudeAfterStateCommit(t, f, skills, "finalize grouped directory mutation", "update", "demo", "--target", "claude", "--yes")
			out, stderr, err := f.execute(false, rerun, "demo", "--target", "claude", "--yes")
			if err != nil {
				t.Fatalf("%s after the interrupted update: %v\n%s\n%s", rerun, err, out, stderr)
			}
			if !strings.Contains(stderr, finishedInterruptedLine) {
				t.Fatalf("%s did not report the finished interrupted operation:\n%s", rerun, stderr)
			}
			requireNoInterruptedLeftovers(t, f, skills, rerun)
			state, err := f.store.Load()
			if err != nil || len(state.Installations) != 1 || state.Installations[0].Package.Version != "1.0.1" {
				t.Fatalf("%s did not keep the committed update: %+v, %v", rerun, state, err)
			}
		})
	}
}

// Imperative commands recover only unowned work. A deps environment journal
// stays for deps recovery, whatever the command then reports.
func TestImperativeCommandsLeaveEnvironmentJournalForDepsRecovery(t *testing.T) {
	t.Parallel()
	f, _, plugin := newClaudeLifecycleFixture(t)
	journal, body := plantEnvironmentJournal(t, f)
	for _, args := range [][]string{
		{"remove", "demo", "--target", "claude", "--yes"},
		{"add", plugin, "--target", "claude", "--yes"},
	} {
		_, stderr, _ := f.execute(false, args...)
		after, err := os.ReadFile(journal)
		if err != nil || !bytes.Equal(after, body) || strings.Contains(stderr, finishedInterruptedLine) {
			t.Fatalf("%s changed the deps environment journal: %v\n%s", args[0], err, stderr)
		}
	}
	state, err := f.store.Load()
	if err != nil || !installationHasTarget(state.Installations[0], domain.ClientClaude, "user") {
		t.Fatalf("refused recovery still changed the installation: %+v, %v", state, err)
	}
}

func newClaudeLifecycleFixture(t *testing.T) (cliFixture, string, string) {
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
	return f, filepath.Join(claude.ConfigRoot, "skills"), plugin
}

// interruptClaudeAfterStateCommit cancels the command exactly as SIGINT does
// once the commit decision is durable: only the context-checked directory
// finalization stops, leaving the journal and hidden backup behind.
func interruptClaudeAfterStateCommit(t *testing.T, f cliFixture, skills, wantErr string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupted := f
	interrupted.app.StateStore = cancelAfterCommitDecision{Store: f.store, cancel: cancel}
	_, _, err := executeCLIContext(ctx, interrupted, args...)
	var groupErr *transaction.GroupError
	if !errors.As(err, &groupErr) || groupErr.Phase != transaction.GroupFailureCommitted || !strings.Contains(groupErr.Error(), wantErr) || !errors.Is(err, context.Canceled) {
		t.Fatalf("%s was not interrupted at finalization: %v", args[0], err)
	}
	state, loadErr := f.store.Load()
	if loadErr != nil || !hasStateCommittedReceipt(state) {
		t.Fatalf("%s cancellation did not retain its durable commit decision: state=%+v err=%v", args[0], state, loadErr)
	}
	if leftovers := interruptedClaudeLeftovers(t, f, skills); len(leftovers) != 2 {
		t.Fatalf("fixture did not leave a backup and an open journal: %v", leftovers)
	}
}

type cancelAfterCommitDecision struct {
	statev2.Store
	cancel context.CancelFunc
}

func (s cancelAfterCommitDecision) Save(state domain.StateFileV2) error {
	if err := s.Store.Save(state); err != nil {
		return err
	}
	if hasStateCommittedReceipt(state) {
		s.cancel()
	}
	return nil
}

func hasStateCommittedReceipt(state domain.StateFileV2) bool {
	receipts := append([]domain.MutationReceipt(nil), state.TransactionReceipts...)
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			receipts = append(receipts, binding.Receipts...)
		}
	}
	for _, receipt := range receipts {
		if receipt.Phase == transaction.ReceiptPhaseStateCommitted {
			return true
		}
	}
	return false
}

func requireNoInterruptedLeftovers(t *testing.T, f cliFixture, skills, command string) {
	t.Helper()
	if leftovers := interruptedClaudeLeftovers(t, f, skills); len(leftovers) != 0 {
		t.Fatalf("%s rerun left the interrupted transaction open: %v", command, leftovers)
	}
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

func writeCLIPluginVersion(t *testing.T, plugin, version string) {
	t.Helper()
	body := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"` + version + `","description":"Demo plugin"}`
	if err := os.WriteFile(filepath.Join(plugin, "plugin.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// plantEnvironmentJournal leaves a deps-owned directory transaction open,
// using the existing fault seam instead of a hand-written journal.
func plantEnvironmentJournal(t *testing.T, f cliFixture) (string, []byte) {
	t.Helper()
	const operation = "deps-interrupted-journal"
	data := filepath.Join(f.root, "data")
	owned := filepath.Join(data, "managed")
	staging := filepath.Join(owned, ".agentplugins-staging-"+operation)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	manager := dirswap.Manager{Namespace: data, JournalDir: f.operations, Fault: func(phase string) error {
		if phase == dirswap.PhaseBackupPending {
			return errors.New("disposable interrupted deps transaction")
		}
		return nil
	}}
	receipt, err := manager.Apply(context.Background(), dirswap.Input{EnvironmentID: environment.EnvironmentUserDefault, OperationID: operation,
		ClientBindingID: "deps-journal-fixture", Sequence: 1, OwnedBase: owned, ActivePath: filepath.Join(owned, "pending-"+operation),
		StagingPath: staging, RequireAbsent: true})
	if err == nil || receipt.OperationID != operation {
		t.Fatalf("fixture did not interrupt the deps transaction: %+v, %v", receipt, err)
	}
	receipt.Phase = dirswap.PhaseIntent
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(f.operations, operation+".json")
	if err := os.WriteFile(journal, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return journal, body
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
