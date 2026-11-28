package enterprisepolicy

import (
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validateRules(p *domain.InstallPolicy) error {
	for _, hosts := range [][]string{p.Sources.AllowHosts.Values, p.Sources.DenyHosts, p.MCP.AllowHosts.Values, p.MCP.DenyHosts} {
		for _, host := range hosts {
			if !domain.ValidPolicyHost(host) {
				return configError("invalid_hostname")
			}
		}
	}
	for _, list := range []*[]string{&p.Sources.AllowRepositories.Values, &p.Sources.DenyRepositories} {
		for i, raw := range *list {
			value, ok := canonicalPolicyRepository(raw)
			if !ok {
				return configError("invalid_repository_pattern")
			}
			(*list)[i] = value
		}
		slices.Sort(*list)
		*list = slices.Compact(*list)
	}
	if err := validateEnums(*p); err != nil {
		return err
	}
	for _, target := range p.Install.AllowedTargets.Values {
		if !domain.IsSupportedClient(domain.ClientID(target)) {
			return configError("invalid_target")
		}
	}
	return nil
}

func validateEnums(p domain.InstallPolicy) error {
	sets := []struct {
		values  []string
		allowed []string
	}{
		{p.Sources.AllowedKinds.Values, []string{"directory", "git", "archive", "web", "local"}},
		{p.MCP.AllowedTransports.Values, []string{"stdio", "streamable-http", "sse"}},
		{p.Install.AllowedScopes.Values, []string{"user", "project", "local"}},
		{p.Components.Denied, []string{"mcp", "skills", "commands", "apps", "extensions"}},
	}
	for _, set := range sets {
		for _, value := range set.values {
			if !slices.Contains(set.allowed, value) {
				return configError("invalid_rule_value")
			}
		}
	}
	return nil
}

func canonicalPolicyRepository(raw string) (string, bool) {
	if len(raw) > 2048 || raw != strings.TrimSpace(raw) {
		return "", false
	}
	host, repo, ok := strings.Cut(raw, "/")
	repo = strings.TrimSuffix(repo, ".git")
	if !ok || !domain.ValidPolicyHost(host) || !validRepositoryPath(repo, true) {
		return "", false
	}
	parts := strings.Split(repo, "/")
	if len(parts) < 2 || (host == "github.com" && len(parts) != 2) {
		return "", false
	}
	if host == "github.com" {
		repo = strings.ToLower(repo)
	}
	return host + "/" + repo, true
}

func validRepositoryPath(value string, wildcard bool) bool {
	parts := strings.Split(value, "/")
	for i, part := range parts {
		if wildcard && part == "*" && i == len(parts)-1 {
			continue
		}
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, r := range part {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' && r != '.' {
				return false
			}
		}
	}
	return true
}
