package nativeconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

// FileSnapshot binds a mutation to exact bytes and existence. It is transient
// recovery data, never an ownership receipt or persisted client state.
type FileSnapshot struct {
	Body   []byte
	Mode   os.FileMode
	Exists bool
}

type FileEffect string

const (
	FileUnchanged FileEffect = "unchanged"
	FileCommitted FileEffect = "committed"
	FileUncertain FileEffect = "uncertain"
)

// ExactFile holds the normal nativeconfig writer lock across a caller's native
// transaction. It deliberately leaves parsing and ownership digests to the
// caller, for clients whose native shape differs from the generic MCP codec.
// Call Close exactly once, after commit or rollback. The portable content-CAS
// limitation documented on conditionalFileIO also applies here.
type ExactFile struct {
	kernel    Kernel
	path      string
	original  FileSnapshot
	output    []byte
	attempted bool
	conflicted bool
	effect    FileEffect
	release   func() error
}

func (kernel Kernel) ReadExactFile(path string) (FileSnapshot, error) {
	if err := kernel.RequireFileIO(); err != nil {
		return FileSnapshot{}, err
	}
	if err := validateExactPath(path, "native config path"); err != nil {
		return FileSnapshot{}, err
	}
	body, mode, exists, err := kernel.files.ReadNoFollow(path)
	return FileSnapshot{Body: bytes.Clone(body), Mode: mode, Exists: exists}, err
}

func (kernel Kernel) BeginExactFile(path string) (*ExactFile, error) {
	if err := kernel.RequireFileIO(); err != nil {
		return nil, err
	}
	if err := validateExactPath(path, "native config path"); err != nil {
		return nil, err
	}
	acquire := kernel.acquireLocks
	if acquire == nil {
		acquire = kernel.acquireCandidateLocks
	}
	release, err := acquire(Paths{JSON: path}, CodecMCPServers)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, fmt.Errorf("native config lock acquirer returned no release operation")
	}
	snapshot, err := kernel.ReadExactFile(path)
	if err != nil {
		return nil, errors.Join(err, release())
	}
	return &ExactFile{kernel: kernel, path: path, original: snapshot, effect: FileUnchanged, release: release}, nil
}

func (file *ExactFile) Original() FileSnapshot {
	snapshot := file.original
	snapshot.Body = bytes.Clone(snapshot.Body)
	return snapshot
}
func (file *ExactFile) Effect() FileEffect { return file.effect }

// Apply compares the original bytes/existence at the write boundary, and reads
// back even when WriteAtomic reports an error after making its write visible.
// On error the caller must invoke Rollback before releasing the lock.
func (file *ExactFile) Apply(body []byte) error {
	if file.release == nil || file.attempted {
		return fmt.Errorf("native config transaction is closed or already applied")
	}
	file.attempted, file.output = true, bytes.Clone(body)
	mode := file.original.Mode
	if !file.original.Exists {
		mode = 0600
	}
	writeErr := file.kernel.compareAndSwap(file.path, file.original.Body, file.original.Exists, file.output, mode)
	// A rejected CAS did not authorize our write. Even if another writer chose
	// the same bytes as output, readback cannot turn that into our ownership.
	file.conflicted = errors.Is(writeErr, ErrConcurrentChange)
	if writeErr != nil {
		writeErr = fmt.Errorf("write native config: %w", writeErr)
	}
	current, readErr := file.observe()
	if readErr != nil {
		return errors.Join(writeErr, fmt.Errorf("read back native config %q: %w", file.path, readErr))
	}
	if !current.Exists || !bytes.Equal(current.Body, file.output) {
		return errors.Join(writeErr, fmt.Errorf("verify native config %q: %w", file.path, ErrConcurrentChange))
	}
	return writeErr
}

func (file *ExactFile) observe() (FileSnapshot, error) {
	current, err := file.kernel.ReadExactFile(file.path)
	file.effect = FileUncertain
	if err == nil {
		switch {
		case current.Exists == file.original.Exists && bytes.Equal(current.Body, file.original.Body):
			file.effect = FileUnchanged
		case file.attempted && !file.conflicted && current.Exists && bytes.Equal(current.Body, file.output):
			file.effect = FileCommitted
		}
	}
	return current, err
}

// Rollback restores only our exact output. A late foreign version is retained
// and reported as uncertain, with its path. Restore errors also require readback.
func (file *ExactFile) Rollback() error {
	if file.release == nil {
		return fmt.Errorf("native config transaction is closed")
	}
	if !file.attempted {
		return nil
	}
	if _, err := file.observe(); err != nil {
		return fmt.Errorf("retain native config at %q; rollback readback: %w", file.path, err)
	}
	if file.effect == FileUnchanged {
		return nil
	}
	if file.effect != FileCommitted {
		return fmt.Errorf("retain concurrent native config at %q: %w", file.path, ErrConcurrentChange)
	}
	var restoreErr error
	if file.original.Exists {
		restoreErr = file.kernel.compareAndSwap(file.path, file.output, true, file.original.Body, file.original.Mode)
	} else {
		restoreErr = file.kernel.removeIfUnchanged(file.path, file.output)
	}
	_, readErr := file.observe()
	err := errors.Join(restoreErr, readErr)
	if file.effect != FileUnchanged {
		err = errors.Join(err, ErrConcurrentChange)
	}
	if err != nil {
		return fmt.Errorf("rollback native config at %q: %w", file.path, err)
	}
	return nil
}

// Close reports lock cleanup separately from the observed file effect. The
// caller decides whether its complete native transaction committed.
func (file *ExactFile) Close() error {
	if file.release == nil {
		return nil
	}
	release := file.release
	file.release = nil
	return release()
}
