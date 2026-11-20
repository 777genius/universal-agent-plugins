package installer

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
)

// A client found only by its config folder must get an actionable message,
// not the request-field wording meant for API callers.
func TestMissingRequiredClientExecutableNamesTheProgramToInstall(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(copilot.New())
	if err != nil {
		t.Fatal(err)
	}
	eng, err := newTestEngine(t, Config{StateRoot: filepath.Join(base, "uap"), Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(base, ".copilot")
	if !eng.RequiresClientExecutable("copilot") {
		t.Fatal("copilot User scope no longer requires its executable; pick another fixture client")
	}
	const hint = "GitHub Copilot CLI needs its command-line program, which was not found on PATH"

	_, err = eng.detectedClient(Request{Operation: OpInstall, ClientID: "copilot", ClientConfigRoot: config})
	if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), hint) {
		t.Fatalf("install without executable: %v", err)
	}
	_, err = eng.knownTargetIndex(OpInstall, []TargetFacts{{ClientID: "copilot", BindingID: "binding", ConfigRoot: config}})
	if !errors.Is(err, ErrTargetFactsUnavailable) || !strings.Contains(err.Error(), hint) {
		t.Fatalf("target facts without executable: %v", err)
	}
	if _, err := eng.knownTargetIndex(OpRemove, []TargetFacts{{ClientID: "copilot", BindingID: "binding", ConfigRoot: config}}); err != nil {
		t.Fatalf("removal must not require the executable: %v", err)
	}

	// A supplied but malformed path keeps the precise request-validation error.
	_, err = eng.detectedClient(Request{Operation: OpInstall, ClientID: "copilot", ClientConfigRoot: config, ClientExecutable: "relative/copilot"})
	if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "ClientExecutable must be an explicit absolute clean path") {
		t.Fatalf("relative executable: %v", err)
	}
}
