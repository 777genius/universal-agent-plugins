package vscodelocal_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

// Red: optional MCP borrows fake/no helper readiness, gets a direct command
// instead of recorded helper, or substitutes native hook runtime identity.
// Boundary: public Engine projection then actual installed managed dispatcher.
func TestLocalOptionalMCPUsesGenuineRecordedHelper(t *testing.T) {
	f := freshLocal(t, true)
	f.config.NativeStop = false
	f.config.HookSpecs = nil
	f.config.MCPServers = []string{"notify"}
	f.config.Skills = []string{"notify"}
	f.config.QualifiedTuple.TargetShell = ""
	f.config.TargetShell.Shell = ""
	a, err := vscode.NewLocal(f.config)
	must(t, err)
	*f.adapter = *a
	installed := applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
	facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
	if facts.NativeStop || len(facts.MCPServers) != 1 || len(facts.Skills) != 1 {
		t.Fatal("manual selection changed")
	}
	if _, err := os.Lstat(filepath.Join(installed.Binding.TargetPath, hookPath())); !os.IsNotExist(err) {
		t.Fatal("manual-only projection retained Stop", err)
	}
	var doc struct {
		MCPServers map[string]struct {
			Command string
			Args    []string
		}
	}
	must(t, json.Unmarshal(readLocal(t, filepath.Join(installed.Binding.TargetPath, "mcp.json")), &doc))
	server := doc.MCPServers["notify"]
	expected := filepath.Join(installed.Binding.TargetPath, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	if server.Command != expected || len(server.Args) == 0 || server.Args[0] != managedstdio.Mode {
		t.Fatal("optional stdio did not bind recorded helper")
	}
	command := exec.CommandContext(t.Context(), server.Command, server.Args...)
	command.Env = []string{"HOME=" + f.root, "USERPROFILE=" + f.root, "PLUGIN_ROOT=" + installed.Binding.TargetPath, "PLUGIN_DATA=" + installed.Binding.DataRoot, "PATH=/usr/bin:/bin"}
	output, err := runLocalProcess(t, command)
	if err != nil {
		t.Fatalf("installed genuine helper: %v %s", err, output)
	}
	var args []string
	must(t, json.Unmarshal(output, &args))
	if len(args) != 1 || args[0] != installed.Binding.DataRoot+"/mcp-locator" {
		t.Fatalf("managed stdio root projection differs: %q", args)
	}
}
