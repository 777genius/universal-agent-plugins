package agentpluginscli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Exercise the real saved lifecycle and SDK with the existing disposable native
// protocol fixture. Only one selected add/remove command fails; no child starts.
type savedLifecycleRetryRunner struct {
	*dependencyClaudeListing
	nativeVerb string
	cause      error
	armed      bool
	failures   int
	commands   int
}

func (runner *savedLifecycleRetryRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands++
	if runner.armed && len(command.Argv) >= 3 && command.Argv[1] == "plugin" && command.Argv[2] == runner.nativeVerb {
		runner.armed = false
		runner.failures++
		return legacyports.CommandResult{NoChildStarted: true}, runner.cause
	}
	return runner.dependencyClaudeListing.Run(ctx, command)
}

func (runner *savedLifecycleRetryRunner) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	runner.commands++
	return runner.dependencyClaudeListing.RunDuplexToCompletion(ctx, command, exchange)
}

type savedLifecycleRetryFixture struct {
	cli            cliFixture
	runner         *savedLifecycleRetryRunner
	source         *dependencyTestSource
	installationID string
	profile        string
	target         string
	treeDigest     string
}

func newSavedLifecycleRetryFixture(t *testing.T, operation string) savedLifecycleRetryFixture {
	t.Helper()
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	runner := &savedLifecycleRetryRunner{
		dependencyClaudeListing: &dependencyClaudeListing{root: client.ConfigRoot, failNextCodexAdd: operation == "update"},
		nativeVerb:              "remove",
		cause:                   errors.New("injected saved lifecycle command failure"),
	}
	if operation == "update" {
		runner.nativeVerb = "add"
	}
	f, source := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{Runner: runner})
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	_, _, syncErr := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json")
	if (operation == "update") != (syncErr != nil) || runner.failNextCodexAdd {
		t.Fatalf("wrong initial native fixture outcome for %s: %v", operation, syncErr)
	}
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 1 || source.downloads != 0 || source.lookups != 0 {
		t.Fatalf("fixture did not retain one cached owned pin: installations=%d receipts=%d downloads=%d lookups=%d", len(state.Installations), len(state.EnvironmentReceipts), source.downloads, source.lookups)
	}
	binding := savedJourneyBinding(t, state, "codex")
	wantActivation, wantVerification := domain.ActivationActive, domain.VerificationInstalled
	if operation == "update" {
		wantActivation, wantVerification = domain.ActivationFailed, domain.VerificationFailed
	}
	if binding.Activation != wantActivation || binding.Verification != wantVerification {
		t.Fatalf("wrong initial readiness for %s: %+v", operation, binding)
	}
	view, err := f.app.Installer.Inspect(t.Context())
	if err != nil || view.Recovery.Required || len(view.EnvironmentReceipts) != 1 {
		t.Fatalf("fixture lost owned readiness or requires recovery: %+v %v", view, err)
	}
	runner.armed = true
	return savedLifecycleRetryFixture{cli: f, runner: runner, source: source, installationID: state.Installations[0].InstallationID, profile: client.ConfigRoot, target: binding.TargetLocator, treeDigest: pin.TreeDigest}
}

func dropSavedLifecycleRetryAuthority(t *testing.T, fixture savedLifecycleRetryFixture, loss string) {
	t.Helper()
	state, err := fixture.cli.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.EnvironmentReceipts) != 1 || state.EnvironmentReceipts[0].InstallationID != fixture.installationID {
		t.Fatal("first saved lifecycle failure did not retain the selected receipt")
	}
	state.EnvironmentReceipts = slices.DeleteFunc(state.EnvironmentReceipts, func(receipt domain.EnvironmentReceipt) bool {
		return receipt.InstallationID == fixture.installationID
	})
	if loss == "installation" {
		// The store rejects orphan receipts; remove only this installation and
		// its own receipt together. The declaration and lock stay as published.
		state.Installations = slices.DeleteFunc(state.Installations, func(installation domain.Installation) bool {
			return installation.InstallationID == fixture.installationID
		})
	}
	if err := fixture.cli.store.Save(state); err != nil {
		t.Fatal(err)
	}
}

func TestSavedLifecycleRetryRejectsLostAuthority(t *testing.T) {
	for _, operation := range []string{"update", "remove"} {
		for _, loss := range []string{"installation", "receipt"} {
			t.Run(operation+"/"+loss, func(t *testing.T) {
				savedLifecycleRetryLossCase(t, operation, loss)
			})
		}
	}
}

func savedLifecycleRetryLossCase(t *testing.T, operation, loss string) {
	t.Helper()
	fixture := newSavedLifecycleRetryFixture(t, operation)
	cmd, _, _ := retryTestCommand(t)
	cmd.Use = operation
	app := fixture.cli.app
	app.Terminal = true
	opts := &options{target: "codex", scope: "user", format: "human"}
	prompts, callsAtLoss := 0, 0
	var filesAtLoss, profileAtLoss map[string]dependencyCheckFileObservation
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		prompts++
		if prompts != 1 || fixture.runner.failures != 1 || fixture.runner.armed || opts.installResult == nil || len(failedAgents(opts.installResult.Agents)) != 1 {
			t.Fatal("expected the retry prompt after the selected native command failure")
		}
		dropSavedLifecycleRetryAuthority(t, fixture, loss)
		filesAtLoss = observeDependencyCheckFiles(t, fixture.cli.root)
		profileAtLoss = observeDependencyCheckFiles(t, fixture.profile)
		callsAtLoss = fixture.runner.commands
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}
	handled, err := runSavedLifecycleRetryable(cmd, app, opts, fixture.installationID, operation)
	if !handled || !errors.Is(err, fixture.runner.cause) || !errors.Is(err, installer.ErrPlanChanged) {
		t.Fatalf("saved retry lost handling or either cause: handled=%v error=%v", handled, err)
	}
	if prompts != 1 || opts.retryAttempt != 1 || fixture.runner.failures != 1 || fixture.runner.commands != callsAtLoss {
		t.Fatalf("authority loss retried or reached another native command: prompts=%d attempts=%d failures=%d calls=%d/%d", prompts, opts.retryAttempt, fixture.runner.failures, fixture.runner.commands, callsAtLoss)
	}
	if fixture.source.downloads != 0 || fixture.source.lookups != 0 {
		t.Fatal("exact pinned saved lifecycle unexpectedly fetched a source")
	}
	assertDependencyCheckFilesUnchanged(t, fixture.cli.root, filesAtLoss)
	assertDependencyCheckFilesUnchanged(t, fixture.profile, profileAtLoss)
}

func TestSavedLifecycleInitiallyUnownedRetainsFallback(t *testing.T) {
	for _, operation := range []string{"update", "remove"} {
		t.Run(operation, func(t *testing.T) {
			fixture := newSavedLifecycleRetryFixture(t, operation)
			dropSavedLifecycleRetryAuthority(t, fixture, "receipt")
			before := observeDependencyCheckFiles(t, fixture.cli.root)
			profileBefore := observeDependencyCheckFiles(t, fixture.profile)
			calls := fixture.runner.commands
			cmd, _, _ := retryTestCommand(t)
			cmd.Use = operation
			app := fixture.cli.app
			app.Terminal = true
			app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				t.Fatal("initially unowned saved selection prompted")
				return prompt.ConfirmationResult{}, nil
			}}
			opts := &options{target: "codex", scope: "user", format: "human"}
			handled, err := runSavedLifecycleRetryable(cmd, app, opts, fixture.installationID, operation)
			if handled || err != nil || opts.retryAttempt != 0 || opts.installResult != nil || fixture.runner.commands != calls || fixture.source.downloads != 0 || fixture.source.lookups != 0 {
				t.Fatalf("initial saved fallback changed: handled=%v err=%v attempts=%d calls=%d/%d", handled, err, opts.retryAttempt, fixture.runner.commands, calls)
			}
			assertDependencyCheckFilesUnchanged(t, fixture.cli.root, before)
			assertDependencyCheckFilesUnchanged(t, fixture.profile, profileBefore)
		})
	}
}

func TestSavedLifecycleRetryCompletesCurrentSelection(t *testing.T) {
	for _, operation := range []string{"update", "remove"} {
		t.Run(operation, func(t *testing.T) { savedLifecycleRetrySuccessCase(t, operation) })
	}
}

func savedLifecycleRetrySuccessCase(t *testing.T, operation string) {
	t.Helper()
	fixture := newSavedLifecycleRetryFixture(t, operation)
	cmd, _, _ := retryTestCommand(t)
	cmd.Use = operation
	app := fixture.cli.app
	app.Terminal = true
	opts := &options{target: "codex", scope: "user", format: "human"}
	prompts, callsAtRetry := 0, 0
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		prompts++
		if prompts != 1 || fixture.runner.failures != 1 || fixture.runner.armed {
			t.Fatal("expected one retry prompt after the controlled saved lifecycle failure")
		}
		callsAtRetry = fixture.runner.commands
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}
	handled, err := runSavedLifecycleRetryable(cmd, app, opts, fixture.installationID, operation)
	if !handled || err != nil || prompts != 1 || opts.retryAttempt != 1 || fixture.runner.failures != 1 || fixture.runner.commands <= callsAtRetry {
		t.Fatalf("saved lifecycle did not recover on one accepted retry: handled=%v err=%v prompts=%d attempts=%d failures=%d calls=%d/%d", handled, err, prompts, opts.retryAttempt, fixture.runner.failures, fixture.runner.commands, callsAtRetry)
	}
	if opts.installResult == nil || len(failedAgents(opts.installResult.Agents)) != 0 || fixture.source.downloads != 0 || fixture.source.lookups != 0 {
		t.Fatal("successful saved retry retained failures or fetched an exact pinned source")
	}
	if operation == "update" {
		binding := dependencyBinding(t, fixture.cli)
		if binding.Owner.InstallationID != fixture.installationID || binding.TreeDigest != fixture.treeDigest || binding.Activation != string(domain.ActivationActive) || binding.Verification != string(domain.VerificationInstalled) || len(fixture.runner.codexIDs) != 1 {
			t.Fatalf("saved update lost its owned pin or fixture readiness: %+v", binding)
		}
		return
	}
	assertSavedLifecycleRemovalComplete(t, fixture)
}

func assertSavedLifecycleRemovalComplete(t *testing.T, fixture savedLifecycleRetryFixture) {
	t.Helper()
	view, err := fixture.cli.app.Installer.Inspect(t.Context())
	if err != nil || view.Recovery.Required || len(view.EnvironmentReceipts) != 0 || len(fixture.runner.codexIDs) != 0 {
		t.Fatalf("saved removal retained ownership, registration, or recovery: %+v %v", view, err)
	}
	for _, installation := range view.Installations {
		if installation.InstallationID == fixture.installationID && len(installation.Bindings) != 0 {
			t.Fatalf("saved removal retained selected bindings: %+v", installation)
		}
	}
	if _, err := os.Lstat(fixture.target); !os.IsNotExist(err) {
		t.Fatalf("saved removal retained the exact managed target: %v", err)
	}
	mp, lp := fixture.cli.app.dependencyPaths()
	requirements, err := readDependencyManifest(mp)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := readDependencyLock(lp, requirements)
	if err != nil || len(requirements.Dependencies) != 0 || len(requirements.Targets) != 0 || len(lock.Packages) != 0 {
		t.Fatalf("saved removal did not publish empty selected intent: %+v %+v %v", requirements, lock, err)
	}
}
