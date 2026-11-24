package agentpluginscli

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// A package installed earlier without saving blocks the saved add. The message
// must name the client and give the exact command that removes it, and that
// command must actually remove the binding.
func TestSavedAddNamesTheClientAndCommandForAnUnmanagedInstallation(t *testing.T) {
	repo := dependencyRepo(t)
	f, _, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientCodex, executable: true})
	head := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD")
	head.Dir = repo
	rev, err := head.Output()
	if err != nil {
		t.Fatal(err)
	}
	source := githubIdentity("acme/demo", strings.TrimSpace(string(rev)), "")
	dependencyRun(t, f, "add", source, "--target", "codex", "--no-save", "--yes", "--format", "json")
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("fixture must hold exactly one unmanaged installation: %+v %v", state, err)
	}
	id := state.Installations[0].InstallationID

	_, _, err = f.execute(false, "add", source, "--target", "codex", "--yes")
	if err == nil {
		t.Fatal("saved add adopted an unmanaged installation")
	}
	var foreign *environment.ForeignBindingError
	if !errors.As(err, &foreign) {
		t.Fatalf("typed cause lost: %v", err)
	}
	command := "agentplugins remove " + id + " --target codex"
	for _, want := range []string{"already installed outside your dependency list", "Codex", "installation " + id, command, "plugin data is kept", "any step to finish in the client", "add with --no-save", "does not migrate"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("message lacks %q:\n%s", want, err)
		}
	}
	if strings.Contains(err.Error(), "then run this command again") {
		t.Fatal("message promises that disconnecting bindings migrates ownership")
	}
	dependencyRun(t, f, "add", source, "--target", "codex", "--no-save", "--yes", "--format", "json")

	// Codex removal needs a manual uninstall step; the command itself says so and
	// names --external-uninstalled, which this fixture (no real Codex) passes.
	args := strings.Fields(strings.TrimPrefix(command, "agentplugins "))
	if out, stderr, removeErr := f.execute(false, append(args, "--yes", "--external-uninstalled")...); removeErr != nil {
		t.Fatalf("the suggested command failed: %v\n%s\n%s", removeErr, out, stderr)
	}
	// The command removed the binding and kept the plugin data, as it states.
	if state, err = f.store.Load(); err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 {
		t.Fatalf("the suggested command did not remove the binding: %+v %v", state.Installations, err)
	}
}

func TestExplainForeignBindingLeavesOtherErrorsAlone(t *testing.T) {
	other := errors.New("drift: demo/codex")
	if got := explainForeignBinding(App{}, other); !errors.Is(got, other) {
		t.Fatalf("unrelated error rewritten: %v", got)
	}
	if explainForeignBinding(App{}, nil) != nil {
		t.Fatal("nil error invented")
	}
}
