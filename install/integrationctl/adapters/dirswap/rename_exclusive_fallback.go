//go:build !windows && !darwin && !(linux && amd64) && !(linux && arm64)

package dirswap

import "fmt"

func renameDirectoryExclusive(oldPath, newPath string) error {
	return fmt.Errorf("exclusive directory publication is unsupported on this platform")
}
