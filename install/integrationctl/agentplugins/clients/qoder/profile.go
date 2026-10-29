package qoder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

var (
	_ clients.ProfileResolver         = (*Adapter)(nil)
	_ clients.VersionProbeEnvironment = (*Adapter)(nil)
)

func profileSpelling(root string) error {
	if !utf8.ValidString(root) || len(root) > 4096 || strings.TrimSpace(root) != root ||
		!filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root ||
		strings.IndexFunc(root, unicode.IsControl) >= 0 || strings.HasPrefix(root, `\\`) || strings.HasPrefix(root, "//") {
		return fmt.Errorf("qoder config root must be an explicit clean absolute directory")
	}
	return nil
}

// ResolveProfileRoot resolves existing aliases without creating a profile or
// replacing invalid explicit input with an ambient default.
func (*Adapter) ResolveProfileRoot(root string) (string, error) {
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	canonical, err := clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return "", err
	}
	if err := profileSpelling(canonical); err != nil {
		return "", err
	}
	return canonical, nil
}

func detectedProfileRoot(host clients.Host) (string, error) {
	root := host.Env("QODER_CONFIG_DIR")
	if root == "" {
		if err := profileSpelling(host.HomeDir()); err != nil {
			return "", err
		}
		root = filepath.Join(host.HomeDir(), ".qoder")
	}
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	canonical, err := host.CanonicalDirectory(root)
	if err != nil {
		return "", err
	}
	if err := profileSpelling(canonical); err != nil {
		return "", err
	}
	return canonical, nil
}

func validateProfile(root, planned string) error {
	canonical, err := New().ResolveProfileRoot(root)
	if err != nil {
		return err
	}
	if canonical != root {
		return fmt.Errorf("qoder config root must be canonical before native operations")
	}
	if planned != "" && planned != root {
		return fmt.Errorf("qoder planned registry root differs from client config root")
	}
	return nil
}

// Diagnostic probes use the same profile environment as native lifecycle
// commands; a bare version probe must not fall back to the default home.
func (*Adapter) VersionProbeEnvironment(root string) ([]string, error) {
	if err := validateProfile(root, ""); err != nil {
		return nil, err
	}
	return []string{"QODER_CONFIG_DIR=" + root}, nil
}
