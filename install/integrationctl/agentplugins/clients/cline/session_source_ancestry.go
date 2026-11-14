package cline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

type sessionSourceName struct {
	path, link string
	info       os.FileInfo
}

type sessionSourceDirectory struct {
	path, identity string
	root           *os.Root
	file           *os.File
	info           os.FileInfo
	exact          bool
}

type sessionSourceAncestry struct {
	logical, physical string
	anchor            *os.Root
	names             []sessionSourceName
	directories       []sessionSourceDirectory
}

func sessionSourceParents(path string) []string {
	var paths []string
	for {
		paths = append(paths, path)
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	slices.Reverse(paths)
	return paths
}

func captureSessionSourceAncestry(ctx context.Context, parent string) (result *sessionSourceAncestry, resultErr error) {
	result = &sessionSourceAncestry{logical: parent}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	for _, path := range sessionSourceParents(parent) {
		name, err := captureSessionSourceName(ctx, path)
		if err != nil {
			return result, err
		}
		result.names = append(result.names, name)
	}
	physical, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return result, err
	}
	result.physical = physical
	volume := filepath.VolumeName(physical) + string(filepath.Separator)
	result.anchor, err = os.OpenRoot(volume + ".")
	if err != nil {
		return result, err
	}
	for _, path := range sessionSourceParents(physical) {
		relative, err := filepath.Rel(volume, path)
		if err != nil {
			return result, err
		}
		dir, err := captureSessionSourceDirectory(ctx, result.anchor, path, relative, path == physical, openSessionSourceDirectory)
		if err != nil {
			return result, err
		}
		result.directories = append(result.directories, dir)
	}
	if err := result.Revalidate(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func captureSessionSourceName(ctx context.Context, path string) (sessionSourceName, error) {
	if err := ctx.Err(); err != nil {
		return sessionSourceName{}, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return sessionSourceName{}, errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	name := sessionSourceName{path: path, info: info}
	if info.Mode()&os.ModeSymlink != 0 {
		name.link, err = os.Readlink(path)
		if err != nil {
			return sessionSourceName{}, err
		}
	}
	if err := name.revalidate(); err != nil {
		return sessionSourceName{}, err
	}
	return name, nil
}

// Pinned Go1.26 preserves a terminal "." during Root path resolution. The
// selected component is therefore opened as a directory, not as a possibly
// substituted FIFO. Do not replace this explicit suffix with filepath.Join.
func openSessionSourceDirectory(anchor *os.Root, relative string) (*os.Root, error) {
	return anchor.OpenRoot(relative + string(filepath.Separator) + ".")
}

func captureSessionSourceDirectory(ctx context.Context, anchor *os.Root, path, relative string, exact bool, open func(*os.Root, string) (*os.Root, error)) (result sessionSourceDirectory, resultErr error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return result, errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	result.path, result.exact = path, exact
	result.root, err = open(anchor, relative)
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.close())
		}
	}()
	result.file, err = openSessionPinnedPath(path, true)
	if err != nil {
		return result, err
	}
	result.info, err = result.file.Stat()
	if err != nil || !sameSessionSourceDirectory(before, result.info, exact) {
		return result, errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	result.identity, err = directoryidentity.LegacyHandleIdentity(result.file)
	if err != nil {
		return result, err
	}
	if err := result.revalidate(); err != nil {
		return result, err
	}
	return result, ctx.Err()
}

func sameSessionSourceDirectory(original, observed os.FileInfo, exact bool) bool {
	if original == nil || observed == nil || !observed.IsDir() ||
		observed.Mode()&os.ModeSymlink != 0 || original.Mode() != observed.Mode() || !os.SameFile(original, observed) {
		return false
	}
	// Preserve the original immediate-parent extent checks. Ancestors' own
	// membership may change independently; their identity and mode may not.
	return !exact || sameSessionPhysicalIdentity(original, observed, true)
}

func (name sessionSourceName) revalidate() error {
	current, err := os.Lstat(name.path)
	if err != nil || name.info.Mode() != current.Mode() || !os.SameFile(name.info, current) {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	if name.info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(name.path)
		if err != nil || link != name.link || name.info.Size() != current.Size() || !name.info.ModTime().Equal(current.ModTime()) {
			return errors.Join(err, nativeconfig.ErrConcurrentChange)
		}
	}
	return nil
}

func (dir sessionSourceDirectory) revalidate() error {
	held, err := dir.file.Stat()
	if err != nil || !sameSessionSourceDirectory(dir.info, held, dir.exact) {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	rooted, err := dir.root.Stat(".")
	if err != nil || !sameSessionSourceDirectory(dir.info, rooted, dir.exact) {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	named, err := os.Lstat(dir.path)
	if err != nil || !sameSessionSourceDirectory(dir.info, named, dir.exact) {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	identity, err := directoryidentity.LegacyHandleIdentity(dir.file)
	if err != nil || identity != dir.identity {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	return nil
}

func (ancestry *sessionSourceAncestry) Revalidate(ctx context.Context) error {
	for _, name := range ancestry.names {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := name.revalidate(); err != nil {
			return err
		}
	}
	for _, dir := range ancestry.directories {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := dir.revalidate(); err != nil {
			return err
		}
	}
	physical, err := filepath.EvalSymlinks(ancestry.logical)
	if err != nil || physical != ancestry.physical {
		return errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	return ctx.Err()
}

func (ancestry *sessionSourceAncestry) parentRoot() *os.Root {
	return ancestry.directories[len(ancestry.directories)-1].root
}

func (dir sessionSourceDirectory) close() error {
	var result error
	if dir.file != nil {
		result = dir.file.Close()
	}
	if dir.root != nil {
		result = errors.Join(result, dir.root.Close())
	}
	return result
}

func (ancestry *sessionSourceAncestry) Close() error {
	var result error
	for index := len(ancestry.directories) - 1; index >= 0; index-- {
		result = errors.Join(result, ancestry.directories[index].close())
	}
	if ancestry.anchor != nil {
		result = errors.Join(result, ancestry.anchor.Close())
	}
	return result
}
