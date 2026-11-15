package transaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validateExpectedPublication(mutation DirectoryMutation) error {
	if mutation.ExpectedPublication.IsZero() {
		return nil
	}
	if !mutation.Standalone || !mutation.RequireAbsent || mutation.PublicationParentAuthority == nil {
		return fmt.Errorf("original publication proof requires a fresh standalone publication")
	}
	return nil
}

func (kernel Kernel) groupedPublicationOwners(ctx context.Context, group DirectoryGroup) ([]domain.PhysicalProfileOwner, error) {
	for _, mutation := range group.Mutations {
		if !mutation.ExpectedPublication.IsZero() {
			return nil, fmt.Errorf("original publication proof cannot join a package group")
		}
	}
	return kernel.mutationOwners(ctx, group.DesiredState, group.Mutations)
}

// Retain the final information from the original open marker. The pre-write
// stat has a different size and time and cannot authorize the completed bytes.
func (prepared *preparedNativeRoot) captureWrittenMarker(file *os.File) error {
	after, err := file.Stat()
	if err != nil {
		return err
	}
	before := prepared.markerInfo
	if before == nil || !before.Mode().IsRegular() || !after.Mode().IsRegular() ||
		!os.SameFile(before, after) || before.Mode() != after.Mode() || after.Size() != int64(len(prepared.writtenMarker)) {
		return fmt.Errorf("native marker changed while writing its original handle")
	}
	prepared.markerInfo = after
	return nil
}

func (prepared *preparedNativeRoot) closeMarkerHandle() error {
	if prepared.markerFile == nil {
		return nil
	}
	file := prepared.markerFile
	prepared.markerFile = nil
	return file.Close()
}

// Seal the exact original objects before closing a nested descendant. Windows
// cannot rename the staged parent while that descendant directory is open.
// The stage and its original parent remain held throughout publication.
func (prepared *preparedNativeRoot) sealForPublication() error {
	if prepared.markerFile == nil {
		return fmt.Errorf("original native marker handle is unavailable before sealing")
	}
	if !prepared.expectedPublication.IsZero() {
		return fmt.Errorf("native root publication was already sealed")
	}
	if err := prepared.verifyNamed(); err != nil {
		return err
	}
	entries, err := prepared.originalPublicationEntries()
	if err != nil {
		return err
	}
	proof, err := dirswap.CaptureOriginalPublication(prepared.staged.root, entries)
	if err != nil {
		return err
	}
	if err := prepared.verifyNamed(); err != nil {
		return err
	}
	prepared.expectedPublication = proof
	if err := prepared.closeMarkerHandle(); err != nil {
		return err
	}
	if prepared.selected != prepared.staged {
		selected := prepared.selected.root
		prepared.selected.root = nil
		return selected.Close()
	}
	return nil
}

func (prepared *preparedNativeRoot) originalPublicationEntries() ([]dirswap.OriginalPublicationEntry, error) {
	stageInfo, err := prepared.staged.root.Stat(".")
	if err != nil || !sameNativeRoot(prepared.staged.info, stageInfo) {
		return nil, fmt.Errorf("original native staging directory changed before sealing")
	}
	entries := []dirswap.OriginalPublicationEntry{{RelativePath: ".", Info: stageInfo}}
	markerPath := prepared.markerName
	if prepared.selected != prepared.staged {
		selectedInfo, err := prepared.selected.root.Stat(".")
		if err != nil || !sameNativeRoot(prepared.selected.info, selectedInfo) {
			return nil, fmt.Errorf("original native selected directory changed before sealing")
		}
		name := filepath.Base(prepared.selected.path)
		entries = append(entries, dirswap.OriginalPublicationEntry{RelativePath: name, Info: selectedInfo})
		markerPath = filepath.Join(name, markerPath)
	}
	return append(entries, dirswap.OriginalPublicationEntry{RelativePath: filepath.ToSlash(markerPath), Info: prepared.markerInfo}), nil
}

// A pre-journal refusal may release staging back to the preparer. Reopen only
// beneath the still-held original stage, retaining the original identity. A
// same-byte replacement is foreign and must survive cleanup.
func (prepared *preparedNativeRoot) restoreOriginalSelected() (resultErr error) {
	if prepared.selected.root != nil {
		return nil
	}
	if prepared.selected == prepared.staged || prepared.staged.root == nil {
		return fmt.Errorf("original native staging handle is unavailable; retained")
	}
	if err := errors.Join(prepared.baseRoot.verifyNamed(), prepared.staged.verifyNamed()); err != nil {
		return err
	}
	name := filepath.Base(prepared.selected.path)
	before, err := prepared.staged.root.Lstat(name)
	if err != nil || !sameNativeRoot(prepared.selected.info, before) {
		return fmt.Errorf("original native selected directory changed; retained")
	}
	root, err := prepared.staged.root.OpenRoot(name)
	if err != nil {
		return err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, root.Close())
		}
	}()
	opened, err := root.Stat(".")
	named, namedErr := prepared.staged.root.Lstat(name)
	if err != nil || namedErr != nil || !sameNativeRoot(prepared.selected.info, opened) || !sameNativeRoot(prepared.selected.info, named) {
		return fmt.Errorf("original native selected directory changed during reopen; retained")
	}
	if err := errors.Join(prepared.baseRoot.verifyNamed(), prepared.staged.verifyNamed()); err != nil {
		return err
	}
	prepared.selected.root = root
	return nil
}
