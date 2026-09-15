//go:build !windows

package jsonmaint

import "os"

func syncParent(parent *os.File) error { return parent.Sync() }
