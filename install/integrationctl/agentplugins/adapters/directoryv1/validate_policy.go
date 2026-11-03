package directoryv1

import (
	"fmt"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validatePolicy(p domain.DirectoryReleasePolicy) error {
	if p.Status != domain.ReleaseActive && p.Status != domain.ReleaseSuperseded && p.Status != domain.ReleaseRevoked {
		return fmt.Errorf("invalid status")
	}
	if _, err := parseSemver(p.MinimumInstallerVersion); err != nil {
		return fmt.Errorf("invalid minimum installer version")
	}
	if len(p.Targets) == 0 || p.CurrentEvidence == nil {
		return fmt.Errorf("targets/evidence arrays required")
	}
	clients := map[domain.ClientID]bool{}
	for _, t := range p.Targets {
		expectedDelivery, supportedDelivery := domain.ExpectedDirectoryDelivery(t.Client)
		if !validClient(t.Client) || clients[t.Client] || len(t.Scopes) != 1 || t.Scopes[0] != domain.ScopeUser || !supportedDelivery || t.Delivery != expectedDelivery {
			return fmt.Errorf("invalid target")
		}
		clients[t.Client] = true
		if t.Authentication != domain.AuthenticationRequirementNotRequired && t.Authentication != domain.AuthenticationRequirementRequired && t.Authentication != domain.AuthenticationRequirementUnknown {
			return fmt.Errorf("invalid target authentication")
		}
		definition, ok := domain.ClientDefinitionFor(t.Client)
		if ok && definition.Capabilities.AppSupport != domain.SupportUnsupported {
			if t.AppBinding == nil || t.AppBinding.AppKey == "" || t.AppBinding.ID == "" || t.AppBinding.MCPServer == "" {
				return fmt.Errorf("chatgpt app binding required")
			}
			if err := domain.ValidateAppBindingIdentity(t.AppBinding.AppKey, t.AppBinding.ID, t.AppBinding.MCPServer); err != nil {
				return fmt.Errorf("invalid chatgpt app binding: %w", err)
			}
		} else if t.AppBinding != nil {
			return fmt.Errorf("app binding only allowed for chatgpt")
		}
	}
	seen := map[string]bool{}
	for _, id := range p.CurrentEvidence {
		if !evidenceIDPattern.MatchString(id) || seen[id] {
			return fmt.Errorf("invalid current evidence ID")
		}
		seen[id] = true
	}
	return nil
}

func validateSource(s domain.DirectorySource) error {
	if !repositoryPattern.MatchString(s.Repository) || !shaPattern.MatchString(s.Revision) {
		return fmt.Errorf("malformed immutable source")
	}
	return validateSourcePath(s.Path)
}

func validateSourcePath(sourcePath string) error {
	if sourcePath == "" || strings.HasPrefix(sourcePath, "/") || strings.Contains(sourcePath, "\\") {
		return fmt.Errorf("unsafe source path")
	}
	for _, part := range strings.Split(sourcePath, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("unsafe source path")
		}
		for _, r := range part {
			if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-", r) {
				return fmt.Errorf("unsafe source path")
			}
		}
	}
	return nil
}
func parseTimestamp(v string) (time.Time, error) {
	if !timestampPattern.MatchString(v) {
		return time.Time{}, fmt.Errorf("must be second-precision UTC RFC3339")
	}
	return time.Parse("2006-01-02T15:04:05Z", v)
}
func parseSemver(v string) ([3]uint64, error) {
	var out [3]uint64
	if !semverPattern.MatchString(v) {
		return out, fmt.Errorf("not x.y.z")
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("not x.y.z")
	}
	for i, s := range parts {
		if s == "" || (len(s) > 1 && s[0] == '0') {
			return out, fmt.Errorf("invalid component")
		}
		n, e := fmt.Sscan(s, &out[i])
		if e != nil || n != 1 {
			return out, fmt.Errorf("invalid component")
		}
	}
	return out, nil
}
func validClient(v domain.ClientID) bool {
	return domain.IsSupportedClient(v)
}
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
func releaseKey(distribution string, sequence uint64) string {
	return fmt.Sprintf("%s#%d", distribution, sequence)
}
func uniqueSimpleIDs(values []string, where string) error {
	seen := map[string]bool{}
	for _, v := range values {
		if !simpleIDPattern.MatchString(v) || seen[v] {
			return strictError("%s contains malformed or duplicate %q", where, v)
		}
		seen[v] = true
	}
	return nil
}
func uniqueDistributionIDs(values []string, where string) error {
	seen := map[string]bool{}
	for _, v := range values {
		if !distributionPattern.MatchString(v) || seen[v] {
			return strictError("%s contains malformed or duplicate %q", where, v)
		}
		seen[v] = true
	}
	return nil
}
func strictError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrStrictJSON, fmt.Sprintf(format, args...))
}
