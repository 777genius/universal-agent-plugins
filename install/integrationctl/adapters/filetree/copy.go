package filetree

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func CopyPathIfExists(src, dest string) (bool, error) {
	info, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("symlink source is not allowed: %s", src)
	}
	if info.IsDir() {
		return true, CopyDir(src, dest)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("special file source is not allowed: %s", src)
	}
	return true, CopyFile(src, dest)
}

func CopyDir(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("source must be a real directory: %s", src)
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink source is not allowed: %s", path)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("special file source is not allowed: %s", path)
		}
		return CopyFile(path, target)
	})
}

func CopyFile(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("source must be a regular file: %s", src)
	}
	if existing, err := os.Lstat(dest); err == nil && existing.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("destination symlink is not allowed: %s", dest)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()
	mode := os.FileMode(0o644)
	if info.Mode().Perm()&0o111 != 0 {
		mode = 0o755
	}
	target, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	copied, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if copied != info.Size() {
		return fmt.Errorf("source changed while copying: %s", src)
	}
	return os.Chmod(dest, mode)
}
