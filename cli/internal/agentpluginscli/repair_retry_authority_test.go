package agentpluginscli

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Keep the real SDK, pinned store, native adapter, and existing disposable
// native protocol fixture. Only one later activation command fails; no client
// process is started by this fixture.
type ownedRepairRetryRunner struct {
	*dependencyClaudeListing
	cause    error
	armed    bool
	failures int
	commands int
}

func (runner *ownedRepairRetryRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands++
	if runner.armed && len(command.Argv) >= 3 && command.Argv[1] == "plugin" && command.Argv[2] == "add" {
		runner.armed = false
		runner.failures++
		return legacyports.CommandResult{NoChildStarted: true}, runner.cause
	}
	return runner.dependencyClaudeListing.Run(ctx, command)
}

func (runner *ownedRepairRetryRunner) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	runner.commands++
	return runner.dependencyClaudeListing.RunDuplexToCompletion(ctx, command, exchange)
}

type ownedRepairRetryFixture struct {
	cli            cliFixture
	runner         *ownedRepairRetryRunner
	source         *dependencyTestSource
	installationID string
	profile        string
	treeDigest     string
}

func newOwnedRepairRetryFixture(t *testing.T) ownedRepairRetryFixture {
	t.Helper()
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	runner := &ownedRepairRetryRunner{
		dependencyClaudeListing: &dependencyClaudeListing{root: client.ConfigRoot, failNextCodexAdd: true},
		cause:                   errors.New("injected owned repair activation failure"),
	}
	f, source := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{Runner: runner})
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil || runner.failNextCodexAdd {
		t.Fatalf("fixture did not reach its initial post-materialization activation failure: %v", err)
	}
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 1 || source.downloads != 0 || source.lookups != 0 {
		t.Fatalf("fixture did not install exactly one cached owned pin: installations=%d receipts=%d downloads=%d lookups=%d", len(state.Installations), len(state.EnvironmentReceipts), source.downloads, source.lookups)
	}
	binding := savedJourneyBinding(t, state, "codex")
	if binding.Activation != domain.ActivationFailed || binding.Verification != domain.VerificationFailed {
		t.Fatalf("fixture did not retain failed native readiness for repair: %+v", binding)
	}
	view, err := f.app.Installer.Inspect(t.Context())
	if err != nil || view.Recovery.Required || len(view.EnvironmentReceipts) != 1 {
		t.Fatalf("fixture activation failure left pending recovery or lost ownership: %+v %v", view, err)
	}
	runner.armed = true
	return ownedRepairRetryFixture{cli: f, runner: runner, source: source, installationID: state.Installations[0].InstallationID, profile: client.ConfigRoot, treeDigest: pin.TreeDigest}
}

func dropOwnedRepairAuthority(t *testing.T, fixture ownedRepairRetryFixture, loss string) {
	t.Helper()
	state, err := fixture.cli.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.EnvironmentReceipts) != 1 || state.EnvironmentReceipts[0].InstallationID != fixture.installationID {
		t.Fatal("the first repair lost the selected receipt before the retry prompt")
	}
	state.EnvironmentReceipts = slices.DeleteFunc(state.EnvironmentReceipts, func(receipt domain.EnvironmentReceipt) bool {
		return receipt.InstallationID == fixture.installationID
	})
	if loss == "installation" {
		// The real store rejects orphan receipts, so removing the selected
		// installation also removes only its own matching receipt.
		state.Installations = slices.DeleteFunc(state.Installations, func(installation domain.Installation) bool {
			return installation.InstallationID == fixture.installationID
		})
	}
	if err := fixture.cli.store.Save(state); err != nil {
		t.Fatal(err)
	}
}

func TestOwnedRepairRetryRejectsLostAuthority(t *testing.T) {
	for _, loss := range []string{"installation", "receipt"} {
		t.Run(loss, func(t *testing.T) { ownedRepairRetryLossCase(t, loss) })
	}
}

func ownedRepairRetryLossCase(t *testing.T, loss string) {
	t.Helper()
	fixture := newOwnedRepairRetryFixture(t)
	cmd, _, _ := retryTestCommand(t)
	cmd.Use = "repair"
	app := fixture.cli.app
	app.Terminal = true
	opts := &options{target: "codex", scope: "user", format: "human"}
	prompts, callsAtLoss := 0, 0
	var filesAtLoss, profileAtLoss map[string]dependencyCheckFileObservation
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		prompts++
		if prompts != 1 || fixture.runner.failures != 1 || fixture.runner.armed {
			t.Fatal("expected one retry prompt after the controlled native activation failure")
		}
		dropOwnedRepairAuthority(t, fixture, loss)
		filesAtLoss = observeDependencyCheckFiles(t, fixture.cli.root)
		profileAtLoss = observeDependencyCheckFiles(t, fixture.profile)
		callsAtLoss = fixture.runner.commands
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}
	handled, err := repairOwnedDependencies(cmd, app, opts, fixture.installationID, []domain.ClientID{domain.ClientCodex})
	if !handled || !errors.Is(err, fixture.runner.cause) || !errors.Is(err, installer.ErrPlanChanged) {
		t.Fatalf("owned retry lost handling or either cause: handled=%v error=%v", handled, err)
	}
	if prompts != 1 || opts.retryAttempt != 1 || fixture.runner.failures != 1 || fixture.runner.commands != callsAtLoss {
		t.Fatalf("authority loss retried or reached another native operation: prompts=%d attempts=%d failures=%d calls=%d/%d", prompts, opts.retryAttempt, fixture.runner.failures, fixture.runner.commands, callsAtLoss)
	}
	if fixture.source.downloads != 0 || fixture.source.lookups != 0 {
		t.Fatal("owned retry reacquired the already pinned package")
	}
	assertDependencyCheckFilesUnchanged(t, fixture.cli.root, filesAtLoss)
	assertDependencyCheckFilesUnchanged(t, fixture.profile, profileAtLoss)
}

func TestOwnedRepairInitiallyUnownedRetainsFallback(t *testing.T) {
	fixture := newOwnedRepairRetryFixture(t)
	dropOwnedRepairAuthority(t, fixture, "receipt")
	before := observeDependencyCheckFiles(t, fixture.cli.root)
	profileBefore := observeDependencyCheckFiles(t, fixture.profile)
	calls := fixture.runner.commands
	cmd, _, _ := retryTestCommand(t)
	app := fixture.cli.app
	app.Terminal = true
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		t.Fatal("an initially unowned selection prompted for owned repair")
		return prompt.ConfirmationResult{}, nil
	}}
	opts := &options{target: "codex", scope: "user", format: "human"}
	handled, err := repairOwnedDependencies(cmd, app, opts, fixture.installationID, []domain.ClientID{domain.ClientCodex})
	if handled || err != nil || opts.retryAttempt != 0 || opts.installResult != nil || fixture.runner.commands != calls {
		t.Fatalf("initial unowned fallback changed: handled=%v err=%v attempts=%d calls=%d/%d", handled, err, opts.retryAttempt, fixture.runner.commands, calls)
	}
	assertDependencyCheckFilesUnchanged(t, fixture.cli.root, before)
	assertDependencyCheckFilesUnchanged(t, fixture.profile, profileBefore)
}

func TestOwnedRepairRetryRestoresOwnedPin(t *testing.T) {
	fixture := newOwnedRepairRetryFixture(t)
	cmd, _, _ := retryTestCommand(t)
	cmd.Use = "repair"
	app := fixture.cli.app
	app.Terminal = true
	opts := &options{target: "codex", scope: "user", format: "human"}
	prompts, callsAtRetry := 0, 0
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		prompts++
		if prompts != 1 || fixture.runner.failures != 1 || fixture.runner.armed {
			t.Fatal("expected one retry prompt after the controlled native activation failure")
		}
		callsAtRetry = fixture.runner.commands
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}
	handled, err := repairOwnedDependencies(cmd, app, opts, fixture.installationID, []domain.ClientID{domain.ClientCodex})
	if !handled || err != nil || prompts != 1 || opts.retryAttempt != 1 || fixture.runner.failures != 1 || fixture.runner.commands <= callsAtRetry {
		t.Fatalf("owned repair did not recover on exactly one accepted retry: handled=%v err=%v prompts=%d attempts=%d failures=%d calls=%d/%d", handled, err, prompts, opts.retryAttempt, fixture.runner.failures, fixture.runner.commands, callsAtRetry)
	}
	if opts.installResult == nil || len(failedAgents(opts.installResult.Agents)) != 0 || len(fixture.runner.codexIDs) != 1 {
		t.Fatal("successful retry retained a failed report or lost the selected native fixture registration")
	}
	binding := dependencyBinding(t, fixture.cli)
	if binding.Owner.InstallationID != fixture.installationID || binding.TreeDigest != fixture.treeDigest || binding.Activation != string(domain.ActivationActive) || binding.Verification != string(domain.VerificationInstalled) {
		t.Fatalf("retry did not restore the original owned pin and verified fixture readiness: %+v", binding)
	}
	if fixture.source.downloads != 0 || fixture.source.lookups != 0 {
		t.Fatal("owned retry reacquired the already pinned package")
	}
}
