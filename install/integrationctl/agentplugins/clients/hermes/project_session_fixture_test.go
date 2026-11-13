package hermes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// createProjectSessionRoot allocates only an absent child of an existing real
// .hermes directory. Interrupted preparation is retained, never adopted/retried.
func createProjectSessionRoot(ctx context.Context, project string) (receipt domain.NativeObjectOwnership, resultErr error) {
	if err := ctx.Err(); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	if _, err := noAliasDirectory(project); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	parent := filepath.Join(project, ".hermes")
	before, err := noAliasDirectory(parent)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	directory, err := os.OpenRoot(parent)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	heldParent, err := directory.Stat(".")
	if err != nil || !os.SameFile(before, heldParent) {
		return domain.NativeObjectOwnership{}, fmt.Errorf("hermes session parent changed before creation")
	}
	if err := directory.Mkdir("uap-session", 0o700); err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("hermes session requires a fresh owned root: %w", err)
	}
	childInfo, err := directory.Lstat("uap-session")
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	if err := writeSessionRootMarker(directory, childInfo, project); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	after, err := noAliasDirectory(parent)
	if err != nil || !os.SameFile(before, after) {
		return domain.NativeObjectOwnership{}, fmt.Errorf("hermes session parent changed during preparation")
	}
	if err := ctx.Err(); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	return sessionRootObject(project), nil
}

func sessionChildIdentity(parent *os.Root, original os.FileInfo) error {
	named, err := parent.Lstat("uap-session")
	if err != nil || !named.IsDir() || named.Mode()&os.ModeSymlink != 0 || !os.SameFile(original, named) || original.Mode() != named.Mode() {
		return fmt.Errorf("hermes fresh session child changed")
	}
	return nil
}

func writeSessionRootMarker(parent *os.Root, original os.FileInfo, project string) (resultErr error) {
	if err := sessionChildIdentity(parent, original); err != nil {
		return err
	}
	child, err := parent.OpenRoot("uap-session")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, child.Close()) }()
	held, err := child.Stat(".")
	if err != nil || !os.SameFile(original, held) || original.Mode() != held.Mode() {
		return fmt.Errorf("hermes fresh child changed before open")
	}
	marker, err := child.OpenFile(".uap-owner", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := sessionChildIdentity(parent, original); err != nil {
		return errors.Join(err, marker.Close())
	}
	written, writeErr := marker.Write(sessionRootBody(project))
	closeErr := marker.Close()
	if writeErr != nil || closeErr != nil || written != len(sessionRootBody(project)) {
		return fmt.Errorf("hermes session marker creation failed")
	}
	return sessionChildIdentity(parent, original)
}
