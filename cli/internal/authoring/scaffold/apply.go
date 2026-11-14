package scaffold

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Validate is supplied by the shared conformance/project service. It MUST perform
// actual offline validation, honor ctx, treat stagingRoot as read-only, and close
// all leases before returning. A no-op success callback violates this contract.
// This service deliberately contains no second package parser. A function's
// semantics cannot be checked at runtime; composition tests must prove them.
//
// dir is a live handle to the exact stagingRoot directory, still held open by
// Apply. It exists only so the callback can prove -- by identity, not by a
// second path lookup -- that it is validating this operation's own freshly
// created, exclusively owned payload, never caller-selected content. The
// callback MUST NOT write through dir; it is passed only for that identity
// proof (see packageview.GeneratedStaging).
type Validate func(ctx context.Context, stagingRoot string, dir *os.Root) error

type ApplyOptions struct {
	Destination string   // clean absolute missing destination; parent must exist
	SourceRoots []string // optional protected existing roots; no overlap in either direction
	Validate    Validate // required; nil is rejected before any filesystem work
}

// Result distinguishes a committed destination from a post-commit cleanup error.
// Cancellation observed before the atomic commit leaves no destination. Once the
// kernel commits, cancellation does not turn that commit into a rollback.
type Result struct {
	Destination string
	Committed   bool
}

// CleanupError marks incomplete owned-stage recovery while retaining its cause.
// Public reports must classify it without serializing private filesystem paths.
type CleanupError struct{ Err error }

func (e *CleanupError) Error() string { return e.Err.Error() }
func (e *CleanupError) Unwrap() error { return e.Err }

func Apply(ctx context.Context, p Plan, o ApplyOptions) (Result, error) {
	return apply(ctx, p, o, applyOps{write: writeTree, publish: renamePackage})
}

// Private operation seam for deterministic I/O failure and commit-race tests.
// Public callers cannot replace writes or the absence-preserving primitive.
type applyOps struct {
	write  func(context.Context, *os.Root, []File) error
	rename func(*os.File, string, *os.File, string) error
	// Package publication alone supports a checked retry; ApplySkill uses rename.
	publish func(context.Context, *os.File, string, *os.File, string, func() error) error
}

func apply(ctx context.Context, p Plan, o ApplyOptions, ops applyOps) (result Result, err error) {
	if ctx == nil {
		return result, fmt.Errorf("context is required")
	}
	if o.Validate == nil {
		return result, fmt.Errorf("shared offline validation callback is required")
	}
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
	default:
		return result, fmt.Errorf("absence-preserving scaffold apply is unsupported on %s", runtime.GOOS)
	}
	if err = ctx.Err(); err != nil {
		return
	}
	files := p.Files()
	if err = validateFiles(files); err != nil {
		return
	}
	parentPath, e := validateDestination(o.Destination, o.SourceRoots)
	if e != nil {
		return result, e
	}
	parent, e := openParent(parentPath)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, parent.Close()) }()
	parentInfo, e := parent.Stat(".")
	if e != nil {
		return result, e
	}
	destination := filepath.Base(o.Destination)
	if e = absent(parent, destination); e != nil {
		return result, e
	}
	// A random, exclusively created container keeps payload private even after its
	// intended final 0755 mode is set. All writes/cleanup use pinned directory roots.
	stage := ".authoring-" + rand.Text()
	if e = makePrivateStage(parent, stage); e != nil {
		return result, e
	}
	stageInfo, e := parent.Lstat(stage)
	if e != nil {
		return result, fmt.Errorf("inspect newly created stage %q; ownership unavailable, cleanup refused: %w", filepath.Join(parentPath, stage), e)
	}
	owned, e := parent.OpenRoot(stage)
	if e != nil {
		current, statErr := parent.Lstat(stage)
		if statErr == nil && os.SameFile(stageInfo, current) {
			return result, errors.Join(e, parent.Remove(stage))
		}
		return result, errors.Join(e, fmt.Errorf("stage ownership unavailable; cleanup refused for %q", filepath.Join(parentPath, stage)))
	}
	current, e := owned.Stat(".")
	if e != nil || !os.SameFile(stageInfo, current) {
		return result, errors.Join(fmt.Errorf("staging ownership changed: %w", errOr(e, fs.ErrInvalid)), owned.Close())
	}
	defer func() {
		// Payload cleanup has already run through its pinned root. Never recursively
		// remove the container by its mutable sibling pathname.
		cleanup := owned.Close()
		current, statErr := parent.Lstat(stage)
		if statErr == nil && os.SameFile(stageInfo, current) {
			cleanup = errors.Join(cleanup, parent.Remove(stage))
		} else {
			cleanup = errors.Join(cleanup, fmt.Errorf("staging ownership changed; refused sibling cleanup: %w", errOr(statErr, fs.ErrInvalid)))
		}
		if cleanup != nil {
			err = errors.Join(err, &CleanupError{fmt.Errorf("cleanup staging %q: %w", filepath.Join(parentPath, stage), cleanup)})
		}
	}()
	if e = owned.Mkdir("payload", 0700); e != nil {
		return result, e
	}
	root, e := owned.OpenRoot("payload")
	if e != nil {
		return result, e
	}
	// Keep the payload root pinned through validation and commit. Windows rooted
	// opens share deletion, so the exclusive rename can proceed with this held.
	payloadInfo, e := root.Stat(".")
	if e != nil {
		return result, errors.Join(e, root.Close())
	}
	defer func() {
		var cleanup error
		if !result.Committed {
			cleanup = clearOwnedPayload(root)
		}
		cleanup = errors.Join(cleanup, root.Close())
		if !result.Committed {
			now, statErr := owned.Lstat("payload")
			if statErr == nil && os.SameFile(payloadInfo, now) {
				cleanup = errors.Join(cleanup, owned.Remove("payload"))
			} else {
				cleanup = errors.Join(cleanup, fmt.Errorf("payload ownership changed; refused pathname cleanup: %w", errOr(statErr, fs.ErrInvalid)))
			}
		}
		if cleanup != nil {
			err = errors.Join(err, &CleanupError{fmt.Errorf("cleanup owned payload: %w", cleanup)})
		}
	}()
	if e = ops.write(ctx, root, files); e != nil {
		return result, e
	}
	if e = checkParent(parentPath, parentInfo); e != nil {
		return result, e
	}
	stagingPath := filepath.Join(parentPath, stage, "payload")
	if e = ctx.Err(); e != nil {
		return result, e
	}
	if e = o.Validate(ctx, stagingPath, root); e != nil {
		return result, fmt.Errorf("validate generated package: %w", e)
	}
	if e = ctx.Err(); e != nil {
		return result, e
	}
	check := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if e := checkParent(parentPath, parentInfo); e != nil {
			return e
		}
		current, e := parent.Lstat(stage)
		if e != nil || !os.SameFile(stageInfo, current) {
			return fmt.Errorf("staging ownership changed: %w", errOr(e, fs.ErrInvalid))
		}
		payloadNow, e := owned.Lstat("payload")
		if e != nil || !payloadNow.IsDir() || payloadNow.Mode()&os.ModeSymlink != 0 || !os.SameFile(payloadInfo, payloadNow) {
			return fmt.Errorf("payload ownership changed: %w", errOr(e, fs.ErrInvalid))
		}
		if e := verifyTree(ctx, root, files); e != nil {
			return e
		}
		// Diagnostic only; the kernel still enforces no replacement.
		if e := absent(parent, destination); e != nil {
			return e
		}
		return ctx.Err()
	}
	from, e := owned.Open(".")
	if e != nil {
		return result, e
	}
	to, e := parent.Open(".")
	if e != nil {
		return result, errors.Join(e, from.Close())
	}
	if ops.publish != nil {
		e = ops.publish(ctx, from, "payload", to, destination, check)
	} else if e = check(); e == nil {
		e = ops.rename(from, "payload", to, destination)
	}
	if e == nil {
		result = Result{Destination: o.Destination, Committed: true}
	}
	return result, errors.Join(e, from.Close(), to.Close())
}
func absent(r *os.Root, name string) error {
	_, err := r.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("destination already exists: %w", fs.ErrExist)
	}
	return err
}
func checkParent(path string, expected fs.FileInfo) error {
	r, err := openParent(path)
	if err != nil {
		return err
	}
	info, e := r.Stat(".")
	err = errors.Join(e, r.Close())
	if err != nil {
		return err
	}
	if !os.SameFile(expected, info) {
		return fmt.Errorf("destination parent changed")
	}
	return nil
}
func chmodDir(r *os.Root, path string, mode fs.FileMode) error {
	f, err := r.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(f.Chmod(mode), f.Close())
}

type outputFile interface {
	Write([]byte) (int, error)
	Chmod(fs.FileMode) error
	Close() error
}
type openOutput func(*os.Root, string) (outputFile, error)

func writeTree(ctx context.Context, r *os.Root, files []File) error {
	return writeTreeWith(ctx, r, files, func(root *os.Root, name string) (outputFile, error) {
		return root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	})
}
func writeTreeWith(ctx context.Context, r *os.Root, files []File, open openOutput) error {
	for _, d := range planDirectories(files) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.Mkdir(d, 0755); err != nil {
			return err
		}
		if err := chmodDir(r, d, 0755); err != nil {
			return err
		}
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		f, err := open(r, file.Path)
		if err != nil {
			return err
		}
		// Bounded chunks make cancellation observable during larger fixture writes.
		for off := 0; off < len(file.Bytes) && err == nil; {
			if err = ctx.Err(); err != nil {
				break
			}
			end := min(off+32*1024, len(file.Bytes))
			var n int
			n, err = f.Write(file.Bytes[off:end])
			if err == nil && n != end-off {
				err = io.ErrShortWrite
			}
			off += n
		}
		if err == nil {
			err = f.Chmod(file.Mode)
		}
		err = errors.Join(err, f.Close())
		if err != nil {
			return err
		}
	}
	return chmodDir(r, ".", 0755)
}

// verifyTree checks exact planned bytes/types/modes after the trusted read-only
// validator. It is a bounded tree equality check, not package interpretation.
func verifyTree(ctx context.Context, r *os.Root, files []File) error {
	expected := map[string]File{}
	dirs := map[string]bool{".": true}
	for _, f := range files {
		expected[f.Path] = f
	}
	for _, d := range planDirectories(files) {
		dirs[d] = true
	}
	count := 0
	err := fs.WalkDir(r.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		count++
		if count > maxFiles*9+1 {
			return fmt.Errorf("staging tree exceeds bound")
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.IsDir() && dirs[path] {
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0755 {
				return fmt.Errorf("staging directory mode changed: %s", path)
			}
			delete(dirs, path)
			return nil
		}
		f, ok := expected[path]
		if !ok || !info.Mode().IsRegular() || info.Size() != int64(len(f.Bytes)) {
			return fmt.Errorf("staging tree differs at %s", path)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != f.Mode {
			return fmt.Errorf("staging file mode changed: %s", path)
		}
		opened, err := r.Open(path)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(io.LimitReader(opened, int64(len(f.Bytes))+1))
		err = errors.Join(err, opened.Close())
		if err != nil {
			return err
		}
		if !bytes.Equal(body, f.Bytes) {
			return fmt.Errorf("staging bytes changed: %s", path)
		}
		delete(expected, path)
		return nil
	})
	if err != nil {
		return err
	}
	if len(expected) != 0 || len(dirs) != 0 {
		return fmt.Errorf("staging tree is incomplete")
	}
	return nil
}

// Cleanup uses the original opened payload even if a callback replaces its
// pathname. It never traverses a replacement payload or an outside symlink.
func clearOwnedPayload(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	names, readErr := dir.Readdirnames(maxFiles*9 + 2)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	err = errors.Join(readErr, dir.Close())
	if err != nil {
		return err
	}
	if len(names) > maxFiles*9+1 {
		return fmt.Errorf("owned cleanup exceeds entry bound")
	}
	for _, name := range names {
		err = errors.Join(err, root.RemoveAll(name))
	}
	return err
}
