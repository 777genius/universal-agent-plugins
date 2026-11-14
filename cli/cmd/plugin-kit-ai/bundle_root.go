package main

import (
	"context"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

type bundleRunner interface {
	BundleInstall(app.PluginBundleInstallOptions) (app.PluginBundleInstallResult, error)
	BundleFetch(context.Context, app.PluginBundleFetchOptions) (app.PluginBundleFetchResult, error)
	BundlePublish(context.Context, app.PluginBundlePublishOptions) (app.PluginBundlePublishResult, error)
}

var (
	bundleInstallDest  string
	bundleInstallForce bool

	bundleFetchURL           string
	bundleFetchDest          string
	bundleFetchSHA256        string
	bundleFetchAssetName     string
	bundleFetchPlatform      string
	bundleFetchRuntime       string
	bundleFetchTag           string
	bundleFetchGitHubToken   string
	bundleFetchGitHubAPIBase string
	bundleFetchForce         bool
	bundleFetchLatest        bool

	bundlePublishPlatform      string
	bundlePublishRepo          string
	bundlePublishTag           string
	bundlePublishDraft         bool
	bundlePublishGitHubToken   string
	bundlePublishGitHubAPIBase string
	bundlePublishForce         bool
)

func newBundleCmd(runner bundleRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Bundle tooling for exported interpreted-runtime handoff archives",
	}
	cmd.AddCommand(newBundleInstallCmd(runner))
	cmd.AddCommand(newBundleFetchCmd(runner))
	cmd.AddCommand(newBundlePublishCmd(runner))
	return cmd
}
