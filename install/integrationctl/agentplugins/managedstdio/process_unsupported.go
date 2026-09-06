//go:build !darwin && !linux

package managedstdio

import "fmt"

// Windows remains gated until suspended creation + Job Object lifecycle has
// native evidence. Cross-compilation is not evidence of process-tree cleanup.
func Supported() bool { return false }
func replaceProcess(string, []string, string) error {
	return fmt.Errorf("managed stdio is unsupported on this platform")
}
