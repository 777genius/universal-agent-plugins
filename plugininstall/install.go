package plugininstall

import (
	"context"
)

// Params configures Install (plugin binary from GitHub Releases).
type Params struct {
	Owner, Repo, Tag string
	UseLatest        bool   // if true, Tag is ignored; use GitHub releases/latest
	InstallDir       string // default "bin" (relative to cwd resolved to abs)
	Force            bool
	Token            string // optional; also read from GITHUB_TOKEN in CLI
	AllowPrerelease  bool   // default false
	OutputName       string // empty = name from archive
	GitHubBaseURL    string // empty = default
	GOOS             string // optional explicit target override
	GOARCH           string // optional explicit target override
}

type Result struct {
	ResolvedInstallPath string
	InstalledFileName   string
	ReleaseRef          string
	ReleaseSource       string
	AssetName           string
	TargetGOOS          string
	TargetGOARCH        string
	Overwrote           bool
	PayloadKind         string
}

// Install downloads and verifies a release tarball and writes the binary to InstallDir.
// This file is the module composition root: the only place that wires concrete adapters into usecase.Installer.
func Install(ctx context.Context, p Params) (Result, error) {
	got, err := newInstaller(p).Run(ctx, buildInstallInput(p))
	if err != nil {
		return Result{}, err
	}
	return resultFromUsecase(got), nil
}
