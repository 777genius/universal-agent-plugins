package providers_test

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

// Scope-isolation fixtures start after the selected trust preference and its
// normal writer-lock identity exist. Subsequent no-op verification must retain
// every captured byte and mode; dedicated trust tests cover initial creation.
func projectTrustFixture(t *testing.T, path, body string) {
	t.Helper()
	projectGeminiWrite(t, path, body)
	file, err := nativeconfig.New().BeginExactFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
