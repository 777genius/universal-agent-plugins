package copilot

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCopilotNativeProjectVersionAdmission(t *testing.T) {
	tests := []struct {
		version string
		native  bool
	}{
		{"", false},
		{"1.0.80", false},
		{"1.0.81-7", false},
		{"1.0.81-8", true},
		{"1.0.81", true},
		{"1.0.82", true},
		{"1.0.103", true},
		{"v1.0.105", true},
		{"2.0.0", true},
		{"0.999.99", false},
		{"1.0.81-unknown", false},
		{"1.0.81-999999999999999999999999", false},
		{"1.0.82-beta", false},
		{"1.0.81x", false},
		{"1.0.82.1", false},
		{"garbage 1.0.103", false},
	}
	for _, test := range tests {
		if actual := copilotSupportsProjectPlugin(test.version); actual != test.native {
			t.Errorf("Copilot %q: native=%v want %v", test.version, actual, test.native)
		}
	}
}

func TestCopilotProjectVersionProbeRequiresSelectedIsolatedProfile(t *testing.T) {
	a := New()
	if !a.RequiresProjectVersion(domain.DetectedClient{ClientID: domain.ClientCopilot}) ||
		a.RequiresProjectVersion(domain.DetectedClient{ClientID: domain.ClientKimi}) ||
		!a.RequiresDisposableVersionProbeProfile() {
		t.Fatal("Copilot version gate or isolated profile contract changed")
	}
	root := filepath.Join(t.TempDir(), "isolated")
	env, err := a.VersionProbeEnvironment(root)
	if err != nil || len(env) != 1 || env[0] != "COPILOT_HOME="+root {
		t.Fatalf("selected probe env: %q %v", env, err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("Windows absolute-path variants differ")
	}
	for _, bad := range []string{"", "relative", "/", root + "/..", root + "\n"} {
		if _, err := a.VersionProbeEnvironment(bad); err == nil {
			t.Fatalf("unsafe profile accepted: %q", bad)
		}
	}
}
