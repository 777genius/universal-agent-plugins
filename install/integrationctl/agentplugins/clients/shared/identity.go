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

// ErrNoAuthoritativeManifest is returned when a package directory has none of
// the recognized plugin or marketplace identity documents.
var ErrNoAuthoritativeManifest = errors.New("native package has no recognized authoritative manifest")

// InspectUnqualifiedPluginRoot is the default prepared-registry inspection: it
// walks a directory of plugin-shaped entries and reports whether the declared
// name is free, already ours, or claimed by someone else.
//
// A shared plugins root is not a private registry. Files, dangling links,
// empty directories, and unreadable foreign siblings are skipped. Only a
// proven same-name claim collides. Unreadable or malformed identity on the
// owned ActivePath is Indeterminate because that is our object.
func InspectUnqualifiedPluginRoot(root, name, activePath string, owned bool) (clients.RegistryFinding, error) {
	if strings.TrimSpace(root) == "" {
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
		if strings.HasPrefix(entry.Name(), ".agentplugins-staging-") {
			continue
		}
		path, ok := PluginDirectoryPath(root, entry)
		if !ok {
			continue
		}
		manifestName, qualified, namespace, err := NativeManifestIdentity(path)
		if err != nil {
			if errors.Is(err, ErrNoAuthoritativeManifest) {
				continue
			}
			if owned && activePath != "" && SameOwnedPluginDirectory(path, activePath) {
				return clients.RegistryIndeterminate, err
			}
			continue
		}
		if manifestName != name {
			continue
		}
		if qualified && namespace != "" && namespace != ManagedMarketplaceName(filepath.Base(activePath)) {
			continue
		}
		if activePath != "" && SameOwnedPluginDirectory(path, activePath) && owned {
			finding = clients.RegistryExpected
			continue
		}
		return clients.RegistryCollision, nil
	}
	return finding, nil
}

// PluginDirectoryPath reports whether a shared plugins/skills-root entry is a
// directory that may hold a plugin claim. Files, dangling symlinks, and
// symlinks to non-directories cannot claim an identity.
func PluginDirectoryPath(root string, entry os.DirEntry) (string, bool) {
	path := filepath.Join(root, entry.Name())
	if entry.Type()&os.ModeSymlink != 0 {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return "", false
		}
		return path, true
	}
	if !entry.IsDir() {
		return "", false
	}
	return path, true
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
	return "", false, "", ErrNoAuthoritativeManifest
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

// EquivalentLocalPath compares filesystem identity, not only spelling. macOS
// commonly exposes /tmp through /private/tmp and /var through /private/var.
// Identical cleaned paths are accepted without a stat; differing paths must
// both exist and be the same file.
func EquivalentLocalPath(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	left, right = filepath.Clean(leftAbs), filepath.Clean(rightAbs)
	if left == right {
		return true
	}
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}

// SameOwnedPluginDirectory reports whether path is the owned ActivePath.
// A directory symlink with a different leaf name is not owned even when it
// points at the same directory: Claude lists that symlink as its own slot.
func SameOwnedPluginDirectory(path, activePath string) bool {
	if !EquivalentLocalPath(path, activePath) {
		return false
	}
	return filepath.Base(filepath.Clean(path)) == filepath.Base(filepath.Clean(activePath))
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
