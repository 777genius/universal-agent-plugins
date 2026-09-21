package claude

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	errClaudeSkillSymlinkDangling     = errors.New("claude skill symlink target does not exist")
	errClaudeSkillSymlinkNotDirectory = errors.New("claude skill symlink does not resolve to a directory")
)

var (
	_ clients.RegistryInspector         = (*Adapter)(nil)
	_ clients.PreparedRegistryInspector = (*Adapter)(nil)
)

func (*Adapter) UsesNativeRegistryExecutable() bool { return true }

func (*Adapter) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if strings.TrimSpace(plan.NativeRegistryExecutable) == "" || env.Runner == nil {
		return clients.RegistryIndeterminate, nil
	}
	configRoot := strings.TrimSpace(plan.TargetAnchor)
	if configRoot == "" {
		configRoot = strings.TrimSpace(client.ConfigRoot)
	}
	if configRoot == "" && strings.TrimSpace(plan.TargetRoot) != "" {
		configRoot = filepath.Dir(filepath.Clean(plan.TargetRoot))
	}
	command, err := ClaudeListCommand(plan.NativeRegistryExecutable, configRoot, plan.ActivePath)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	result, err := RunClaudeListCommand(ctx, env.Runner, command)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return clients.RegistryIndeterminate, ctxErr
		}
		return clients.RegistryIndeterminate, err
	}
	if result.ExitCode != 0 {
		return clients.RegistryIndeterminate, fmt.Errorf("the Claude Code plugin registry command failed with exit code %d", result.ExitCode)
	}
	switch PluginStatusFromList(result.Stdout, plan.DeclaredName, plan.ActivePath) {
	case StatusInstalled:
		if managed == nil {
			return clients.RegistryCollision, nil
		}
		return clients.RegistryExpected, nil
	case StatusAbsent:
		return clients.RegistryClear, nil
	case StatusCollision:
		return clients.RegistryCollision, nil
	default:
		return clients.RegistryIndeterminate, nil
	}
}

func (*Adapter) InspectPreparedRegistry(plan domain.DeliveryPlan, name string, owned bool) (clients.RegistryFinding, error) {
	root := strings.TrimSpace(plan.TargetRoot)
	if root == "" {
		return clients.RegistryIndeterminate, nil
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return clients.RegistryClear, nil
	}
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	finding := clients.RegistryClear
	for _, entry := range entries {
		path, isDirectory, entryErr := claudePreparedEntry(root, entry)
		if errors.Is(entryErr, errClaudeSkillSymlinkDangling) && !shared.SameCleanPath(path, plan.ActivePath) {
			// Stale links to removed shared skills are common and cannot claim a
			// plugin identity while their target is absent. They are unrelated to
			// this mutation, so do not let one block every grouped install. A
			// dangling link at the planned active path still fails closed below.
			continue
		}
		if entryErr != nil {
			return clients.RegistryIndeterminate, entryErr
		}
		if !isDirectory {
			// A plain file cannot contain the .claude-plugin/plugin.json this
			// scheme requires, so it can never claim a competing plugin
			// identity. OS-generated artifacts such as .DS_Store are common
			// in a Finder-browsed skills directory and must not block every
			// other plugin's repair/update.
			continue
		}
		manifest := filepath.Join(path, ".claude-plugin", "plugin.json")
		manifestName, readErr := shared.ReadJSONManifestName(manifest)
		if os.IsNotExist(readErr) {
			// Plain skills legitimately share this directory and do not claim a
			// plugin identity.
			if _, skillErr := os.Lstat(filepath.Join(path, "SKILL.md")); skillErr == nil {
				continue
			} else if !os.IsNotExist(skillErr) {
				return clients.RegistryIndeterminate, skillErr
			}
			return clients.RegistryIndeterminate, nil
		}
		if readErr != nil {
			return clients.RegistryIndeterminate, readErr
		}
		if manifestName != name {
			continue
		}
		if shared.SameCleanPath(path, plan.ActivePath) && owned {
			finding = clients.RegistryExpected
			continue
		}
		return clients.RegistryCollision, nil
	}
	return finding, nil
}

func claudePreparedEntry(root string, entry os.DirEntry) (string, bool, error) {
	path := filepath.Join(root, entry.Name())
	if entry.Type()&os.ModeSymlink == 0 {
		return path, entry.IsDir(), nil
	}
	// Claude Code skills are commonly shared through symlinks. Follow the link
	// for read-only identity classification so a normal linked skill does not
	// block every unrelated plugin install. The caller ignores a dangling link
	// only when it is unrelated to the planned active path; all other unresolved
	// or non-directory links remain fail-closed.
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return path, false, fmt.Errorf("%w: %s", errClaudeSkillSymlinkDangling, path)
	}
	if err != nil {
		return path, false, err
	}
	if !info.IsDir() {
		return path, false, errClaudeSkillSymlinkNotDirectory
	}
	return path, true, nil
}
