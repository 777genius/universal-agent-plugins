package main

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

func newPublicationMaterializeCmd(runner publicationMaterializeRunner) *cobra.Command {
	var target string
	var dest string
	var packageRoot string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "materialize [path]",
		Short: "Materialize a safe local marketplace root for Codex or Claude",
		Long: `Create or update a local marketplace root for a single publication-capable package target.

This workflow is intentionally limited to documented local/catalog flows:
- Codex marketplace roots with .agents/plugins/marketplace.json
- Claude marketplace roots with .claude-plugin/marketplace.json

It copies the materialized package bundle under a managed package root, then merges or creates the marketplace catalog artifact.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			result, err := runner.PublicationMaterialize(app.PluginPublicationMaterializeOptions{
				Root:        root,
				Target:      target,
				Dest:        dest,
				PackageRoot: packageRoot,
				DryRun:      dryRun,
			})
			if err != nil {
				return err
			}
			for _, line := range result.Lines {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", `materialization target ("claude" or "codex-package")`)
	cmd.Flags().StringVar(&dest, "dest", "", "destination marketplace root directory")
	cmd.Flags().StringVar(&packageRoot, "package-root", "", "relative package root inside the destination marketplace root (default: plugins/<name>)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the materialized package root and catalog changes without writing them")
	_ = cmd.MarkFlagRequired("target")
	_ = cmd.MarkFlagRequired("dest")
	return cmd
}

func newPublicationRemoveCmd(runner publicationMaterializeRunner) *cobra.Command {
	var target string
	var dest string
	var packageRoot string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "remove [path]",
		Short: "Remove a materialized local marketplace package root and catalog entry",
		Long: `Remove a single plugin from a local Codex or Claude marketplace root.

This workflow is intentionally scoped to documented local/catalog flows and is safe to rerun.
It removes the selected package root and prunes the matching plugin entry from the marketplace catalog while preserving the marketplace root itself.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			result, err := runner.PublicationRemove(app.PluginPublicationRemoveOptions{
				Root:        root,
				Target:      target,
				Dest:        dest,
				PackageRoot: packageRoot,
				DryRun:      dryRun,
			})
			if err != nil {
				return err
			}
			for _, line := range result.Lines {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", `removal target ("claude" or "codex-package")`)
	cmd.Flags().StringVar(&dest, "dest", "", "destination marketplace root directory")
	cmd.Flags().StringVar(&packageRoot, "package-root", "", "relative package root inside the destination marketplace root (default: plugins/<name>)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the package root and catalog pruning without writing changes")
	_ = cmd.MarkFlagRequired("target")
	_ = cmd.MarkFlagRequired("dest")
	return cmd
}
