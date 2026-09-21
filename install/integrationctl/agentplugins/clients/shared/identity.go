package shared

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var errPluginRootDangling = errors.New("plugin root symlink target does not exist")

// InspectUnqualifiedPluginRoot is the default prepared-registry inspection: it
// walks a directory of plugin-shaped entries and reports whether the declared
// name is free, already ours, or claimed by someone else.
//
// Anything it cannot read turns into Indeterminate rather than Clear, because
// "we could not look" must never become evidence of absence.
func InspectUnqualifiedPluginRoot(root, name, activePath string, owned bool) (clients.RegistryFinding, error) {
	if strings.TrimSpace(root) == "" {
		return clients.RegistryIndeterminate, nil
	}
	entries, err := readPluginRootEntries(root)
	if os.IsNotExist(err) {
		return clients.RegistryClear, nil
	}
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	finding := clients.RegistryClear
	for _, entry := range entries {
		next, skip, classErr := classifyUnqualifiedPluginEntry(root, entry, name, activePath, owned)
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

func readPluginRootEntries(root string) ([]os.DirEntry, error) {
	meta, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if meta.Mode()&os.ModeSymlink != 0 {
		info, statErr := os.Stat(root)
		if os.IsNotExist(statErr) {
			return nil, fmt.Errorf("%w: %s", errPluginRootDangling, root)
		}
		if statErr != nil {
			return nil, statErr
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("plugin root is not a directory: %s", root)
		}
	} else if !meta.IsDir() {
		return nil, fmt.Errorf("plugin root is not a directory: %s", root)
	}
	return os.ReadDir(root)
}

func classifyUnqualifiedPluginEntry(root string, entry os.DirEntry, name, activePath string, owned bool) (clients.RegistryFinding, bool, error) {
	if strings.HasPrefix(entry.Name(), ".agentplugins-staging-") {
		return clients.RegistryClear, true, nil
	}
	path := filepath.Join(root, entry.Name())
	meta, err := os.Lstat(path)
	if err != nil {
		return clients.RegistryIndeterminate, false, err
	}
	if meta.Mode()&os.ModeSymlink != 0 {
		return clients.RegistryIndeterminate, false, nil
	}
	if !meta.IsDir() {
		// A plain file cannot contain the manifest this scheme requires, so it
		// can never claim a competing plugin identity. OS-generated artifacts
		// such as .DS_Store are common here and must not block every other
		// plugin's repair/update. The planned active path is the managed
		// package, so a file or FIFO there is integrity failure.
		if activePath != "" && SameCleanPath(path, activePath) {
			return clients.RegistryIndeterminate, false, fmt.Errorf("managed path is not a directory: %s", path)
		}
		return clients.RegistryClear, true, nil
	}
	manifestName, qualified, namespace, err := NativeManifestIdentity(path)
	if err != nil {
		return clients.RegistryIndeterminate, false, err
	}
	if manifestName != name {
		return clients.RegistryClear, true, nil
	}
	if qualified && namespace != "" && namespace != ManagedMarketplaceName(filepath.Base(activePath)) {
		return clients.RegistryClear, true, nil
	}
	if activePath != "" && SameCleanPath(path, activePath) && owned {
		return clients.RegistryExpected, false, nil
	}
	return clients.RegistryCollision, false, nil
}

// NativeManifestIdentity reads the authoritative identity of a delivered native
// package: the prepared marketplace document when there is one (qualified by
// its namespace), otherwise the plain plugin manifest.
func NativeManifestIdentity(root string) (name string, qualified bool, namespace string, err error) {
	for _, marketplace := range []string{filepath.Join(root, ".agents", "plugins", "marketplace.json"), filepath.Join(root, ".github", "plugin", "marketplace.json")} {
		body, readErr := os.ReadFile(marketplace)
		if readErr == nil {
			return marketplaceIdentity(body)
		}
		if !os.IsNotExist(readErr) {
			return "", false, "", readErr
		}
	}
	for _, manifest := range []string{filepath.Join(root, ".claude-plugin", "plugin.json"), filepath.Join(root, "plugin.json"), filepath.Join(root, ".cursor-plugin", "plugin.json"), filepath.Join(root, ".codex-plugin", "plugin.json")} {
		name, readErr := ReadJSONManifestName(manifest)
		if readErr == nil {
			return name, false, "", nil
		}
		if !os.IsNotExist(readErr) {
			return "", false, "", readErr
		}
	}
	return "", false, "", fmt.Errorf("native package has no recognized authoritative manifest")
}

func marketplaceIdentity(body []byte) (name string, qualified bool, namespace string, err error) {
	value, decodeErr := DecodeStrictJSONObject(body)
	if decodeErr != nil {
		return "", false, "", decodeErr
	}
	namespace, _ = value["name"].(string)
	plugins, ok := value["plugins"].([]any)
	if namespace == "" || !ok || len(plugins) != 1 {
		return "", false, "", fmt.Errorf("invalid prepared marketplace identity")
	}
	plugin, ok := plugins[0].(map[string]any)
	if !ok {
		return "", false, "", fmt.Errorf("invalid prepared marketplace plugin identity")
	}
	name, ok = plugin["name"].(string)
	if !ok || name == "" {
		return "", false, "", fmt.Errorf("invalid prepared marketplace plugin name")
	}
	return name, true, namespace, nil
}

// SameCleanPath reports whether two paths denote the same location lexically,
// after making both absolute. It is not a symlink-aware comparison.
func SameCleanPath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbsolute) == filepath.Clean(rightAbsolute)
}

// ManagedPackageDigest returns the digest recorded for the managed package
// directory of a binding, or "" when the binding owns no such object.
func ManagedPackageDigest(client domain.ClientBinding) string {
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" {
			return object.ManagedDigest
		}
	}
	return ""
}
