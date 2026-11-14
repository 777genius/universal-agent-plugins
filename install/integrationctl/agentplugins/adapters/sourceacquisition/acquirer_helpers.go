package sourceacquisition

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

func normalizeSubpath(value string) (string, error) {
	value = strings.TrimSpace(strings.Trim(value, "/"))
	if value == "" {
		return "", nil
	}
	if strings.Contains(value, `\`) || path.IsAbs(value) || path.Clean(value) != value {
		return "", fmt.Errorf("plugin subpath must be a normalized repository-relative directory")
	}
	for _, segment := range strings.Split(value, "/") {
		if err := pathpolicy.ValidatePortablePathSegment(segment); err != nil {
			return "", fmt.Errorf("unsafe plugin subpath %q: %w", value, err)
		}
	}
	return value, nil
}

func (acquirer Acquirer) runner() Runner {
	if acquirer.Runner != nil {
		return acquirer.Runner
	}
	return OSRunner{}
}

func (acquirer Acquirer) digester() packagedigest.Builder {
	digester := acquirer.Digester
	if digester.TempRoot == "" {
		digester.TempRoot = acquirer.TempRoot
	}
	return digester
}

func (acquirer Acquirer) now() time.Time {
	if acquirer.Now != nil {
		return acquirer.Now().UTC()
	}
	return time.Now().UTC()
}

// CanonicalGitHubRepository validates the same repository grammar used by Git
// acquisition and returns GitHub's case-insensitive owner/repository identity.
func CanonicalGitHubRepository(repository string) (string, error) {
	if !repositoryPattern.MatchString(repository) {
		return "", fmt.Errorf("github repository must be owner/repo")
	}
	return strings.ToLower(repository), nil
}
