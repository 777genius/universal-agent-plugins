package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPluginKitAIValidateTextPrintsStructuredFailureForMissingManifest(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	missingRoot := t.TempDir()

	validateCmd := exec.Command(pluginKitAIBin, "validate", missingRoot)
	validateCmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := validateCmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected validate failure")
	}
	text := string(out)
	if !strings.Contains(text, "Failure: required manifest missing: "+authoredSlash("plugin.yaml")) {
		t.Fatalf("validate output missing structured failure line:\n%s", text)
	}
	if strings.Contains(text, "Usage:") {
		t.Fatalf("validate output should not include cobra usage for report failures:\n%s", text)
	}
}
