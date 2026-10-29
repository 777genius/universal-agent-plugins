package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func TestComposedManagementDiscoveryDoesNotExecuteOrMaterialize(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Chdir(home)
	for _, key := range []string{"APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME"} {
		t.Setenv(key, filepath.Join(home, key))
	}
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "GEMINI_CLI_HOME", "CLINE_DATA_DIR", "CLINE_MCP_SETTINGS_PATH", "AGENTPLUGINS_DIRECTORY_ORIGIN", "AGENTPLUGINS_DISCOVERY_ORIGIN", "AGENTPLUGINS_SECURITY_ORIGIN"} {
		t.Setenv(key, "")
	}
	bin := filepath.Join(home, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if runtime.GOOS != "windows" {
		// Version probing this executable would leave a persistent witness.
		body := "#!/bin/sh\necho invoked > \"$HOME/native-was-executed\"\necho 1.0.0\n"
		if err := os.WriteFile(filepath.Join(bin, "cursor"), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(home, ".cursor", "extensions", "saoudrizwan.claude-dev-1.0.0"), 0700); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(home, "uap-state-must-stay-absent")
	t.Setenv("AGENTPLUGINS_HOME", stateRoot)
	snapshot := func() map[string]string {
		files := map[string]string{}
		if err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				files[path] = "directory"
				return nil
			}
			body, err := os.ReadFile(path)
			if err == nil {
				files[path] = string(body)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	app, err := composeAgentpluginsApp(home, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	app.Output = &output
	command := agentpluginscli.NewRoot(app)
	command.SetArgs([]string{"manage", "discover"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	var discovered management.TargetDiscovery
	if err := json.Unmarshal(output.Bytes(), &discovered); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, target := range discovered.Targets {
		if target.Client != "cline" {
			continue
		}
		found = target.Available && target.DetectionStatus == management.ClientDetected && target.Profile != nil && target.Profile.ConfigRoot == filepath.Join(home, ".cline")
	}
	if !found {
		t.Fatal("composed OS detector did not retain Cline's real profile authority", discovered)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("passive composed command changed client files, acquired sources, probed a native binary or created UAP state")
	}
}
