package agentpluginscli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// preparedKiroSavedHarness has a Kiro whose MCP connection the CLI cannot check
// (its runner has no duplex capability), so a saved install can only prepare
// files. Unlike the all-clients matrix, which reaches preparation only through
// the Windows runtime proxy, this fixture exercises it on every host.
func preparedKiroSavedHarness(t *testing.T) (cliFixture, string) {
	t.Helper()
	repo := dependencyRepo(t)
	kiro := fixtureClient(t, domain.ClientKiro)
	kiro.ExecutablePath = filepath.Join(t.TempDir(), "kiro-cli")
	dependencyWrite(t, kiro.ExecutablePath, "fixture executable, never launched")
	f, _ := newDependencyHarness(t, repo, []domain.DetectedClient{kiro}, installer.Config{Runner: &cliRunOnlyRunner{}})
	head := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD")
	head.Dir = repo
	out, err := head.Output()
	if err != nil {
		t.Fatal(err)
	}
	return f, githubIdentity("acme/demo", strings.TrimSpace(string(out)), "")
}

func TestSavedAddOffersAndPreparesKiroWhenItsConnectionCannotBeChecked(t *testing.T) {
	f, source := preparedKiroSavedHarness(t)
	app := f.app
	app.Terminal = true
	// The picker offers Kiro as prepare-only instead of dropping it with the
	// "cannot check" explanation.
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	targets, err := savedAddTargets(cmd, app, &options{format: "human", scope: "user"}, source, nil)
	if err != nil || len(targets) != 1 || targets[0] != domain.ClientKiro {
		t.Fatalf("Kiro was not offered for a saved install: targets=%v err=%v", targets, err)
	}
	choice := targetChoiceFor(preparedOnlyClient(t, app, source))
	if choice.Note != preparedOnlyNote || choice.Info != preparedOnlyInfo || strings.Contains(choice.Label, "prepare only") {
		t.Fatalf("prepare-only marker was not moved into the annotation fields: %+v", choice)
	}

	// Add, repeat and remove through the public commands keep the file-only
	// intent, never claim runtime verification and keep the saved agreement.
	out, stderr, err := f.executeInput(true, "y\n", "add", source)
	if err != nil {
		t.Fatalf("saved add: %v\n%s\n%s", err, out, stderr)
	}
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 1 {
		t.Fatalf("saved add left no single owned installation: %+v %v", state, err)
	}
	var binding domain.ClientBinding
	for _, saved := range state.Installations[0].Clients {
		binding = saved
	}
	if binding.ClientID != string(domain.ClientKiro) || binding.InstallIntent != domain.InstallIntentPrepare ||
		binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationPackageValid {
		t.Fatalf("Kiro binding is not an honest prepared binding: %+v", binding)
	}
	if choice := targetChoiceFor(preparedOnlyClient(t, app, source)); choice.Note != preparedOnlyNote {
		t.Fatalf("existing preparation was not restored in the picker: %+v", choice)
	}
	before, err := os.ReadFile(f.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.executeInput(true, "y\n", "add", source, "--target", "kiro"); err != nil {
		t.Fatalf("repeat add: %v", err)
	}
	after, err := os.ReadFile(f.store.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("repeat add changed state (err=%v)", err)
	}
	if out, stderr, err := f.execute(false, "remove", state.Installations[0].InstallationID, "--target", "kiro", "--yes"); err != nil {
		t.Fatalf("remove: %v\n%s\n%s", err, out, stderr)
	}
	state, err = f.store.Load()
	if err != nil || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("removal kept the owner receipt: %+v %v", state.EnvironmentReceipts, err)
	}
}

func TestSavedKiroDoesNotOfferFreshPreparationForExistingAutomaticInstallation(t *testing.T) {
	f, source := preparedKiroSavedHarness(t)
	dependencyRun(t, f, "add", source, "--target", "kiro", "--yes", "--format", "json")
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Model a previously automatic installation whose host can no longer verify
	// MCP. The installer must not silently change its recorded intent.
	for key, binding := range state.Installations[0].Clients {
		binding.InstallIntent = domain.InstallIntentAutomatic
		state.Installations[0].Clients[key] = binding
	}
	state.Installations[0].InstallPreferences = nil
	if err := f.store.Save(state); err != nil {
		t.Fatal(err)
	}
	clients, err := f.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	compatible, skipped, _, err := compatibleSavedAddTargets(t.Context(), f.app, &options{}, source, clients)
	if err != nil || len(compatible) != 0 || len(skipped) != 1 {
		t.Fatalf("existing automatic installation offered as fresh preparation: compatible=%v skipped=%v err=%v", compatible, skipped, err)
	}
}

func preparedOnlyClient(t *testing.T, app App, source string) domain.DetectedClient {
	t.Helper()
	clients, err := app.Detector.Detect(t.Context())
	if err != nil || len(clients) != 1 {
		t.Fatalf("%v %v", clients, err)
	}
	compatible, skipped, _, err := compatibleSavedAddTargets(t.Context(), app, &options{}, source, clients)
	if err != nil || len(compatible) != 1 || len(skipped) != 0 {
		t.Fatalf("compatible=%v skipped=%v err=%v", compatible, skipped, err)
	}
	return compatible[0]
}

func TestSavedKiroDoesNotOfferPreparationWhenPreparationPreflightFails(t *testing.T) {
	f, source := preparedKiroSavedHarness(t)
	f.app.Lifecycle.Activator = deniedEligibilityActivator{}
	clients, err := f.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	compatible, skipped, _, err := compatibleSavedAddTargets(t.Context(), f.app, &options{}, source, clients)
	if err != nil || len(compatible) != 0 || len(skipped) != 1 {
		t.Fatalf("failed preparation was offered: compatible=%v skipped=%v err=%v", compatible, skipped, err)
	}
}
