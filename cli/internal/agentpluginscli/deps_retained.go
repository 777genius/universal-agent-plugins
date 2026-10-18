package agentpluginscli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func savedClosureAliases(previous, current environment.Lock, aliases []string) []string {
	selected := map[string]bool{}
	for _, alias := range previous.ClosureAliases(aliases) {
		selected[alias] = true
	}
	for _, alias := range current.ClosureAliases(aliases) {
		selected[alias] = true
	}
	out := make([]string, 0, len(selected))
	for alias := range selected {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

func renderSavedRetention(cmd *cobra.Command, pkg environment.LockedPackage) error {
	consumers := map[string]bool{}
	for _, edge := range pkg.RequiredBy {
		consumers[edge.Alias] = true
	}
	names := make([]string, 0, len(consumers))
	for name := range consumers {
		names = append(names, name)
	}
	sort.Strings(names)
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s retained: required by %s\n", pkg.Alias, strings.Join(names, ", "))
	return err
}

// Lock-only dependencies have no installation receipt before the first sync.
func selectLockedLifecycle(app App, state domain.StateFileV2, requirements environment.Requirements, selector string, selection savedLifecycleSelection) (savedLifecycleSelection, error) {
	if _, root := requirements.Dependencies[selection.alias]; root {
		return selection, nil
	}
	_, lockPath := app.dependencyPaths()
	lock, err := readDependencyLock(lockPath, requirements)
	if os.IsNotExist(err) {
		return selection, nil
	}
	if err != nil {
		return selection, err
	}
	alias := selection.alias
	if alias == "" {
		alias = selector
	}
	pin, exists := lock.Package(alias)
	if !exists {
		return selection, nil
	}
	installation, err := savedAliasInstallation(state, alias, selector)
	if err != nil {
		return selection, err
	}
	selection.alias, selection.installation = alias, installation
	if _, root := requirements.Dependencies[alias]; !root && len(pin.RequiredBy) > 0 {
		selection.retained = &pin
	}
	return selection, nil
}

func renderSavedRetentionResult(cmd *cobra.Command, opts *options, pin environment.LockedPackage) error {
	if opts.format == "json" {
		return renderJSON(cmd.OutOrStdout(), map[string]any{"alias": pin.Alias, "retained": true, "required_by": pin.RequiredBy})
	}
	return renderSavedRetention(cmd, pin)
}
