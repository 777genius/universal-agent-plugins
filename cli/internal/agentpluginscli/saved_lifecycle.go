package agentpluginscli

import (
	"fmt"
	"slices"
	"sort"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// resolveSavedAlias resolves intent and live ownership together. A name never
// silently selects an alias when an unrelated installation has the same name.
func resolveSavedAlias(state domain.StateFileV2, requirements environment.Requirements, selector string, environmentIDs ...string) (string, *domain.Installation, error) {
	aliases := matchingSavedAliases(state, requirements, selector, environmentIDs...)
	if len(aliases) == 0 {
		return "", nil, nil
	}
	if len(aliases) != 1 {
		return "", nil, ambiguousSavedSelector(selector)
	}
	var alias string
	for key := range aliases {
		alias = key
	}
	installation, err := savedAliasInstallation(state, alias, selector, environmentIDs...)
	return alias, installation, err
}

func ambiguousSavedSelector(selector string) error {
	return fmt.Errorf("ambiguous dependency %q; specify an installation ID", selector)
}

func matchingSavedAliases(state domain.StateFileV2, requirements environment.Requirements, selector string, environmentIDs ...string) map[string]bool {
	aliases := map[string]bool{}
	if requirement, ok := requirements.Dependencies[selector]; ok {
		source := mustSourceKey(requirement)
		for _, receipt := range state.EnvironmentReceipts {
			if source != "" && receipt.EnvironmentID == selectedEnvironmentID(environmentIDs) && receipt.SourceKey == source {
				aliases[receipt.Alias] = true
			}
		}
		if len(aliases) == 0 {
			aliases[selector] = true
		}
	}
	names := make(map[string]string, len(state.Installations))
	for _, installation := range state.Installations {
		names[installation.InstallationID] = installation.DeclaredName
	}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID != selectedEnvironmentID(environmentIDs) {
			continue
		}
		if receipt.Alias == selector || receipt.InstallationID == selector || names[receipt.InstallationID] == selector {
			aliases[receipt.Alias] = true
		}
	}
	return aliases
}

func ownsSavedAlias(state domain.StateFileV2, installationID, alias string, environmentIDs ...string) bool {
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID == selectedEnvironmentID(environmentIDs) && receipt.InstallationID == installationID && receipt.Alias == alias {
			return true
		}
	}
	return false
}

func savedAliasInstallation(state domain.StateFileV2, alias, selector string, environmentIDs ...string) (*domain.Installation, error) {
	var selected *domain.Installation
	for index := range state.Installations {
		installation := &state.Installations[index]
		owned := ownsSavedAlias(state, installation.InstallationID, alias, environmentIDs...)
		if (installation.InstallationID == selector || installation.DeclaredName == selector) && !owned {
			return nil, ambiguousSavedSelector(selector)
		}
		if !owned {
			continue
		}
		if selected != nil && selected.InstallationID != installation.InstallationID {
			return nil, ambiguousSavedSelector(selector)
		}
		selected = installation
	}
	return selected, nil
}

func savedAliasTargets(requirements environment.Requirements, state domain.StateFileV2, alias string, environmentIDs ...string) []string {
	declaration := alias
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID == selectedEnvironmentID(environmentIDs) && receipt.Alias == alias {
			for key, requirement := range requirements.Dependencies {
				if receipt.SourceKey != "" && mustSourceKey(requirement) == receipt.SourceKey {
					declaration = key
				}
			}
		}
	}
	targets := map[string]bool{}
	for target, aliases := range requirements.Targets {
		if slices.Contains(aliases, declaration) {
			targets[target] = true
		}
	}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID == selectedEnvironmentID(environmentIDs) && receipt.Alias == alias {
			targets[receipt.ClientID] = true
		}
	}
	result := make([]string, 0, len(targets))
	for target := range targets {
		result = append(result, target)
	}
	sort.Strings(result)
	return result
}

func removeSavedTargets(requirements environment.Requirements, alias string, targets []string) environment.Requirements {
	out := environment.Requirements{SchemaVersion: requirements.SchemaVersion, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{}}
	for key, value := range requirements.Dependencies {
		out.Dependencies[key] = value
	}
	remaining := false
	for target, aliases := range requirements.Targets {
		kept := append([]string(nil), aliases...)
		if slices.Contains(targets, target) {
			kept = slices.DeleteFunc(kept, func(value string) bool { return value == alias })
		}
		if len(kept) > 0 {
			out.Targets[target] = kept
		}
		if slices.Contains(kept, alias) {
			remaining = true
		}
	}
	if !remaining {
		delete(out.Dependencies, alias)
	}
	return out
}
