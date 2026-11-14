package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/plugininstall/ports"
)

// Input is the install use case input.
type Input struct {
	Owner, Repo, Tag string
	UseLatest        bool
	InstallDir       string
	Force            bool
	AllowPrerelease  bool
	OutputName       string
	Target           ports.TargetPlatform
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

// Installer wires ports.
type Installer struct {
	GitHub    ports.ReleaseSource
	Archive   ports.ArchiveExtractor
	FS        ports.FileSystem
	Resolver  ports.InstallDirResolver
	Selector  ports.AssetSelector
	Checksums ports.ChecksumVerifier
	Perms     ports.PermissionPolicy
}

// Run performs install.
func (in *Installer) Run(ctx context.Context, input Input) (Result, error) {
	return in.runInstall(ctx, input)
}
