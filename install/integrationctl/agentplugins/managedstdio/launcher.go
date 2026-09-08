// Package managedstdio implements the private, versioned Windsurf process adapter.
// It has no installer, network, configuration or state dependencies.
package managedstdio

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

const Mode = "--internal-stdio-v1"
const RelativeDirectory = "io.github.777genius.agentplugins/managed-stdio-v1"
const ExecutableName = "agentplugins"

// Arguments uses positional fields so child flags and empty arguments remain opaque.
func Arguments(plugin, data, cwd string, anchor pathcontract.Anchor, command string, args []string) []string {
	return append([]string{Mode, plugin, data, cwd, string(anchor), "--", command}, args...)
}

// Dispatch returns false only for an ordinary CLI invocation. Malformed private
// invocations never fall through to user configuration or Directory access.
func Dispatch(args []string, stderr io.Writer) (bool, int) {
	if len(args) == 0 || args[0] != Mode {
		return false, 0
	}
	if err := run(args[1:]); err != nil {
		fmt.Fprintln(stderr, "managed stdio:", err)
		return true, 126
	}
	return true, 0
}

func run(args []string) error {
	if !Supported() {
		return fmt.Errorf("managed stdio is unsupported on this platform")
	}
	if len(args) < 6 || args[4] != "--" {
		return fmt.Errorf("invalid protocol v1 arguments")
	}
	plugin, data, cwd := args[0], args[1], args[2]
	if !filepath.IsAbs(plugin) || !filepath.IsAbs(data) || !filepath.IsAbs(cwd) {
		return fmt.Errorf("protocol roots and cwd must be absolute")
	}
	root := plugin
	switch pathcontract.Anchor(args[3]) {
	case pathcontract.Plugin:
	case pathcontract.Data:
		root = data
	default:
		return fmt.Errorf("invalid cwd anchor")
	}
	// Strip without Clean/Rel: symlink/.. traversal must reach the resolver intact.
	relative := ""
	if cwd != root {
		prefix := strings.TrimRight(root, string(filepath.Separator)) + string(filepath.Separator)
		if !strings.HasPrefix(cwd, prefix) {
			return fmt.Errorf("cwd does not use its declared root")
		}
		relative = filepath.ToSlash(strings.TrimPrefix(cwd, prefix))
	}
	resolved := pathcontract.Resolve(root, relative)
	if resolved.State != pathcontract.Resolved {
		return fmt.Errorf("cwd unavailable: %v", resolved.Err)
	}
	info, err := os.Stat(resolved.Path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("cwd is not a directory")
	}
	command := args[5]
	if strings.HasPrefix(command, "./") {
		relative, err := pathcontract.ParseCommand(command)
		if err != nil {
			return err
		}
		result := pathcontract.Resolve(plugin, relative)
		if result.State != pathcontract.Resolved {
			return fmt.Errorf("bundled command unavailable: %v", result.Err)
		}
		command = result.Path
	} else {
		if command == "" || strings.ContainsAny(command, "/\\\x00") {
			return fmt.Errorf("command must be a bare executable or bundled path")
		}
		// Resolve PATH before chdir. Go rejects implicit current-directory lookup.
		command, err = exec.LookPath(command)
		if err != nil {
			return fmt.Errorf("runtime unavailable: %w", err)
		}
		if !filepath.IsAbs(command) {
			return fmt.Errorf("runtime PATH resolved to a relative executable")
		}
	}
	return replaceProcess(command, append([]string{command}, args[6:]...), resolved.Path)
}
