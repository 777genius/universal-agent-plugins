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
	entries, err := readClaudePreparedEntries(root)
	if os.IsNotExist(err) {
		return clients.RegistryClear, nil
	}
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	finding := clients.RegistryClear
	for _, entry := range entries {
		next, skip, classErr := classifyClaudePreparedEntry(root, entry, plan, name, owned)
		if classErr != nil {
			return clients.RegistryIndeterminate, classErr
		}
		if skip {
			continue
		}
		if next == clients.RegistryCollision || next == clients.RegistryIndeterminate {
			return next, nil
		}
		if next == clients.RegistryExpected {
			finding = clients.RegistryExpected
		}
	}
	return finding, nil
}

func classifyClaudePreparedEntry(root string, entry os.DirEntry, plan domain.DeliveryPlan, name string, owned bool) (clients.RegistryFinding, bool, error) {
	path, isDirectory, entryErr := claudePreparedEntry(root, entry)
	if ignoreUnrelatedDanglingClaudeSkill(entryErr, path, plan.ActivePath) {
		return clients.RegistryClear, true, nil
	}
	if entryErr != nil {
		return clients.RegistryIndeterminate, false, entryErr
	}
	if !isDirectory {
		// A plain file cannot contain the .claude-plugin/plugin.json this
		// scheme requires, so it can never claim a competing plugin
		// identity. OS-generated artifacts such as .DS_Store are common
		// in a Finder-browsed skills directory and must not block every
		// other plugin's repair/update. The planned active path is the
		// managed package, so a file or FIFO there is integrity failure.
		return clients.RegistryClear, true, refuseNonDirectoryClaudePath(path, plan.ActivePath)
	}
	return classifyClaudePreparedDirectory(path, plan, name, owned)
}

func classifyClaudePreparedDirectory(path string, plan domain.DeliveryPlan, name string, owned bool) (clients.RegistryFinding, bool, error) {
	manifestName, readErr := shared.ReadJSONManifestName(filepath.Join(path, ".claude-plugin", "plugin.json"))
	if os.IsNotExist(readErr) {
		// Plain skills legitimately share this directory and do not claim a
		// plugin identity.
		if _, skillErr := os.Lstat(filepath.Join(path, "SKILL.md")); skillErr == nil {
			return clients.RegistryClear, true, nil
		} else if !os.IsNotExist(skillErr) {
			return clients.RegistryIndeterminate, false, skillErr
		}
		return clients.RegistryIndeterminate, false, nil
	}
	if readErr != nil {
		return clients.RegistryIndeterminate, false, readErr
	}
	if manifestName != name {
		return clients.RegistryClear, true, nil
	}
	if shared.SameCleanPath(path, plan.ActivePath) && owned {
		return clients.RegistryExpected, false, nil
	}
	return clients.RegistryCollision, false, nil
}

func readClaudePreparedEntries(root string) ([]os.DirEntry, error) {
	if err := claudePreparedRoot(root); err != nil {
		return nil, err
	}
	return os.ReadDir(root)
}

func claudePreparedRoot(root string) error {
	meta, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if meta.Mode()&os.ModeSymlink == 0 {
		if meta.IsDir() {
			return nil
		}
		return fmt.Errorf("claude skills root is not a directory: %s", root)
	}
	// Lstat before ReadDir so a FIFO, socket, or dangling skills root cannot
	// look unused (ReadDir follows and reports ENOENT) or hang the preflight.
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errClaudeSkillSymlinkDangling, root)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", errClaudeSkillSymlinkNotDirectory, root)
	}
	return nil
}

func ignoreUnrelatedDanglingClaudeSkill(err error, path, activePath string) bool {
	return errors.Is(err, errClaudeSkillSymlinkDangling) && !shared.SameCleanPath(path, activePath)
}

func refuseNonDirectoryClaudePath(path, activePath string) error {
	if shared.SameCleanPath(path, activePath) {
		return fmt.Errorf("claude managed path is not a directory: %s", path)
	}
	return nil
}

func claudePreparedEntry(root string, entry os.DirEntry) (string, bool, error) {
	path := filepath.Join(root, entry.Name())
	meta, err := os.Lstat(path)
	if err != nil {
		return path, false, err
	}
	if meta.Mode()&os.ModeSymlink == 0 {
		return path, meta.IsDir(), nil
	}
	// Claude Code skills are commonly shared through symlinks. Follow the link
	// for read-only identity classification so a normal linked skill does not
	// block every unrelated plugin install. Classify via Lstat first so a host
	// that omits directory-entry types still fail-closes a dangling active path.
	// The caller ignores a dangling link only when it is unrelated to the
	// planned active path; all other unresolved or non-directory links remain
	// fail-closed.
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
