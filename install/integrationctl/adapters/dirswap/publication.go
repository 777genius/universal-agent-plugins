package dirswap

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// publicationProof covers every entry, including empty directories, exact modes,
// symlink targets and internal metadata. It is a rollback proof, not a package digest.
func publicationProof(root string) (string, string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("publication root is not a real directory")
	}
	identity, err := directoryIdentity(root, info)
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	field := func(value string) {
		_ = binary.Write(h, binary.BigEndian, uint64(len(value)))
		_, _ = io.WriteString(h, value)
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		stat, err := entry.Info()
		if err != nil {
			return err
		}
		field(filepath.ToSlash(rel))
		field(fmt.Sprint(uint32(stat.Mode())))
		switch {
		case stat.IsDir():
		case stat.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			field(target)
		case stat.Mode().IsRegular():
			_ = binary.Write(h, binary.BigEndian, uint64(stat.Size()))
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			opened, err := f.Stat()
			if err != nil {
				f.Close()
				return err
			}
			if !os.SameFile(stat, opened) {
				f.Close()
				return fmt.Errorf("publication file replaced while hashing")
			}
			n, readErr := io.Copy(h, f)
			closeErr := f.Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			if n != stat.Size() {
				return fmt.Errorf("publication file changed while hashing")
			}
		default:
			return fmt.Errorf("unsupported publication entry %q", rel)
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	after, err := os.Lstat(root)
	if err != nil {
		return "", "", err
	}
	if !os.SameFile(info, after) {
		return "", "", fmt.Errorf("publication root replaced while hashing")
	}
	return identity, hex.EncodeToString(h.Sum(nil)), nil
}

func matchesPublication(receipt Receipt, path string) error {
	if receipt.PublishedIdentity == "" || receipt.PublishedDigest == "" {
		return fmt.Errorf("directory rollback ownership evidence unavailable; recovery required")
	}
	identity, digest, err := publicationProof(path)
	if err != nil {
		return fmt.Errorf("verify directory rollback ownership: %w", err)
	}
	if identity != receipt.PublishedIdentity || digest != receipt.PublishedDigest {
		return fmt.Errorf("directory changed or replaced since staging; recovery required")
	}
	return nil
}

func (manager Manager) rollbackAbsent(receipt Receipt, activeExists, backupExists bool) error {
	if !activeExists && !backupExists {
		return nil
	}
	if receipt.Operation != OperationSwap {
		return fmt.Errorf("unexpected directory for absent removal; recovery required")
	}
	if backupExists {
		// A crash may leave the provisional object quarantined before cleanup.
		if err := matchesPublication(receipt, receipt.BackupPath); err != nil {
			return err
		}
		if activeExists {
			return fmt.Errorf("active directory appeared during rollback; recovery required")
		}
	} else {
		if err := matchesPublication(receipt, receipt.ActivePath); err != nil {
			return err
		}
		// Quarantine atomically before the final ownership check. A path replacement
		// during the earlier hash is preserved, never passed to RemoveAll.
		if err := renameDirectoryExclusive(receipt.ActivePath, receipt.BackupPath); err != nil {
			return err
		}
		if err := atomicSyncRollback(receipt); err != nil {
			return err
		}
		if err := manager.inject(FaultRollbackQuarantined); err != nil {
			return err
		}
		if err := matchesPublication(receipt, receipt.BackupPath); err != nil {
			// Best effort exclusive restoration preserves a concurrently created active.
			_ = renameDirectoryExclusive(receipt.BackupPath, receipt.ActivePath)
			_ = atomicSyncRollback(receipt)
			return err
		}
	}
	if err := removeOwnedDirectory(receipt.OwnedBase, receipt.BackupPath); err != nil {
		return err
	}
	return manager.inject(FaultRollbackRemoved)
}

const FaultRollbackQuarantined = "rollback_quarantined"

func atomicSyncRollback(receipt Receipt) error { return syncReceiptParents(receipt, false) }
