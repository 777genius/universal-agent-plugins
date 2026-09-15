//go:build !linux && !darwin

package nativeimport

import "os"

func openNoFollow(path string) (*os.File, error) { return os.Open(path) }
