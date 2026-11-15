package nativeconfig

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

// ErrBoundedReadUnavailable refuses an injected backend that has not explicitly
// supplied bounded, no-follow reads. It never authorizes an OS fallback.
var ErrBoundedReadUnavailable = errors.New("native config bounded file IO is required")

// This optional capability leaves the original FileIO contract unchanged.
// A custom backend may supply it explicitly; its result is still checked.
type boundedFileIO interface {
	ReadNoFollowBounded(string, int64) ([]byte, os.FileMode, bool, error)
}

func (kernel Kernel) RequireBoundedRead() error {
	if err := kernel.RequireFileIO(); err != nil {
		return err
	}
	if _, ok := kernel.files.(boundedFileIO); !ok {
		return ErrBoundedReadUnavailable
	}
	return nil
}

// ReadExactFileBounded preserves the injected reader's authority. Unsupported
// custom readers refuse without calling their unbounded method or probing disk.
func (kernel Kernel) ReadExactFileBounded(path string, limit int64) (FileSnapshot, error) {
	if err := kernel.RequireBoundedRead(); err != nil {
		return FileSnapshot{}, err
	}
	if err := validateExactPath(path, "native config path"); err != nil {
		return FileSnapshot{}, err
	}
	if limit < 0 || limit == math.MaxInt64 {
		return FileSnapshot{}, fmt.Errorf("invalid bounded native config read limit")
	}
	files, ok := kernel.files.(boundedFileIO)
	if !ok {
		return FileSnapshot{}, ErrBoundedReadUnavailable
	}
	body, mode, exists, err := files.ReadNoFollowBounded(path, limit)
	if err != nil {
		return FileSnapshot{}, err
	}
	if int64(len(body)) > limit || !exists && len(body) != 0 {
		return FileSnapshot{}, fmt.Errorf("bounded native config reader returned an inconsistent snapshot")
	}
	return FileSnapshot{Body: bytes.Clone(body), Mode: mode, Exists: exists}, nil
}

func (osFiles) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	before, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, 0o600, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	body, err := atomicfile.ReadBoundedRegularFile(path, limit)
	if err != nil {
		return nil, 0, false, err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, 0, false, errors.Join(err, ErrConcurrentChange)
	}
	if !before.Mode().IsRegular() || !after.Mode().IsRegular() ||
		!os.SameFile(before, after) || before.Mode() != after.Mode() ||
		before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) ||
		int64(len(body)) != after.Size() {
		return nil, 0, false, ErrConcurrentChange
	}
	return body, after.Mode().Perm(), true, nil
}
