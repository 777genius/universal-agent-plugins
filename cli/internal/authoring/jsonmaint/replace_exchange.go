//go:build linux || darwin

package jsonmaint

import "os"

func replaceOwned(parent *os.File, _ string, replacement, target string) (string, error) {
	return replacement, exchange(parent, replacement, target)
}

func restoreOwned(parent *os.File, _ string, old, target string) (string, error) {
	return old, exchange(parent, old, target)
}
