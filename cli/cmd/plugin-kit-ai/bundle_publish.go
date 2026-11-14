package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

func newBundlePublishCmd(runner bundleRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish [path]",
		Short: "Publish an exported Python/Node bundle to GitHub Releases",
		Long: `Publish an exported Python/Node bundle to GitHub Releases.

This stable producer-side handoff surface exports a bundle, creates a published release by default,
uses --draft to keep the release as draft, uploads the bundle plus a sibling .sha256 asset,
and remains separate from the binary-only plugin-kit-ai install flow.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			result, err := runner.BundlePublish(ctx, buildBundlePublishOptions(args))
			if err != nil {
				return err
			}
			for _, line := range result.Lines {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bundlePublishPlatform, "platform", "", "target platform to export and publish (codex-runtime or claude)")
	cmd.Flags().StringVar(&bundlePublishRepo, "repo", "", "GitHub owner/repo that will receive the bundle assets")
	cmd.Flags().StringVar(&bundlePublishTag, "tag", "", "GitHub release tag to reuse or create")
	cmd.Flags().BoolVar(&bundlePublishDraft, "draft", false, "keep the target release as draft instead of published")
	cmd.Flags().StringVar(&bundlePublishGitHubToken, "github-token", "", "GitHub token (optional; default from GITHUB_TOKEN env)")
	cmd.Flags().StringVar(&bundlePublishGitHubAPIBase, "github-api-base", "", "GitHub API base URL override (for tests or GitHub Enterprise)")
	cmd.Flags().BoolVarP(&bundlePublishForce, "force", "f", false, "replace existing bundle assets with the same name")
	_ = cmd.MarkFlagRequired("platform")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("tag")
	_ = cmd.Flags().MarkHidden("github-api-base")
	return cmd
}

func buildBundlePublishOptions(args []string) app.PluginBundlePublishOptions {
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	token := strings.TrimSpace(bundlePublishGitHubToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	return app.PluginBundlePublishOptions{
		Root:          root,
		Platform:      bundlePublishPlatform,
		Repo:          bundlePublishRepo,
		Tag:           bundlePublishTag,
		Draft:         bundlePublishDraft,
		GitHubToken:   token,
		GitHubAPIBase: bundlePublishGitHubAPIBase,
		Force:         bundlePublishForce,
	}
}
