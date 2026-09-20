package usecase

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func (in *Installer) prepareInstallRequest(ctx context.Context, input Input) (installRequest, error) {
	if err := in.validateDeps(); err != nil {
		return installRequest{}, err
	}
	tag, err := validateInstallInput(input)
	if err != nil {
		return installRequest{}, err
	}
	if err := validateOutputName(input.OutputName); err != nil {
		return installRequest{}, err
	}
	absDir, err := in.resolveInstallDir(ctx, input.InstallDir)
	if err != nil {
		return installRequest{}, err
	}
	releaseSource := "tag"
	if input.UseLatest {
		releaseSource = "latest"
	}
	return installRequest{
		input:         input,
		tag:           tag,
		absDir:        absDir,
		releaseSource: releaseSource,
	}, nil
}

func (in *Installer) validateDeps() error {
	switch {
	case in.GitHub == nil:
		return domain.NewError(domain.ExitUsage, "installer github source is required")
	case in.Archive == nil:
		return domain.NewError(domain.ExitUsage, "installer archive extractor is required")
	case in.FS == nil:
		return domain.NewError(domain.ExitUsage, "installer file system is required")
	case in.Resolver == nil:
		return domain.NewError(domain.ExitUsage, "installer path resolver is required")
	case in.Selector == nil:
		return domain.NewError(domain.ExitUsage, "installer asset selector is required")
	case in.Checksums == nil:
		return domain.NewError(domain.ExitUsage, "installer checksum verifier is required")
	case in.Perms == nil:
		return domain.NewError(domain.ExitUsage, "installer permission policy is required")
	default:
		return nil
	}
}

func validateInstallInput(input Input) (string, error) {
	if input.Owner == "" || input.Repo == "" {
		return "", domain.NewError(domain.ExitUsage, "owner and repo are required")
	}
	tag := strings.TrimSpace(input.Tag)
	switch {
	case tag != "" && input.UseLatest:
		return "", domain.NewError(domain.ExitUsage, "use either --tag or --latest, not both")
	case tag == "" && !input.UseLatest:
		return "", domain.NewError(domain.ExitUsage, "set --tag or use --latest")
	}
	if strings.TrimSpace(input.Target.GOOS) == "" || strings.TrimSpace(input.Target.GOARCH) == "" {
		return "", domain.NewError(domain.ExitUsage, "target GOOS/GOARCH are required")
	}
	return tag, nil
}

func (in *Installer) resolveInstallDir(ctx context.Context, dir string) (string, error) {
	if dir == "" {
		dir = "bin"
	}
	absDir, err := in.Resolver.Resolve(dir)
	if err != nil {
		return "", domain.NewError(domain.ExitFS, "install dir: "+err.Error())
	}
	dirInfo, err := in.FS.PathInfo(ctx, absDir)
	if err != nil {
		return "", err
	}
	if dirInfo.Exists && !dirInfo.IsDir {
		return "", domain.NewError(domain.ExitFS, "install dir is an existing file: "+absDir)
	}
	return absDir, nil
}

func validateOutputName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if s == "." || s == ".." {
		return domain.NewError(domain.ExitUsage, "invalid --output-name")
	}
	if strings.Contains(s, "/") || strings.ContainsRune(s, '\\') {
		return domain.NewError(domain.ExitUsage, "--output-name must be a single file name (no path separators)")
	}
	if filepath.Base(s) != s {
		return domain.NewError(domain.ExitUsage, "--output-name must be a base file name only")
	}
	return nil
}
