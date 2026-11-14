//go:build !windows

package atomicfile

import "os"

// Replace publishes an already synced and closed temporary file. Callers own
// cleanup and provide source and target names in the same directory.
func Replace(source, target string) error { return os.Rename(source, target) }
