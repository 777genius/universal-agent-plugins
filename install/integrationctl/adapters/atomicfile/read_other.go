//go:build !windows

package atomicfile

import "os"

// OpenReadFile uses ordinary Unix read semantics; renames remain atomic even
// while readers hold a reference to the old inode.
func OpenReadFile(path string) (*os.File, error) { return os.Open(path) }

// ReadFile reads an atomically published file.
func ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
