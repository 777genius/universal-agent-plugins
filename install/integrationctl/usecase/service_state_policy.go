package usecase

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func defaultBool(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func desiredPolicyFromLock(in domain.InstallPolicy) domain.InstallPolicy {
	return domain.InstallPolicy{
		Scope:           defaultString(in.Scope, "project"),
		AutoUpdate:      in.AutoUpdate,
		AdoptNewTargets: defaultString(in.AdoptNewTargets, "manual"),
		AllowPrerelease: in.AllowPrerelease,
	}
}

func resolveWorkspaceLockSource(lockPath, source string) string {
	source = strings.TrimSpace(source)
	if source == "" || filepath.IsAbs(source) {
		return source
	}
	if strings.Contains(source, ":") && !strings.HasPrefix(source, ".") && !strings.HasPrefix(source, "..") {
		return source
	}
	return filepath.Join(filepath.Dir(lockPath), source)
}

func boolPtr(v bool) *bool { return &v }

func (s Service) workspaceRootForPolicy(policy domain.InstallPolicy) string {
	if !strings.EqualFold(strings.TrimSpace(policy.Scope), "project") {
		return ""
	}
	if root := strings.TrimSpace(s.CurrentWorkspaceRoot); root != "" {
		return filepath.Clean(root)
	}
	return ""
}
