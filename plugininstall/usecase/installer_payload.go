package usecase

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func (in *Installer) resolvePayload(ctx context.Context, req installRequest, rel *domain.Release) (installPayload, error) {
	checksumsAsset := findAsset(rel.Assets, "checksums.txt")
	if checksumsAsset == nil {
		return installPayload{}, domain.NewError(domain.ExitChecksum, "release has no checksums.txt (required for verified install)")
	}
	payload, fromTarGz, err := in.Selector.Pick(rel.Assets, req.input.Target)
	if err != nil {
		return installPayload{}, err
	}
	sumBody, _, err := in.GitHub.DownloadAsset(ctx, checksumsAsset.BrowserDownloadURL)
	if err != nil {
		return installPayload{}, err
	}
	expected, err := in.Checksums.Expected(sumBody, payload.Name)
	if err != nil {
		return installPayload{}, domain.NewError(domain.ExitChecksum, "checksums.txt: "+err.Error())
	}
	payloadBody, _, err := in.GitHub.DownloadAsset(ctx, payload.BrowserDownloadURL)
	if err != nil {
		return installPayload{}, err
	}
	if err := in.Checksums.Verify(payloadBody, expected, payload.Name); err != nil {
		return installPayload{}, err
	}
	binName, binData, payloadKind, err := in.materializePayloadBinary(ctx, payload.Name, payloadBody, fromTarGz)
	if err != nil {
		return installPayload{}, err
	}
	return installPayload{
		assetName:   payload.Name,
		payloadKind: payloadKind,
		binName:     binName,
		binData:     binData,
	}, nil
}

func (in *Installer) materializePayloadBinary(ctx context.Context, payloadName string, payloadBody []byte, fromTarGz bool) (string, []byte, string, error) {
	if fromTarGz {
		binName, binData, err := in.Archive.ExtractRootExecutable(ctx, bytes.NewReader(payloadBody))
		if err != nil {
			return "", nil, "", err
		}
		return binName, binData, "tar.gz", nil
	}
	return filepath.Base(payloadName), payloadBody, "raw", nil
}

func findAsset(assets []domain.Asset, wantName string) *domain.Asset {
	w := strings.ToLower(wantName)
	for i := range assets {
		if strings.ToLower(assets[i].Name) == w {
			return &assets[i]
		}
	}
	return nil
}
