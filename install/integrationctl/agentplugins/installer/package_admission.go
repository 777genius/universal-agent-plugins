package installer

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validateAcquiredRequest(req Request) error {
	acquired := req.AcquiredSnapshot
	key, err := validateAcquiredSource(acquired, req.EnvironmentAuthority != nil)
	if err != nil {
		return err
	}
	if req.EnvironmentAuthority != nil {
		if acquired.OriginMode == domain.OriginModeDirectory {
			if acquired.Directory == nil {
				return fmt.Errorf("%w: directory source is missing publication identity", ErrInvalidRequest)
			}
			key = "directory:" + strings.ToLower(acquired.Directory.DistributionID)
		}
		if req.EnvironmentAuthority.SourceKey != key {
			return fmt.Errorf("%w: environment source key differs from acquisition", ErrInvalidRequest)
		}
	}
	if acquired.OriginMode != domain.OriginModeDirect && acquired.OriginMode != domain.OriginModeDirectory {
		return ErrInvalidRequest
	}
	if len(req.Targets) != 0 {
		return fmt.Errorf("%w: acquired snapshots support single-target requests only", ErrInvalidRequest)
	}
	if acquired.TreeDigest == "" || acquired.DigestAlgorithm != domain.TreeDigestAlgorithm {
		return fmt.Errorf("%w: acquired snapshot digest identity is incomplete", ErrInvalidRequest)
	}
	if req.ExecutableFiles != nil && !equalStrings(req.ExecutableFiles, acquired.ExecutableFiles) {
		return fmt.Errorf("%w: request executable inventory differs from acquired snapshot", ErrInvalidRequest)
	}
	return nil
}
func validateAcquiredExecutables(root string, files []string) error {
	for _, relative := range files {
		if relative == "." || relative == ".." || path.IsAbs(relative) || path.Clean(relative) != relative || strings.HasPrefix(relative, "../") || strings.Contains(relative, `\`) || filepath.VolumeName(relative) != "" {
			return fmt.Errorf("%w: acquired snapshot executable path is invalid", ErrInvalidRequest)
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("%w: acquired snapshot executable is not a regular package file", ErrInvalidRequest)
		}
	}
	return nil
}

// Native configuration adapters need only an explicit profile. CLI registries
// and host-profile probes also require the caller's executable authority.
func requiresExecutable(registry *clients.Registry, id domain.ClientID, scopes ...domain.InstallScope) bool {
	scope := domain.ScopeUser
	if len(scopes) > 0 && scopes[0] != "" {
		scope = scopes[0]
	}
	if (domain.SelectedDelivery{}).EffectiveTraitsForScope(id, scope).LifecycleKind == domain.LifecycleCLIRegistry {
		return true
	}
	consumer, ok := clients.As[clients.OpenCodeHostProfileConsumer](registry, id)
	return ok && consumer.UsesOpenCodeHostProfile()
}
