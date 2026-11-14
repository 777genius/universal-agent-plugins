package usecase

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func (in *Installer) prepareDestination(ctx context.Context, req installRequest, binName string) (installDestination, error) {
	outName := strings.TrimSpace(req.input.OutputName)
	if outName == "" {
		outName = binName
	}
	destPath := filepath.Join(req.absDir, outName)
	destInfo, err := in.FS.PathInfo(ctx, destPath)
	if err != nil {
		return installDestination{}, err
	}
	if destInfo.Exists && destInfo.IsDir {
		return installDestination{}, domain.NewError(domain.ExitFS, "destination is an existing directory: "+destPath)
	}
	if destInfo.Exists && !req.input.Force {
		return installDestination{}, domain.NewError(domain.ExitFS, destPath+" already exists (use --force to overwrite)")
	}
	if destInfo.Exists && req.input.Force {
		_ = in.FS.RemoveBestEffort(ctx, destPath)
	}
	return installDestination{
		outName:   outName,
		destPath:  destPath,
		overwrote: destInfo.Exists,
	}, nil
}

func (in *Installer) writeInstalledBinary(ctx context.Context, req installRequest, dest installDestination, binData []byte) error {
	r := bytes.NewReader(binData)
	return in.FS.WriteFileAtomic(ctx, req.absDir, dest.outName, r, int64(len(binData)), in.Perms.FileMode(req.input.Target))
}
