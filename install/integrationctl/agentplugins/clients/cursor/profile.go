package cursor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProfileResolver         = (*Adapter)(nil)
	_ clients.ProfileBindingValidator = (*Adapter)(nil)
)

func profileSpelling(root string) error {
	if !utf8.ValidString(root) || len(root) > 4096 || !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root || strings.IndexFunc(root, unicode.IsControl) >= 0 || strings.HasPrefix(root, `\\`) || strings.HasPrefix(root, "//") {
		return fmt.Errorf("cursor profile must be an explicit clean absolute directory")
	}
	return nil
}

// ResolveProfileRoot selects physical filesystem authority, without creating
// missing parents. It does not assert that a native client reads this directory.
// Existing symlink aliases resolve before the shared no-follow writer policy.
func (*Adapter) ResolveProfileRoot(root string) (string, error) {
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	root, err := clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return "", err
	}
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	return root, nil
}

func detectedProfileRoot(host clients.Host) (string, error) {
	// The pinned editor source uses pathService.userHome()/.cursor, independently
	// of --user-data-dir. No CLI environment redirect is inferred from that fact.
	if err := profileSpelling(host.HomeDir()); err != nil {
		return "", err
	}
	root, err := host.CanonicalDirectory(filepath.Join(host.HomeDir(), ".cursor"))
	if err != nil {
		return "", err
	}
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	return root, nil
}

// ValidateBindingProfile checks the recorded package locator against the
// selected physical root. Legacy preparation has no native activation receipt;
// only an exact artifact path can establish its filesystem profile authority.
// Directory replacement identity still requires the shared UC-P seam.
func (adapter *Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	selected, err := adapter.ResolveProfileRoot(root)
	if err != nil {
		return err
	}
	if err := (pathpolicy.Policy{}).ValidateLeafID(binding.PhysicalArtifact); err != nil {
		return err
	}
	expected := filepath.Join(selected, "plugins", "local", binding.PhysicalArtifact)
	if binding.TargetLocator != expected {
		return fmt.Errorf("recorded Cursor package belongs to another profile")
	}
	if binding.NativeProfileRoot != "" && binding.NativeProfileRoot != selected {
		return fmt.Errorf("recorded Cursor native profile differs from selected root")
	}
	return nil
}
