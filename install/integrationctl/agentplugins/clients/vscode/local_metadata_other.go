//go:build !linux && !(darwin && arm64)

package vscode

import "fmt"

func localWritableMetadata(_ string) error {
	return fmt.Errorf("local profile metadata mutation unqualified on this OS; native Windows ACL/profile CI required")
}
