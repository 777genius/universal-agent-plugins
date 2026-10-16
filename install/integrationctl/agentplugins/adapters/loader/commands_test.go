package loader

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const loaderCommand = "---\ndescription: Say hello\ndisable-model-invocation: false\n---\nHello\n"

// The loader reads only the commands directory, never follows a link and
// never looks at the rest of the namespace, which belongs to other features.
func TestLoadCommandsFromTheReservedDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeMinimalPlugin(t, root, "demo")
	writeLoaderFile(t, filepath.Join(root, "skills", "guide", "SKILL.md"), "---\nname: guide\ndescription: Guide\n---\n")
	writeLoaderFile(t, filepath.Join(root, "skills", "notes.txt"), "not a skill")
	writeLoaderFile(t, filepath.Join(root, domain.ExtensionNamespace, domain.LockedRuntimeDir, "launcher.mjs"), "${CLAUDE_PLUGIN_ROOT}")
	commands := filepath.Join(root, filepath.FromSlash(commandext.RelativeDirectory))
	writeLoaderFile(t, filepath.Join(commands, "hello.md"), loaderCommand)
	writeLoaderFile(t, filepath.Join(commands, ".gitkeep"), "")
	writeLoaderFile(t, filepath.Join(commands, ".drafts", "x.md"), "draft")
	if err := os.Symlink(filepath.Join(commands, "hello.md"), filepath.Join(commands, "alias.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	envelope, err := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if command := envelope.Commands["hello"]; len(envelope.Commands) != 1 || command.Description != "Say hello" || string(command.Body) != "Hello\n" {
		t.Fatalf("commands = %+v", envelope.Commands)
	}
	if !reflect.DeepEqual(envelope.InvalidCommands, []string{"alias"}) {
		t.Fatalf("invalid = %v", envelope.InvalidCommands)
	}
	var commandCodes []string
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Boundary == domain.BoundaryCommand {
			commandCodes = append(commandCodes, diagnostic.Item+":"+diagnostic.Code)
		}
	}
	if !reflect.DeepEqual(commandCodes, []string{"alias:command_name_invalid"}) {
		t.Fatalf("command diagnostics = %v", commandCodes)
	}
	entries := append([]string(nil), envelope.SkillsDirEntries...)
	if !reflect.DeepEqual(entries, []string{"guide", "notes.txt"}) {
		t.Fatalf("skills entries = %v", entries)
	}
}

func TestLoadRejectsALinkedCommandsDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeMinimalPlugin(t, root, "demo")
	outside := t.TempDir()
	writeLoaderFile(t, filepath.Join(outside, "hello.md"), loaderCommand)
	namespace := filepath.Join(root, domain.ExtensionNamespace)
	if err := os.MkdirAll(namespace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(namespace, domain.CommandsDir)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	envelope, err := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope.Commands) != 0 || len(envelope.Diagnostics) != 1 || envelope.Diagnostics[0].Code != "commands_root_invalid" {
		t.Fatalf("envelope = %+v", envelope)
	}
}
