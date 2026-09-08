package providers

import (
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
	"os"
	"path/filepath"
	"strings"
)

// A copied staging tree has no symlinks (filetree.CopyDir enforces this). Resolve
// there before projecting canonical contained paths to its future active root.
func resolveStdioPaths(command, cwd, pluginRoot, dataRoot string, observationRoot ...string) (string, string, error) {
	observedRoot := pluginRoot
	if len(observationRoot) > 0 {
		observedRoot = observationRoot[0]
	}
	resolve := func(anchor pathcontract.Anchor, relative string) (string, error) {
		root := observedRoot
		if anchor == pathcontract.Data {
			root = dataRoot
		}
		r := pathcontract.Resolve(root, relative)
		if r.State != pathcontract.Resolved {
			return "", fmt.Errorf("stdio path %s: %v", r.State, r.Err)
		}

		base, err := filepath.EvalSymlinks(root)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(base, r.Path)
		if err != nil {
			return "", err
		}
		destination := pluginRoot
		if anchor == pathcontract.Data {
			destination = dataRoot
		}
		return filepath.Join(destination, rel), nil
	}
	if strings.HasPrefix(command, "./") {
		relative, err := pathcontract.ParseCommand(command)
		if err != nil {
			return "", "", err
		}
		command, err = resolve(pathcontract.Plugin, relative)
		if err != nil {
			return "", "", err
		}
	}
	parsed, err := pathcontract.ExpandCWD(cwd, pluginRoot, dataRoot)
	if err != nil {
		return "", "", err
	}
	resolvedCWD, err := resolve(parsed.Anchor, parsed.Relative)
	if err != nil {
		return "", "", err
	}
	// Directory readiness is checked against the observed topology, not a future path.
	root := observedRoot
	if parsed.Anchor == pathcontract.Data {
		root = dataRoot
	}
	r := pathcontract.Resolve(root, parsed.Relative)
	info, err := os.Stat(r.Path)
	if err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("stdio cwd is not an available directory")
	}
	return command, resolvedCWD, nil
}
