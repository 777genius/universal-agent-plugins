package copilot

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (

	_ clients.VersionProbeEnvironment       = (*Adapter)(nil)
	_ clients.DisposableVersionProbeProfile = (*Adapter)(nil)
)

// A native directory marketplace must never be assumed from mere CLI
// presence: the project-overlay contract began at Copilot 1.0.81-8.
func (*Adapter) RequiresProjectVersion(client domain.DetectedClient) bool {
	return client.ClientID == domain.ClientCopilot
}

// Probe the genuine CLI only when available. Legacy Copilot project files
// continue to work if version discovery fails or is unrecognized.
func (*Adapter) ProjectVersionProbeBestEffort() bool { return true }

// --version runs only against a disposable profile, not the selected user's
// profile or ambient COPILOT_HOME, and cannot confer native registry authority.
func (*Adapter) RequiresDisposableVersionProbeProfile() bool { return true }

func (*Adapter) VersionProbeEnvironment(root string) ([]string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root ||
		strings.TrimSpace(root) != root || strings.ContainsAny(root, "\x00\r\n") {
		return nil, fmt.Errorf("copilot version probe requires a canonical absolute profile")
	}
	return []string{"COPILOT_HOME=" + root}, nil
}

// Only a normalized observed CLI version, never a user-provided directory,
// authorizes whole-plugin project registration. Older clients retain UAP's
// existing independently verified project components.
func copilotSupportsProjectPlugin(version string) bool {
	core, suffix, prerelease := strings.Cut(strings.TrimPrefix(version, "v"), "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	var values [3]int
	for i, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return false
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return false
		}
		values[i] = value
	}
	major, minor, patch := values[0], values[1], values[2]
	// A higher-looking prerelease is not evidence of this native capability.
	// Admit only the measured 1.0.81-N build family as an explicit exception.
	if prerelease {
		if major != 1 || minor != 0 || patch != 81 {
			return false
		}
		build, err := strconv.Atoi(suffix)
		return err == nil && build >= 8
	}
	return major > 1 || major == 1 && (minor > 0 || patch >= 81)
}
