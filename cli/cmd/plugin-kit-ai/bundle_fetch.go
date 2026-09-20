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

func newBundleFetchCmd(runner bundleRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch [owner/repo]",
		Short: "Fetch and install a remote exported Python/Node bundle into a destination directory",
		Long: `Fetch a remote exported Python/Node bundle and install it into a destination directory.

Use either a direct HTTPS bundle URL with --url or a GitHub release reference as owner/repo plus --tag or --latest.
This stable remote handoff surface is intentionally separate from the binary-only plugin-kit-ai install flow.`,
		Args: validateBundleFetchArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			result, err := runner.BundleFetch(ctx, buildBundleFetchOptions(args))
			if err != nil {
				return err
			}
			for _, line := range result.Lines {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bundleFetchURL, "url", "", "direct HTTPS URL to an exported .tar.gz bundle")
	cmd.Flags().StringVar(&bundleFetchDest, "dest", "", "destination directory for unpacked bundle contents")
	cmd.Flags().StringVar(&bundleFetchSHA256, "sha256", "", "expected SHA256 for URL mode; overrides .sha256 sidecar lookup")
	cmd.Flags().StringVar(&bundleFetchAssetName, "asset-name", "", "specific GitHub release bundle asset name to install")
	cmd.Flags().StringVar(&bundleFetchPlatform, "platform", "", "bundle platform hint for GitHub mode (codex-runtime or claude)")
	cmd.Flags().StringVar(&bundleFetchRuntime, "runtime", "", "bundle runtime hint for GitHub mode (python or node)")
	cmd.Flags().StringVar(&bundleFetchTag, "tag", "", "GitHub release tag for bundle selection")
	cmd.Flags().BoolVar(&bundleFetchLatest, "latest", false, "install from the latest GitHub release instead of --tag")
	cmd.Flags().StringVar(&bundleFetchGitHubToken, "github-token", "", "GitHub token (optional; default from GITHUB_TOKEN env)")
	cmd.Flags().StringVar(&bundleFetchGitHubAPIBase, "github-api-base", "", "GitHub API base URL override (for tests or GitHub Enterprise)")
	cmd.Flags().BoolVarP(&bundleFetchForce, "force", "f", false, "overwrite an existing destination directory")
	_ = cmd.MarkFlagRequired("dest")
	return cmd
}

func buildBundleFetchOptions(args []string) app.PluginBundleFetchOptions {
	token := strings.TrimSpace(bundleFetchGitHubToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	ref := ""
	if len(args) == 1 {
		ref = args[0]
	}
	return app.PluginBundleFetchOptions{
		URL:           bundleFetchURL,
		Ref:           ref,
		Tag:           bundleFetchTag,
		Latest:        bundleFetchLatest,
		Dest:          bundleFetchDest,
		SHA256:        bundleFetchSHA256,
		AssetName:     bundleFetchAssetName,
		Platform:      bundleFetchPlatform,
		Runtime:       bundleFetchRuntime,
		GitHubToken:   token,
		GitHubAPIBase: bundleFetchGitHubAPIBase,
		Force:         bundleFetchForce,
	}
}

func validateBundleFetchArgs(cmd *cobra.Command, args []string) error {
	hasURL := strings.TrimSpace(bundleFetchURL) != ""
	switch {
	case hasURL && len(args) != 0:
		return fmt.Errorf("bundle fetch accepts no positional args with --url")
	case !hasURL && len(args) != 1:
		return fmt.Errorf("bundle fetch requires --url or owner/repo")
	default:
		return nil
	}
}
