package plugininstall

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/adapters/archive"
	"github.com/777genius/plugin-kit-ai/plugininstall/adapters/fs"
	gh "github.com/777genius/plugin-kit-ai/plugininstall/adapters/github"
	"github.com/777genius/plugin-kit-ai/plugininstall/usecase"
)

func newInstaller(p Params) *usecase.Installer {
	client := gh.NewClient(p.Token)
	if p.GitHubBaseURL != "" {
		client.BaseURL = strings.TrimSuffix(p.GitHubBaseURL, "/")
	}
	return &usecase.Installer{
		GitHub:    client,
		Archive:   archive.TarGzExtractor{},
		FS:        fs.OS{},
		Resolver:  hostPathResolver{},
		Selector:  hostAssetSelector{},
		Checksums: hostChecksumVerifier{},
		Perms:     hostPermissionPolicy{},
	}
}

func buildInstallInput(p Params) usecase.Input {
	return usecase.Input{
		Owner:           p.Owner,
		Repo:            p.Repo,
		Tag:             p.Tag,
		UseLatest:       p.UseLatest,
		InstallDir:      p.InstallDir,
		Force:           p.Force,
		AllowPrerelease: p.AllowPrerelease,
		OutputName:      p.OutputName,
		Target:          hostTarget(p.GOOS, p.GOARCH),
	}
}

func resultFromUsecase(got usecase.Result) Result {
	return Result{
		ResolvedInstallPath: got.ResolvedInstallPath,
		InstalledFileName:   got.InstalledFileName,
		ReleaseRef:          got.ReleaseRef,
		ReleaseSource:       got.ReleaseSource,
		AssetName:           got.AssetName,
		TargetGOOS:          got.TargetGOOS,
		TargetGOARCH:        got.TargetGOARCH,
		Overwrote:           got.Overwrote,
		PayloadKind:         got.PayloadKind,
	}
}
