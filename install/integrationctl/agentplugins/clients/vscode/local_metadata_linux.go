package vscode

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// The existing atomic writer preserves mode, not ACL/xattrs. Refuse attributed
// profiles rather than stripping metadata. Do not inspect unrelated files.
func localWritableMetadata(path string) error {
	count, err := unix.Llistxattr(path, nil)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect selected Local profile metadata: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("local profile ACL/xattr preservation unsupported; retain profile")
	}
	return nil
}
