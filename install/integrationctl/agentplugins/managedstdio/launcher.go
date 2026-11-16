// Package managedstdio implements the private, versioned Windsurf process adapter.
// It has no installer, network, configuration or state dependencies.
package managedstdio

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

const Mode = "--internal-stdio-v1"
const RelativeDirectory = domain.ExtensionNamespace + "/" + domain.ManagedStdioDir

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
		_, _ = fmt.Fprintln(stderr, "managed stdio:", err)
		return true, 126
	}
	return true, 0
}

func run(args []string) error {
	if !Supported() {
		return fmt.Errorf("managed stdio is unsupported on this platform")
	}
	if len(args) < 6 {
		return fmt.Errorf("invalid protocol v1 arguments")
	}
	child, err := DecodeCommandArguments(args[4], args[5:])
	if err != nil {
		return err
	}
	plugin, data, cwd := args[0], args[1], args[2]
	if !filepath.IsAbs(plugin) || !filepath.IsAbs(data) || !filepath.IsAbs(cwd) {
		return fmt.Errorf("protocol roots and cwd must be absolute")
	}
	root := plugin
	if anchor := pathcontract.Anchor(args[3]); anchor == pathcontract.Data {
		root = data
	} else if anchor != pathcontract.Plugin {
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
		return fmt.Errorf("cwd unavailable: %w", resolved.Err)
	}
	info, err := os.Stat(resolved.Path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("cwd is not a directory")
	}
	command, err := resolveRuntimeCommand(plugin, child[0])
	if err != nil {
		return err
	}
	return replaceProcess(command, append([]string{command}, child[1:]...), resolved.Path)
}
