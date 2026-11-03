package atomicfile

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// Admission refusals are distinct from filesystem permission and I/O errors.
var (
	ErrUnsafeFile   = errors.New("file is not a regular non-symlink file")
	ErrFileTooLarge = errors.New("file exceeds its byte limit")
	ErrFileChanged  = errors.New("file changed during read")
)

// ReadBoundedRegularFile reads one stable regular file without following a leaf
// symlink or blocking while opening a substituted FIFO. Parent directories may
// contain links; this is not a containment or concurrent directory-isolation API.
func ReadBoundedRegularFile(path string, limit int64) ([]byte, error) {
	return readBoundedRegularFile(path, limit, openRegularNoFollow)
}

// The explicit opener keeps race tests local to their invocation; production
// calls always use the platform's no-follow, nonblocking admission operation.
func readBoundedRegularFile(path string, limit int64, open func(string) (*os.File, error)) ([]byte, error) {
	if limit < 0 || limit == math.MaxInt64 {
		return nil, fmt.Errorf("invalid regular-file byte limit %d", limit)
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := admitRegularRead(before, limit); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	file, err := open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if err := admitRegularRead(opened, limit); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !sameReadObservation(before, opened) {
		return nil, fmt.Errorf("%w: %s changed while opening", ErrFileChanged, path)
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: %s exceeds %d bytes", ErrFileTooLarge, path, limit)
	}
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	current, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s disappeared during read", ErrFileChanged, path)
		}
		return nil, err
	}
	if !sameReadObservation(opened, after) || !sameReadObservation(after, current) || int64(len(body)) != after.Size() {
		return nil, fmt.Errorf("%w: %s changed during read", ErrFileChanged, path)
	}
	return body, nil
}

func admitRegularRead(info os.FileInfo, limit int64) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: expected a regular non-symlink file", ErrUnsafeFile)
	}
	if info.Size() > limit {
		return fmt.Errorf("%w: exceeds %d bytes", ErrFileTooLarge, limit)
	}
	return nil
}

func sameReadObservation(left, right os.FileInfo) bool {
	return left.Mode().IsRegular() && right.Mode().IsRegular() && os.SameFile(left, right) &&
		left.Mode() == right.Mode() && left.Size() == right.Size() && left.ModTime().Equal(right.ModTime())
}

// OpenRootReadFile permits symlinks contained by root, and refuses an opened
// nonregular file without reading it. Unix opens are nonblocking so a FIFO
// substituted during rooted resolution cannot stall before handle inspection.
// The caller owns Close and any byte limit or stable-read checks it requires.
func OpenRootReadFile(root *os.Root, name string) (*os.File, error) {
	if root == nil {
		return nil, errors.New("read root is required")
	}
	file, err := openRootReadFile(root, name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s is not a regular rooted file", ErrUnsafeFile, name)
	}
	return file, nil
}
