package shared

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

func TestCopilotRegistryFindingRequiresExactNativeIdentity(t *testing.T) {
	marketplace := "agentplugins-demo-0123456789ab"
	tests := []struct {
		name string
		body string
		want clients.RegistryFinding
	}{
		{name: "exact", body: "Installed plugins:\n  • demo@" + marketplace + " (v1.0.0)\n", want: clients.RegistryExpected},
		{name: "wrong_version", body: "Installed plugins:\n  • demo@" + marketplace + " (v1.0.1)\n", want: clients.RegistryIndeterminate},
		{name: "unrelated", body: "Installed plugins:\n  • other@" + marketplace + " (v1.0.0)\n", want: clients.RegistryClear},
		{name: "duplicate", body: "Installed plugins:\n  • demo@" + marketplace + " (v1.0.0)\n  • demo@" + marketplace + " (v1.0.0)\n", want: clients.RegistryIndeterminate},
		{name: "absent_short", body: "No plugins installed.\n", want: clients.RegistryIndeterminate},
		{name: "absent_official_1_0_80", body: "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n", want: clients.RegistryClear},
		{name: "absent_official_with_prefix", body: "\nNo plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n", want: clients.RegistryIndeterminate},
		{name: "absent_official_with_suffix", body: "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n\n", want: clients.RegistryIndeterminate},
		{name: "unknown", body: "Copilot plugins are unavailable right now\n", want: clients.RegistryIndeterminate},
		{name: "unknown_prefix", body: "experimental output\nInstalled plugins:\n  • demo@" + marketplace + " (v1.0.0)\n", want: clients.RegistryIndeterminate},
		{name: "unknown_suffix", body: "Installed plugins:\n  • demo@" + marketplace + " (v1.0.0)\nupdate available\n", want: clients.RegistryIndeterminate},
		{name: "unknown_unindented_entry", body: "Installed plugins:\ndemo@" + marketplace + " (v1.0.0)\n", want: clients.RegistryIndeterminate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CopilotRegistryFinding([]byte(test.body), "demo", marketplace, "1.0.0", true); got != test.want {
				t.Fatalf("finding = %d, want %d", got, test.want)
			}
		})
	}
}

func TestCopilotLiveRegistryFindingRequiresExactIdentityStatusAndPath(t *testing.T) {
	t.Parallel()
	marketplace := "agentplugins-demo-0123456789ab"
	path := filepath.Join(t.TempDir(), "managed", "demo")
	header := CopilotLiveHeader + "\n"
	entry := func(name, version, status, from string) string {
		return header + "  • " + name + "@" + marketplace + " (v" + version + ") (" + status + ")\n      from " + from + "\n"
	}
	tests := []struct {
		name  string
		body  string
		owned bool
		want  clients.RegistryFinding
	}{
		{name: "managed", body: entry("demo", "1.7.0-uap.1", "enabled", path), owned: true, want: clients.RegistryExpected},
		{name: "managed with unrelated", body: entry("other", "9.9.9", "enabled", path) + strings.TrimPrefix(entry("demo", "1.7.0-uap.1", "enabled", path), header), owned: true, want: clients.RegistryExpected},
		{name: "managed CRLF", body: strings.ReplaceAll(entry("demo", "1.7.0-uap.1", "enabled", path), "\n", "\r\n"), owned: true, want: clients.RegistryExpected},
		{name: "unowned collision", body: entry("demo", "1.7.0-uap.1", "enabled", path), want: clients.RegistryCollision},
		{name: "unrelated", body: entry("other", "9.9.9", "enabled", path), owned: true, want: clients.RegistryClear},
		{name: "disabled", body: entry("demo", "1.7.0-uap.1", "disabled", path), owned: true, want: clients.RegistryIndeterminate},
		{name: "wrong version", body: entry("demo", "1.7.0-uap.0", "enabled", path), owned: true, want: clients.RegistryIndeterminate},
		{name: "wrong path", body: entry("demo", "1.7.0-uap.1", "enabled", filepath.Join(t.TempDir(), "other")), owned: true, want: clients.RegistryIndeterminate},
		{name: "dot segment path", body: entry("demo", "1.7.0-uap.1", "enabled", filepath.Dir(path)+string(filepath.Separator)+"alias"+string(filepath.Separator)+".."+string(filepath.Separator)+filepath.Base(path)), owned: true, want: clients.RegistryIndeterminate},
		{name: "duplicate", body: entry("demo", "1.7.0-uap.1", "enabled", path) + strings.TrimPrefix(entry("demo", "1.7.0-uap.1", "enabled", path), header), owned: true, want: clients.RegistryIndeterminate},
		{name: "missing from", body: header + "  • demo@" + marketplace + " (v1.7.0-uap.1) (enabled)\n", owned: true, want: clients.RegistryIndeterminate},
		{name: "extra suffix", body: entry("demo", "1.7.0-uap.1", "enabled", path) + "unexpected\n", owned: true, want: clients.RegistryIndeterminate},
		{name: "ansi prefix", body: "\x1b[32m" + entry("demo", "1.7.0-uap.1", "enabled", path), owned: true, want: clients.RegistryIndeterminate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CopilotRegistryFindingAt([]byte(test.body), "demo", marketplace, "1.7.0-uap.1", path, test.owned); got != test.want {
				t.Fatalf("finding=%d want=%d", got, test.want)
			}
		})
	}
}
