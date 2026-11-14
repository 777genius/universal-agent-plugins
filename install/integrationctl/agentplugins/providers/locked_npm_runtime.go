package providers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	lockedRuntimePath = domain.ExtensionNamespace + "/" + domain.LockedRuntimeDir
	lockedLauncherArg = "${PLUGIN_ROOT}/" + lockedRuntimePath + "/launcher.mjs"
	runtimeMarkerName = ".agentplugins-runtime.json"
)

var lockedNPMName = regexp.MustCompile(`^(?:@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*$`)

type lockedRuntimeConfig struct {
	SchemaVersion     int    `json:"schema_version"`
	Package           string `json:"package"`
	Version           string `json:"version"`
	Entrypoint        string `json:"entrypoint"`
	PackageLockSHA256 string `json:"package_lock_sha256"`
	OmitOptional      bool   `json:"omit_optional"`
}

type lockedRuntimeMarker struct {
	SchemaVersion int    `json:"schema_version"`
	LockDigest    string `json:"lock_digest"`
	Package       string `json:"package"`
	Version       string `json:"version"`
	OmitOptional  bool   `json:"omit_optional"`
	Entrypoint    string `json:"entrypoint"`
}

// PrepareRuntime materializes a declarative, integrity-locked npm runtime before
// client activation. It never runs the plugin's launcher or npm lifecycle scripts.
// Unselected/non-runtime MCP packages remain inert.
func (manager PluginDataManager) PrepareRuntime(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	selected, err := selectedLockedRuntime(envelope, plan)
	if err != nil {
		return err
	}
	if !selected {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := pathpolicy.RequireContainedChild(manager.Base, dataPath); err != nil {
		return fmt.Errorf("unsafe PLUGIN_DATA runtime locator: %w", err)
	}
	if _, err := manager.readOwned(dataPath); err != nil {
		return err
	}
	runtimeRoot := filepath.Join(envelope.SnapshotRoot, filepath.FromSlash(lockedRuntimePath))
	if err := requireRealDirectory(envelope.SnapshotRoot); err != nil {
		return fmt.Errorf("locked runtime snapshot: %w", err)
	}
	if err := requireRealDirectory(filepath.Dir(runtimeRoot)); err != nil {
		return fmt.Errorf("locked runtime parent: %w", err)
	}
	if err := requireRealDirectory(runtimeRoot); err != nil {
		return fmt.Errorf("locked runtime directory: %w", err)
	}
	configBody, err := readRuntimeFile(runtimeRoot, "runtime.json")
	if err != nil {
		return err
	}
	packageBody, err := readRuntimeFile(runtimeRoot, "package.json")
	if err != nil {
		return err
	}
	lockBody, err := readRuntimeFile(runtimeRoot, "package-lock.json")
	if err != nil {
		return err
	}
	if _, err := readRuntimeFile(runtimeRoot, "launcher.mjs"); err != nil {
		return err
	}
	config, digest, err := validateLockedRuntime(configBody, packageBody, lockBody)
	if err != nil {
		return err
	}
	runner := manager.RunNPM
	if runner == nil {
		runner = runLockedNPM
	}
	return prepareLockedRuntime(ctx, dataPath, config, digest, packageBody, lockBody, runner)
}

// selectedLockedRuntime reports whether the plan selects a stdio server that
// runs the reserved locked-runtime launcher. Any selected server carrying that
// launcher must run under node: an unrecognized command would silently skip
// preparation and defer npm work to the client's first probe, recreating the
// cold-start diagnostic race the contract exists to prevent.
func selectedLockedRuntime(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (bool, error) {
	found := false
	for _, name := range domain.SelectedMCPNames(plan) {
		server := envelope.MCP.Servers[name]
		if server.Type != "stdio" || !hasLockedLauncherArg(server.Decoded["args"]) {
			continue
		}
		command, ok := server.Decoded["command"].(string)
		if !ok || !isNodeCommand(command) {
			return false, fmt.Errorf("MCP server %q runs the locked npm runtime launcher but its command %v is not node", name, server.Decoded["command"])
		}
		found = true
	}
	return found, nil
}

func isNodeCommand(command string) bool {
	base := command
	if index := strings.LastIndexAny(base, `/\`); index >= 0 {
		base = base[index+1:]
	}
	return strings.EqualFold(base, "node") || strings.EqualFold(base, "node.exe")
}

// hasLockedLauncherArg reports whether any argument is the reserved launcher
// path. The launcher can execute from any argument position, so a non-zero
// index must not silently escape preparation.
func hasLockedLauncherArg(value any) bool {
	switch args := value.(type) {
	case []any:
		for _, arg := range args {
			if arg == lockedLauncherArg {
				return true
			}
		}
	case []string:
		for _, arg := range args {
			if arg == lockedLauncherArg {
				return true
			}
		}
	}
	return false
}

func requireRealDirectory(name string) error {
	info, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is not a real directory", name)
	}
	return nil
}

func readRuntimeFile(root, name string) ([]byte, error) {
	file := filepath.Join(root, name)
	info, err := os.Lstat(file)
	if err != nil {
		return nil, fmt.Errorf("read locked runtime %s: %w", name, err)
	}
	if !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return nil, fmt.Errorf("locked runtime %s must be a regular file of at most 4 MiB", name)
	}
	body, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read locked runtime %s: %w", name, err)
	}
	return body, nil
}
