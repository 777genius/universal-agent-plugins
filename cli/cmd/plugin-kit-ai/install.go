package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/plugininstall"
	"github.com/spf13/cobra"
)

var installRunner = app.NewInstallRunner(nil)

var (
	installTag        string
	installDir        string
	installForce      bool
	installPrerelease bool
	installOutputName string
	installLatest     bool
	installToken      string
	installAPIBase    string // hidden, for tests / GitHub Enterprise
	installGOOS       string
	installGOARCH     string
)

var installCmd = &cobra.Command{
	Use:   "install [owner/repo]",
	Short: "Install a plugin binary from GitHub Releases (verified via checksums.txt)",
	Long: `Downloads checksums.txt and a release asset for your GOOS/GOARCH, verifies SHA256, and writes the binary to --dir
(default bin). Asset selection: (1) a single *_<goos>_<goarch>.tar.gz (GoReleaser) — file extracted from archive root;
or (2) a raw binary named *-<goos>-<goarch> or *-<goos>-<goarch>.exe on Windows (e.g. claude-notifications-darwin-arm64).

Use exactly one of --tag or --latest. Draft releases are refused; prerelease requires --pre.
Optional --output-name sets the installed filename (single path segment).

This command installs third-party plugin binaries, not the plugin-kit-ai CLI itself (build plugin-kit-ai from source or use a release installer).`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

func init() {
	installCmd.Flags().StringVar(&installTag, "tag", "", "Git release tag (required unless --latest), e.g. v0.1.0")
	installCmd.Flags().BoolVar(&installLatest, "latest", false, "install from GitHub releases/latest (non-prerelease) instead of --tag")
	installCmd.Flags().StringVar(&installDir, "dir", "bin", "directory for the installed binary (created if missing)")
	installCmd.Flags().BoolVarP(&installForce, "force", "f", false, "overwrite existing binary")
	installCmd.Flags().BoolVar(&installPrerelease, "pre", false, "allow GitHub prerelease (non-stable) releases")
	installCmd.Flags().StringVar(&installOutputName, "output-name", "", "write binary under this filename in --dir (default: name from archive)")
	installCmd.Flags().StringVar(&installToken, "github-token", "", "GitHub token (optional; default from GITHUB_TOKEN env)")
	installCmd.Flags().StringVar(&installGOOS, "goos", "", "target GOOS override (default: host GOOS)")
	installCmd.Flags().StringVar(&installGOARCH, "goarch", "", "target GOARCH override (default: host GOARCH)")
	installCmd.Flags().StringVar(&installAPIBase, "github-api-base", "", "")
	_ = installCmd.Flags().MarkHidden("github-api-base")
}

func runInstall(cmd *cobra.Command, args []string) error {
	ref := strings.TrimSpace(args[0])
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("argument must be owner/repo")
	}
	token := strings.TrimSpace(installToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	tag := strings.TrimSpace(installTag)
	if installLatest && tag != "" {
		return fmt.Errorf("do not use --tag together with --latest")
	}
	if !installLatest && tag == "" {
		return fmt.Errorf("set --tag or use --latest")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := installRunner.Install(ctx, plugininstall.Params{
		Owner:           parts[0],
		Repo:            parts[1],
		Tag:             tag,
		UseLatest:       installLatest,
		InstallDir:      installDir,
		Force:           installForce,
		AllowPrerelease: installPrerelease,
		OutputName:      strings.TrimSpace(installOutputName),
		Token:           token,
		GitHubBaseURL:   strings.TrimSpace(installAPIBase),
		GOOS:            strings.TrimSpace(installGOOS),
		GOARCH:          strings.TrimSpace(installGOARCH),
	})
	if err != nil {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
		return exitx.Wrap(err, plugininstall.ExitCodeFromErr(err))
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Installed %s\n", result.ResolvedInstallPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Release: %s (%s)\n", result.ReleaseRef, result.ReleaseSource)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Asset: %s\n", result.AssetName)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Target: %s/%s\n", result.TargetGOOS, result.TargetGOARCH)
	if result.Overwrote {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Overwrote existing file: yes")
	}
	return nil
}
