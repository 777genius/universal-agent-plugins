package providers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The launcher addresses runtimes by lock digest, so the installer must keep
// that target name. A different marker or damaged installation can be replaced
// only after a complete new runtime has been prepared and the old owned runtime
// has been moved aside without deleting it.
func commitLockedRuntimeTarget(store, target, temporary string, expected lockedRuntimeMarker) error {
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return os.Rename(temporary, target)
	} else if err != nil {
		return err
	}
	if err := requireRealDirectory(target); err != nil {
		return fmt.Errorf("locked npm runtime target is not owned: %w", err)
	}
	body, err := readRuntimeFile(target, runtimeMarkerName)
	if err != nil {
		return fmt.Errorf("locked npm runtime target has no valid ownership marker: %w", err)
	}
	var marker lockedRuntimeMarker
	if err := decodeStrictJSON(body, &marker); err != nil {
		return fmt.Errorf("locked npm runtime target has no valid ownership marker: %w", err)
	}
	if marker.SchemaVersion != expected.SchemaVersion || marker.LockDigest != expected.LockDigest ||
		marker.Package != expected.Package || marker.Version != expected.Version {
		return fmt.Errorf("locked npm runtime target has a different owner; refusing to replace unrelated data")
	}
	retiredDir, err := os.MkdirTemp(store, ".retired-"+filepath.Base(target)+"-")
	if err != nil {
		return err
	}
	retiredTarget := filepath.Join(retiredDir, "runtime")
	if err := os.Rename(target, retiredTarget); err != nil {
		_ = os.Remove(retiredDir)
		return err
	}
	if err := os.Rename(temporary, target); err != nil {
		if _, absentErr := os.Lstat(target); errors.Is(absentErr, os.ErrNotExist) {
			if restoreErr := os.Rename(retiredTarget, target); restoreErr == nil {
				_ = os.Remove(retiredDir)
			} else {
				return errors.Join(err, fmt.Errorf("old runtime preserved at %s; restore failed: %w", retiredTarget, restoreErr))
			}
		}
		return err
	}
	return nil
}
