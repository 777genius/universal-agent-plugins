package kimi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

var _ clients.VersionProbeEnvironment = (*Adapter)(nil)

// Kimi bootstraps native workers and staged-update handling before printing its
// version. Bind its supported profile/cache controls without inventing a HOME.
func (*Adapter) VersionProbeEnvironment(root string) ([]string, error) {
	if root == "" || strings.TrimSpace(root) != root || strings.ContainsAny(root, "\x00\r\n") ||
		!utf8.ValidString(root) || !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return nil, fmt.Errorf("kimi version profile must be an explicit clean absolute directory")
	}
	parent, err := os.Lstat(filepath.Dir(root))
	if err != nil {
		return nil, fmt.Errorf("inspect kimi version profile parent: %w", err)
	}
	if !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("kimi version profile parent must be an ordinary directory")
	}
	cache := filepath.Join(root, "cache")
	for _, path := range []string{root, cache} {
		canonical, err := clients.CanonicalDirectory(path, os.Lstat, filepath.EvalSymlinks)
		if err != nil {
			return nil, err
		}
		if canonical != path {
			return nil, fmt.Errorf("kimi version profile/cache must not contain aliases")
		}
	}
	if err := pathpolicy.RequireContainedChild(root, cache); err != nil {
		return nil, err
	}
	return []string{"KIMI_CODE_HOME=" + root, "KIMI_CODE_CACHE_DIR=" + cache, "KIMI_CODE_NO_AUTO_UPDATE=1"}, nil
}
