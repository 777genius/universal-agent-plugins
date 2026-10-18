package agentpluginscli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

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
