package ports

import "github.com/777genius/plugin-kit-ai/plugininstall/domain"

type TargetPlatform struct {
	GOOS   string
	GOARCH string
}

type InstallDirResolver interface {
	Resolve(path string) (string, error)
}

type AssetSelector interface {
	Pick(assets []domain.Asset, target TargetPlatform) (payload *domain.Asset, fromTarGz bool, err error)
}

type ChecksumVerifier interface {
	Expected(checksumsFile []byte, assetName string) ([]byte, error)
	Verify(payload []byte, expected []byte, assetName string) error
}

type PermissionPolicy interface {
	FileMode(target TargetPlatform) uint32
}
