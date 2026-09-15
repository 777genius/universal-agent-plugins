//go:build !linux && !darwin

package jsonmaint

import "os"

// os.Root provides the platform's rooted reparse-point protections. Lstat and
// same-file checks around this open reject selected-document link substitution.
func openNoFollow(root *os.Root, name string) (*os.File, error) { return root.Open(name) }
