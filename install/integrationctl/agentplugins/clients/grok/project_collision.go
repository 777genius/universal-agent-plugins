package grok

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const projectManifestLimit = 64 << 10
const projectDiscoveryLimit = 1 << 20
const projectDirectoryLimit = 128
const projectAncestorLimit = 64

type projectFileScanner struct {
	remaining int
	plan      domain.DeliveryPlan
	owned     bool
	expected  bool
}

func inspectProjectFiles(plan domain.DeliveryPlan, owned bool) (clients.RegistryFinding, error) {
	scanner := projectFileScanner{remaining: projectDiscoveryLimit, plan: plan, owned: owned}
	roots := []string{plan.TargetContext.UserConfigRoot, filepath.Join(filepath.Dir(plan.TargetContext.UserConfigRoot), ".claude")}
	current := plan.TargetContext.ProjectRoot
	for count := 0; ; count++ {
		if count >= projectAncestorLimit {
			return clients.RegistryIndeterminate, fmt.Errorf("grok project ancestor discovery exceeds finite bound")
		}
		roots = append(roots, filepath.Join(current, ".grok"), filepath.Join(current, ".claude"))
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	seen := map[string]bool{}
	for _, root := range roots {
		if seen[root] {
			continue
		}
		seen[root] = true
		collision, err := scanner.scanRoot(root)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if collision {
			return clients.RegistryCollision, nil
		}
	}
	if scanner.expected {
		return clients.RegistryExpected, nil
	}
	return clients.RegistryClear, nil
}
func (scanner *projectFileScanner) scanRoot(root string) (bool, error) {
	skills, err := projectDirectoryEntries(filepath.Join(root, "skills"), nil)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	for _, entry := range skills {
		if entry.Name() == scanner.plan.DeclaredName {
			return true, nil
		}
	}
	entries, err := projectDirectoryEntries(filepath.Join(root, "plugins"), nil)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("grok plugin discovery contains an unqualified entry")
		}
		path := filepath.Join(root, "plugins", entry.Name())
		name, err := scanner.pluginName(path)
		if err != nil {
			return false, err
		}
		if name != scanner.plan.DeclaredName {
			continue
		}
		if scanner.owned && path == scanner.plan.ActivePath {
			scanner.expected = true
			continue
		}
		return true, nil
	}
	return false, nil
}
func (scanner *projectFileScanner) pluginName(path string) (string, error) {
	selected := ""
	for _, layout := range []string{".grok-plugin", ".claude-plugin"} {
		manifest := filepath.Join(path, layout, "plugin.json")
		if _, err := os.Lstat(manifest); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return "", err
		}
		if selected != "" {
			return "", fmt.Errorf("grok plugin contains competing native manifests")
		}
		selected = manifest
	}
	if selected == "" {
		return "", fmt.Errorf("grok plugin discovery cannot exhaust an unknown plugin layout")
	}
	body, err := scanner.readProjectManifest(selected, nil)
	if err != nil {
		return "", err
	}
	object, err := shared.DecodeStrictJSONObject(body)
	if err != nil || !exactProjectFields(object, []string{"name"}) {
		return "", fmt.Errorf("grok plugin manifest identity is indeterminate")
	}
	name, ok := object["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("grok plugin manifest name is missing")
	}
	return name, nil
}
func (scanner *projectFileScanner) readProjectManifest(path string, beforeOpen func()) ([]byte, error) {
	file, info, err := openProjectPath(path, false, beforeOpen)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	limit := projectManifestLimit
	if scanner.remaining < limit {
		limit = scanner.remaining
	}
	if limit <= 0 || info.Size() > int64(limit) {
		return nil, fmt.Errorf("grok plugin manifest exceeds bounded discovery budget")
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	scanner.remaining -= len(body)
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, fmt.Errorf("grok plugin manifest exceeds bounded discovery budget")
	}
	if err := stableProjectPath(path, file, info); err != nil {
		return nil, err
	}
	return body, nil
}
func projectDirectoryEntries(path string, beforeOpen func()) ([]os.DirEntry, error) {
	file, info, err := openProjectPath(path, true, beforeOpen)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries, err := file.ReadDir(projectDirectoryLimit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > projectDirectoryLimit {
		return nil, fmt.Errorf("grok plugin directory exceeds bounded discovery entries")
	}
	if err := stableProjectPath(path, file, info); err != nil {
		return nil, err
	}
	return entries, nil
}
