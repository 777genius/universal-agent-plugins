package pluginmodel

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

var geminiExtensionNameRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ValidateGeminiExtensionName(name string) error {
	name = strings.TrimSpace(name)
	if !geminiExtensionNameRe.MatchString(name) {
		return fmt.Errorf("invalid Gemini extension name %q: use lowercase letters, digits, and hyphens only", name)
	}
	return nil
}

func (m Manifest) Validate() error {
	if strings.TrimSpace(m.APIVersion) != APIVersionV1 {
		return fmt.Errorf("invalid plugin.yaml: api_version must be %q", APIVersionV1)
	}
	if err := scaffold.ValidateProjectName(m.Name); err != nil {
		return fmt.Errorf("invalid plugin.yaml: %w", err)
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("invalid plugin.yaml: version required")
	}
	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("invalid plugin.yaml: description required")
	}
	if len(m.Targets) == 0 {
		return fmt.Errorf("invalid plugin.yaml: targets must not be empty")
	}
	seen := map[string]struct{}{}
	supportedTargets := platformmeta.IDs()
	for _, target := range m.Targets {
		target = NormalizeTarget(target)
		if target == "codex" {
			return fmt.Errorf("invalid plugin.yaml: target %q was split; use %q for the official plugin bundle and/or %q for repo-local notify integration", target, "codex-package", "codex-runtime")
		}
		if !slices.Contains(supportedTargets, target) {
			return fmt.Errorf("invalid plugin.yaml: unsupported target %q", target)
		}
		if _, ok := seen[target]; ok {
			return fmt.Errorf("invalid plugin.yaml: duplicate target %q", target)
		}
		seen[target] = struct{}{}
	}
	if _, ok := seen["gemini"]; ok {
		if err := ValidateGeminiExtensionName(m.Name); err != nil {
			return fmt.Errorf("invalid plugin.yaml: %w", err)
		}
	}
	return nil
}

func (l Launcher) Validate() error {
	if _, ok := scaffold.LookupRuntime(l.Runtime); !ok {
		return fmt.Errorf("invalid %s: unsupported runtime %q", LauncherFileName, l.Runtime)
	}
	if strings.TrimSpace(l.Entrypoint) == "" {
		return fmt.Errorf("invalid %s: entrypoint required", LauncherFileName)
	}
	return nil
}

func (m Manifest) EnabledTargets() []string {
	out := make([]string, 0, len(m.Targets))
	for _, target := range m.Targets {
		out = append(out, NormalizeTarget(target))
	}
	return out
}

func (m Manifest) SelectedTargets(target string) ([]string, error) {
	target = NormalizeTarget(target)
	if target == "" || target == "all" {
		return m.EnabledTargets(), nil
	}
	for _, enabled := range m.EnabledTargets() {
		if enabled == target {
			return []string{target}, nil
		}
	}
	return nil, fmt.Errorf("target %q is not enabled in plugin.yaml", target)
}
