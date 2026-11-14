package usecase

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

type installRequest struct {
	input         Input
	tag           string
	absDir        string
	releaseSource string
}

type installPayload struct {
	assetName   string
	payloadKind string
	binName     string
	binData     []byte
}

type installDestination struct {
	outName   string
	destPath  string
	overwrote bool
}

func (in *Installer) runInstall(ctx context.Context, input Input) (Result, error) {
	req, err := in.prepareInstallRequest(ctx, input)
	if err != nil {
		return Result{}, err
	}
	rel, err := in.resolveRelease(ctx, req)
	if err != nil {
		return Result{}, err
	}
	payload, err := in.resolvePayload(ctx, req, rel)
	if err != nil {
		return Result{}, err
	}
	dest, err := in.prepareDestination(ctx, req, payload.binName)
	if err != nil {
		return Result{}, err
	}
	if err := in.writeInstalledBinary(ctx, req, dest, payload.binData); err != nil {
		return Result{}, err
	}
	return Result{
		ResolvedInstallPath: dest.destPath,
		InstalledFileName:   dest.outName,
		ReleaseRef:          strings.TrimSpace(rel.TagName),
		ReleaseSource:       req.releaseSource,
		AssetName:           payload.assetName,
		TargetGOOS:          req.input.Target.GOOS,
		TargetGOARCH:        req.input.Target.GOARCH,
		Overwrote:           dest.overwrote,
		PayloadKind:         payload.payloadKind,
	}, nil
}

func (in *Installer) resolveRelease(ctx context.Context, req installRequest) (*domain.Release, error) {
	var (
		rel *domain.Release
		err error
	)
	if req.input.UseLatest {
		rel, err = in.GitHub.GetLatestRelease(ctx, req.input.Owner, req.input.Repo)
	} else {
		rel, err = in.GitHub.GetReleaseByTag(ctx, req.input.Owner, req.input.Repo, req.tag)
	}
	if err != nil {
		return nil, err
	}
	if rel.Prerelease && !req.input.AllowPrerelease {
		return nil, domain.NewError(domain.ExitRelease, "release is prerelease (refused; use plugin-kit-ai install --pre)")
	}
	return rel, nil
}
