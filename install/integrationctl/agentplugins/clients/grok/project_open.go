package grok

import (
	"fmt"
	"os"
	"path/filepath"
)

// The hook is supplied only by inert race regressions. Production opens are
// anchored, non-following and nonblocking before inspecting the actual handle.
func openProjectPath(path string, directory bool, beforeOpen func()) (*os.File, os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, nil, fmt.Errorf("grok discovery path is not absolute and clean")
	}
	before, err := os.Lstat(path)
	if err != nil {
		// Even an absent leaf must not hide an existing alias in its ancestors.
		probe, openErr := openProjectHandle(path, directory)
		if probe != nil {
			_ = probe.Close()
			return nil, nil, fmt.Errorf("grok discovery path appeared during open")
		}
		return nil, nil, openErr
	}
	if before.Mode()&os.ModeSymlink != 0 || before.IsDir() != directory || (!directory && !before.Mode().IsRegular()) {
		return nil, nil, fmt.Errorf("grok discovery requires a real typed path")
	}
	if beforeOpen != nil {
		beforeOpen()
	}
	file, err := openProjectHandle(path, directory)
	if err != nil {
		return nil, nil, err
	}
	actual, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if !os.SameFile(before, actual) || actual.IsDir() != directory || (!directory && !actual.Mode().IsRegular()) {
		_ = file.Close()
		return nil, nil, fmt.Errorf("grok discovery path changed before open")
	}
	return file, actual, nil
}
func stableProjectPath(path string, file *os.File, before os.FileInfo) error {
	actual, err := file.Stat()
	if err != nil {
		return err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !os.SameFile(before, actual) || !os.SameFile(actual, after) || after.Mode()&os.ModeSymlink != 0 || before.Size() != actual.Size() || actual.Size() != after.Size() || !before.ModTime().Equal(actual.ModTime()) || !actual.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("grok discovery path changed during bounded read")
	}
	return nil
}
