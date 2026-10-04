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

func matchesProof(path, expectedIdentity, expectedDigest string) error {
	if expectedIdentity == "" || expectedDigest == "" {
		return fmt.Errorf("directory ownership evidence unavailable for %q; recovery required", path)
	}
	identity, digest, err := publicationProof(path)
	if err != nil {
		return fmt.Errorf("verify directory ownership at %q: %w", path, err)
	}
	if identity != expectedIdentity || digest != expectedDigest {
		return fmt.Errorf("directory changed or replaced at %q; recovery required", path)
	}
	return nil
}

func matchesPublication(receipt Receipt, path string) error {
	if receipt.SchemaVersion == 5 {
		if err := matchesObject(path, receipt.PublishedObject); err != nil {
			return err
		}
	}
	return matchesProof(path, receipt.PublishedIdentity, receipt.PublishedDigest)
}

func matchesBackup(receipt Receipt, path string) error {
	if receipt.SchemaVersion == 5 {
		if err := matchesObject(path, receipt.OldObject); err != nil {
			return err
		}
	}
	return matchesProof(path, receipt.BackupIdentity, receipt.BackupDigest)
}

const FaultRollbackQuarantined = "rollback_quarantined"
const FaultCommitQuarantined = "commit_quarantined"

// Parent identities prevent later path substitution from redirecting a journal
// to a different tree, including through an ancestor above OwnedBase.
func physicalDirectoryIdentity(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("parent %q is not a real directory", path)
	}
	return directoryIdentity(path, info)
}

func matchesDirectoryIdentity(path, expected string) error {
	if expected == "" {
		return fmt.Errorf("parent identity unavailable for %q", path)
	}
	identity, err := physicalDirectoryIdentity(path)
	if err != nil {
		return err
	}
	if identity != expected {
		return fmt.Errorf("directory parent changed at %q; recovery required", path)
	}
	return nil
}
