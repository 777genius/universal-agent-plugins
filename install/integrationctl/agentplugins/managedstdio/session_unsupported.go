//go:build !darwin && !linux && !windows

package managedstdio

import "fmt"

func replaceProcessEnvironment(string, []string, string, []string) error {
	return fmt.Errorf("native sessions are unsupported on this platform")
}
