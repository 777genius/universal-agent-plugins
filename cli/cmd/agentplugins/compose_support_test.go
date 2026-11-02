package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestInstallerFacadeComposesAllLocalClients(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "uap")
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(home, root, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"codex", "claude", "cursor", "copilot", "kiro", "gemini", "opencode", "cline", "windsurf", "grok", "kimi"} {
		if !app.Installer.SupportsClient(id) {
			t.Fatalf("composition excluded %s", id)
		}
	}
	for _, id := range []string{"chatgpt", "vscode"} {
		if !app.Installer.SupportsClient(id) || installer.ClientSupported(app.ClientRegistry, id) {
			t.Fatalf("SDK registration and saved qualification conflated for %s", id)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("composition wrote state: %v", err)
	}
}
