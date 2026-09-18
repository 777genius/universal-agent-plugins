package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

// ApplyStdioDataContract rewrites a decoded stdio MCP server entry so it points
// at the package's future active root and its plugin data directory.
// PLUGIN_ROOT and PLUGIN_DATA are client-managed: an authored package that sets
// either of them is rejected rather than overwritten, because the override
// would silently redirect the server outside the managed tree.
//
// observationRoot is where the paths are checked to exist. During staging that
// is the staging tree, while pluginRoot is the location the entry has to
// encode; the two are deliberately different.
func ApplyStdioDataContract(config map[string]any, pluginRoot, dataPath string, observationRoot ...string) error {
	expand := strings.NewReplacer("${PLUGIN_ROOT}", pluginRoot, "${PLUGIN_DATA}", dataPath).Replace
	if err := applyStdioEnvironment(config, pluginRoot, dataPath, expand); err != nil {
		return err
	}
	expandStdioArguments(config, expand)
	command, _ := config["command"].(string)
	cwd, _ := config["cwd"].(string)
	resolvedCommand, resolvedCWD, err := ResolveStdioPaths(command, cwd, pluginRoot, dataPath, observationRoot...)
	if err != nil {
		return err
	}
	config["command"], config["cwd"] = resolvedCommand, resolvedCWD
	return nil
}

func applyStdioEnvironment(config map[string]any, pluginRoot, dataPath string, expand func(string) string) error {
	switch env := config["env"].(type) {
	case map[string]any:
		if err := rejectReservedStdioKeys(func(key string) bool { _, exists := env[key]; return exists }); err != nil {
			return err
		}
		for key, value := range env {
			if text, ok := value.(string); ok {
				env[key] = expand(text)
			}
		}
		env["PLUGIN_ROOT"], env["PLUGIN_DATA"] = pluginRoot, dataPath
	case map[string]string:
		if err := rejectReservedStdioKeys(func(key string) bool { _, exists := env[key]; return exists }); err != nil {
			return err
		}
		for key, value := range env {
			env[key] = expand(value)
		}
		env["PLUGIN_ROOT"], env["PLUGIN_DATA"] = pluginRoot, dataPath
	case nil:
		config["env"] = map[string]any{"PLUGIN_ROOT": pluginRoot, "PLUGIN_DATA": dataPath}
	default:
		return fmt.Errorf("stdio env must be an object")
	}
	return nil
}

func rejectReservedStdioKeys(present func(key string) bool) error {
	for _, reserved := range []string{"PLUGIN_ROOT", "PLUGIN_DATA"} {
		if present(reserved) {
			return fmt.Errorf("%s is reserved and client-managed", reserved)
		}
	}
	return nil
}

func expandStdioArguments(config map[string]any, expand func(string) string) {
	switch args := config["args"].(type) {
	case []any:
		for index, value := range args {
			if text, ok := value.(string); ok {
				args[index] = expand(text)
			}
		}
	case []string:
		for index := range args {
			args[index] = expand(args[index])
		}
	}
}

// ResolveStdioPaths projects a package-relative stdio command and working
// directory onto their future active locations.
//
// A copied staging tree has no symlinks (filetree.CopyDir enforces this), so
// resolution happens against the observed tree and the result is re-anchored to
// the future root.
func ResolveStdioPaths(command, cwd, pluginRoot, dataRoot string, observationRoot ...string) (string, string, error) {
	observedRoot := pluginRoot
	if len(observationRoot) > 0 {
		observedRoot = observationRoot[0]
	}
	resolve := func(anchor pathcontract.Anchor, relative string) (string, error) {
		return resolveAnchoredPath(anchor, relative, observedRoot, pluginRoot, dataRoot)
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

func resolveAnchoredPath(anchor pathcontract.Anchor, relative, observedRoot, pluginRoot, dataRoot string) (string, error) {
	root := observedRoot
	if anchor == pathcontract.Data {
		root = dataRoot
	}
	r := pathcontract.Resolve(root, relative)
	if r.State != pathcontract.Resolved {
		return "", fmt.Errorf("stdio path %s: %w", r.State, r.Err)
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
