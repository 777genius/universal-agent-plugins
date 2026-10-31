//go:build !windows

package atomicfile

import "os"

// ReadFile reads an atomically published file.
func ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
