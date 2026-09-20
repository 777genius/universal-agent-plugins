package plugininstall

import (
	"crypto/sha256"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
	"github.com/777genius/plugin-kit-ai/plugininstall/internal/checksum"
	"github.com/777genius/plugin-kit-ai/plugininstall/ports"
)

type hostPathResolver struct{}

func (hostPathResolver) Resolve(path string) (string, error) {
	return filepath.Abs(path)
}

type hostAssetSelector struct{}

func (hostAssetSelector) Pick(assets []domain.Asset, target ports.TargetPlatform) (*domain.Asset, bool, error) {
	return domain.PickInstallAsset(assets, target.GOOS, target.GOARCH)
}

type hostChecksumVerifier struct{}

func (hostChecksumVerifier) Expected(checksumsFile []byte, assetName string) ([]byte, error) {
	return checksum.ExpectedSum(checksumsFile, assetName)
}

func (hostChecksumVerifier) Verify(payload []byte, expected []byte, assetName string) error {
	got := sha256.Sum256(payload)
	if len(expected) != len(got) {
		return domain.NewError(domain.ExitChecksum, "internal checksum length")
	}
	for i := range expected {
		if expected[i] != got[i] {
			return domain.NewError(domain.ExitChecksum, "sha256 mismatch for "+assetName)
		}
	}
	return nil
}

type hostPermissionPolicy struct{}

func (hostPermissionPolicy) FileMode(target ports.TargetPlatform) uint32 {
	if target.GOOS == "windows" {
		return 0o644
	}
	return 0o755
}

func hostTarget(goos, goarch string) ports.TargetPlatform {
	if strings.TrimSpace(goos) == "" {
		goos = runtime.GOOS
	}
	if strings.TrimSpace(goarch) == "" {
		goarch = runtime.GOARCH
	}
	return ports.TargetPlatform{
		GOOS:   strings.TrimSpace(goos),
		GOARCH: strings.TrimSpace(goarch),
	}
}
