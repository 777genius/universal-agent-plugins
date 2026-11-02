//go:build !darwin && !linux && !windows

package dirswap

import (
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
)

func directoryIdentity(path string, info os.FileInfo) (string, error) {
	return directoryidentity.LegacyIdentity(path, info)
}
