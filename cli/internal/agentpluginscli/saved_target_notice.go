package agentpluginscli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func savedTargetNotice(lock environment.Lock, aliases, targets []string) string {
	if len(targets) == 0 {
		return ""
	}
	var retained []string
	for _, pkg := range lock.Packages {
		if !slices.Contains(aliases, pkg.Alias) {
			continue
		}
		for _, target := range pkg.Targets {
			if !slices.Contains(targets, target) && !slices.Contains(retained, target) {
				retained = append(retained, target)
			}
		}
	}
	if len(retained) == 0 {
		return ""
	}
	slices.Sort(retained)
	return fmt.Sprintf("This command selects %s. Saved targets %s are preserved and are not applied now; deps sync reconciles all saved targets.", strings.Join(targets, ", "), strings.Join(retained, ", "))
}

func renderSavedTargetNotice(cmd *cobra.Command, opts *options, lock environment.Lock, aliases, targets []string) error {
	notice := savedTargetNotice(lock, aliases, targets)
	if notice == "" {
		return nil
	}
	if opts.environmentOutput != nil {
		opts.environmentOutput["target_notice"] = notice
	}
	if opts.format == "human" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), notice)
		return err
	}
	return nil
}
