package dirswap

import (
	"fmt"
	"os"
)

// VerifyPending checks the original physical backup and the publication before
// the caller makes its state commit decision. Package/data digests are separate
// caller proofs; neither substitutes for these physical ownership proofs.
func (manager Manager) VerifyPending(receipt Receipt) (resultErr error) {
	defer func() { resultErr = manager.recoveryError(receipt, resultErr) }()
	if err := manager.validateReceipt(receipt); err != nil {
		return err
	}
	if receipt.HadActive {
		if err := matchesBackup(receipt, receipt.BackupPath); err != nil {
			return err
		}
	}
	if receipt.Operation == OperationSwap {
		return matchesPublication(receipt, receipt.ActivePath)
	}
	return requireMissing(receipt.ActivePath)
}

func requireMissing(path string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("unexpected directory entry at %q; recovery required", path)
}

func (manager Manager) recoveryError(receipt Receipt, err error) error {
	if err == nil || receipt.OperationID == "" {
		return err
	}
	return fmt.Errorf("%w; recovery paths: active=%q backup=%q quarantine=%q staging=%q journal=%q", err,
		receipt.ActivePath, receipt.BackupPath, receipt.QuarantinePath, receipt.StagingPath, manager.journalPath(receipt.OperationID))
}

func (manager Manager) finishCommit(r Receipt) error {
	if err := manager.validateReceipt(r); err != nil {
		return err
	}
	if r.Operation == OperationSwap {
		if err := matchesPublication(r, r.ActivePath); err != nil {
			return err
		}
	} else if err := requireMissing(r.ActivePath); err != nil {
		return err
	}
	backup, err := realDirectoryExists(r.BackupPath)
	if err != nil {
		return err
	}
	quarantine, err := realDirectoryExists(r.QuarantinePath)
	if err != nil {
		return err
	}
	if !r.HadActive {
		if backup || quarantine {
			return fmt.Errorf("unexpected backup for absent publication")
		}
		return nil
	}
	if backup {
		if err := matchesBackup(r, r.BackupPath); err != nil {
			return err
		}
		if err := renameDirectoryExclusive(r.BackupPath, r.QuarantinePath); err != nil {
			return err
		}
		if err := syncReceiptParents(r, false); err != nil {
			return err
		}
		if err := manager.inject(FaultCommitQuarantined); err != nil {
			return err
		}
		quarantine = true
	}
	if quarantine {
		return manager.removeCommittedBackup(r)
	}
	// CommitPending may resume after the proven backup was already deleted.
	return nil
}

func (manager Manager) removeCommittedBackup(r Receipt) error {
	if err := manager.validateReceipt(r); err != nil {
		return err
	}
	if err := matchesBackup(r, r.QuarantinePath); err != nil {
		return err
	}
	if r.Operation == OperationSwap {
		if err := matchesPublication(r, r.ActivePath); err != nil {
			return err
		}
	} else if err := requireMissing(r.ActivePath); err != nil {
		return err
	}
	if err := removeOwnedDirectory(r.OwnedBase, r.QuarantinePath); err != nil {
		return err
	}
	return syncReceiptParents(r, false)
}

func (manager Manager) restoreOld(r Receipt) error {
	if err := manager.validateReceipt(r); err != nil {
		return err
	}
	active, err := realDirectoryExists(r.ActivePath)
	if err != nil {
		return err
	}
	backup, err := realDirectoryExists(r.BackupPath)
	if err != nil {
		return err
	}
	quarantine, err := realDirectoryExists(r.QuarantinePath)
	if err != nil {
		return err
	}
	if r.HadActive {
		if !backup {
			// Only the original identity AND contents prove no rename or a completed restore.
			if err := matchesBackup(r, r.ActivePath); err != nil {
				return err
			}
			if quarantine {
				return fmt.Errorf("ambiguous restored active and quarantine")
			}
			return nil
		}
		// A changed original is evidence to retain, even if the new publication is intact.
		if err := matchesBackup(r, r.BackupPath); err != nil {
			return err
		}
	} else if backup {
		return fmt.Errorf("unexpected original backup")
	}
	if err := manager.rollbackPublication(r, active, quarantine); err != nil {
		return err
	}
	if r.HadActive {
		if err := manager.validateReceipt(r); err != nil {
			return err
		}
		if err := matchesBackup(r, r.BackupPath); err != nil {
			return err
		}
		if err := renameDirectoryExclusive(r.BackupPath, r.ActivePath); err != nil {
			return fmt.Errorf("restore original backup: %w", err)
		}
	}
	return requireRollbackResult(r)
}

func (manager Manager) rollbackPublication(r Receipt, active, quarantine bool) error {
	movedPublication := false
	if active {
		if r.Operation != OperationSwap {
			return fmt.Errorf("active entry appeared during removal rollback")
		}
		if err := matchesPublication(r, r.ActivePath); err != nil {
			return err
		}
		if err := renameDirectoryExclusive(r.ActivePath, r.QuarantinePath); err != nil {
			return err
		}
		if err := syncReceiptParents(r, false); err != nil {
			return err
		}
		if err := manager.inject(FaultRollbackQuarantined); err != nil {
			return err
		}
		quarantine, movedPublication = true, true
	}
	if !quarantine {
		return nil
	}
	if err := manager.validateReceipt(r); err != nil {
		return err
	}
	if err := matchesPublication(r, r.QuarantinePath); err != nil {
		if !movedPublication {
			return err
		}
		return manager.restoreRacedQuarantine(r, err)
	}
	if err := requireMissing(r.ActivePath); err != nil {
		return err
	}
	if r.HadActive {
		if err := matchesBackup(r, r.BackupPath); err != nil {
			return err
		}
	}
	if err := removeOwnedDirectory(r.OwnedBase, r.QuarantinePath); err != nil {
		return err
	}
	if err := syncReceiptParents(r, false); err != nil {
		return err
	}
	return manager.inject(FaultRollbackRemoved)
}

func (manager Manager) restoreRacedQuarantine(r Receipt, cause error) error {
	if r.SchemaVersion == 5 {
		if err := manager.validatePhysical(r); err != nil {
			return err
		}
	}
	// Never discard the raced entry, and never replace a late active entry.
	if err := renameDirectoryExclusive(r.QuarantinePath, r.ActivePath); err != nil {
		return fmt.Errorf("%w; restore quarantine failed: %w", cause, err)
	}
	if err := syncReceiptParents(r, false); err != nil {
		return fmt.Errorf("%w; sync quarantine restore: %w", cause, err)
	}
	return cause
}

func requireRollbackResult(r Receipt) error {
	if r.HadActive {
		return matchesBackup(r, r.ActivePath)
	}
	return requireMissing(r.ActivePath)
}
