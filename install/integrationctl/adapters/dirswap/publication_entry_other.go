//go:build !darwin && !linux && !windows

package dirswap

import (
	"fmt"
	"os"
)

func publicationEntryFacts(*os.File, os.FileInfo) (string, string, error) {
	return "", "", fmt.Errorf("original publication object identity is unsupported")
}

func samePublicationMetadata(os.FileInfo, os.FileInfo) bool { return false }
