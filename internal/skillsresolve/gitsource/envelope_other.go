//go:build !linux

package gitsource

import "fmt"

func checkRuntimeEnvelope(string) error {
	return fmt.Errorf("hosted resolver requires the bounded Linux runtime")
}
