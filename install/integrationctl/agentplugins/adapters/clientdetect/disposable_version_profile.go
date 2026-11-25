package clientdetect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

// versionProbeEnvironment preserves the selected native host identity while
// shielding side-effecting --version processes from the user's profile.
func versionProbeEnvironment(adapter clients.Adapter, profile clients.VersionProbeEnvironment, originalRoot string) ([]string, func(), error) {
	noop := func() {}
	isolated, ok := adapter.(clients.DisposableVersionProbeProfile)
	if !ok || !isolated.RequiresDisposableVersionProbeProfile() {
		environment, err := profile.VersionProbeEnvironment(originalRoot)
		return environment, noop, err
	}
	// The ambient OS temp root must never be inside the selected user profile.
	// Resolve both roots before creating scratch content to prevent aliases.
	// Unix os.TempDir preserves a trailing TMPDIR separator; canonical checks require a clean path.
	tempBase, err := clients.CanonicalDirectory(filepath.Clean(os.TempDir()), os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return nil, noop, err
	}
	original, err := clients.CanonicalDirectory(originalRoot, os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return nil, noop, err
	}
	if original != originalRoot {
		// Default-deny preserves existing clients' profile authority. An adapter
		// may explicitly allow aliasing only for its isolated version probe.
		aliasPolicy, ok := adapter.(clients.AliasedVersionProbeProfile)
		if !ok || !aliasPolicy.AllowsAliasedVersionProbeProfile() {
			return nil, noop, fmt.Errorf("native version profile contains a symlink alias")
		}
	}
	relative, err := filepath.Rel(original, tempBase)
	if err != nil || relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return nil, noop, fmt.Errorf("native version temp root overlaps selected profile")
	}
	private, err := os.MkdirTemp(tempBase, "agentplugins-version-profile-")
	if err != nil {
		return nil, noop, err
	}
	cleanup := func() { _ = os.RemoveAll(private) }
	root, err := filepath.EvalSymlinks(private)
	if err != nil {
		cleanup()
		return nil, noop, err
	}
	environment, err := profile.VersionProbeEnvironment(root)
	if err != nil {
		cleanup()
		return nil, noop, err
	}
	return environment, cleanup, nil
}
