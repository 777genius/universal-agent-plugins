package directoryidentity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Capture resolves admitted Unix aliases once and freezes the resulting spelling.
// It does not create a prospective profile or discover a home directory.
func Capture(ctx context.Context, canonicalRoot string) (Authority, error) {
	if err := ctx.Err(); err != nil {
		return Authority{}, err
	}
	if platformScheme() == "unsupported" {
		return Authority{}, ErrUnsupported
	}
	root, err := canonicalize(canonicalRoot)
	if err != nil {
		return Authority{}, bootstrapError(err)
	}
	return observe(ctx, root)
}

// Revalidate never follows a newly introduced alias or recaptures consent.
func Revalidate(ctx context.Context, expected Authority) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if platformScheme() == "unsupported" {
		return ErrUnsupported
	}
	if expected.IsZero() {
		return errors.New("zero directory authority")
	}
	current, err := observe(ctx, expected.Facts().CanonicalRoot)
	if err != nil {
		return err
	}
	if !current.Equal(expected) {
		return ErrChanged
	}
	return ctx.Err()
}

func bootstrapError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrBootstrapRequired, err)
	}
	return err
}

func ancestryPaths(root string) ([]string, error) {
	if err := validatePath(root, platformScheme()); err != nil {
		return nil, err
	}
	paths := make([]string, 0, 256)
	for p := root; ; p = filepath.Dir(p) {
		if len(paths) == 256 {
			return nil, errors.New("directory ancestry limit")
		}
		paths = append(paths, p)
		if filepath.Dir(p) == p {
			break
		}
	}
	for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
		paths[i], paths[j] = paths[j], paths[i]
	}
	return paths, nil
}

func observe(ctx context.Context, root string) (result Authority, err error) {
	paths, err := ancestryPaths(root)
	if err != nil {
		return Authority{}, err
	}
	held := make([]*os.File, 0, len(paths))
	defer func() {
		for _, f := range held {
			err = errors.Join(err, f.Close())
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			result = Authority{}
		}
	}()
	facts := Facts{Version: 1, CanonicalRoot: root, Ancestry: make([]Entry, 0, len(paths))}
	for i, p := range paths {
		if err := ctx.Err(); err != nil {
			return Authority{}, err
		}
		var parent *os.File
		name := p
		if i > 0 {
			parent, name = held[i-1], filepath.Base(p)
		}
		f, err := openDirectory(parent, name)
		if err != nil {
			return Authority{}, bootstrapError(err)
		}
		held = append(held, f)
		e, err := entryOf(f, p)
		if err != nil {
			return Authority{}, fmt.Errorf("observe %q: %w", p, err)
		}
		facts.Ancestry = append(facts.Ancestry, e)
	}
	if err := verifyHeld(ctx, held, facts.Ancestry); err != nil {
		return Authority{}, err
	}
	// A second complete namespace walk observes ancestor substitution even when
	// the retained old parent still has the original child. No atomic exclusion
	// of a rename after this walk is claimed.
	if err := verifyNamespace(ctx, facts.Ancestry); err != nil {
		return Authority{}, err
	}
	if err := ctx.Err(); err != nil {
		return Authority{}, err
	}
	return NewAuthority(facts)
}

func entryOf(f *os.File, p string) (Entry, error) {
	if err := verifyCanonicalName(f, p); err != nil {
		return Entry{}, err
	}
	volume, object, err := directoryFacts(f)
	return Entry{CanonicalPath: p, Scheme: platformScheme(), VolumeID: volume, ObjectID: object}, err
}

func verifyHeld(ctx context.Context, held []*os.File, entries []Entry) error {
	for i, expected := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		actual, err := entryOf(held[i], expected.CanonicalPath)
		if err != nil {
			return err
		}
		if actual != expected {
			return ErrChanged
		}
		var parent *os.File
		name := expected.CanonicalPath
		if i > 0 {
			parent, name = held[i-1], filepath.Base(name)
		}
		if err := checkNamed(parent, name, expected); err != nil {
			return err
		}
	}
	return nil
}

func checkNamed(parent *os.File, name string, expected Entry) error {
	f, err := openDirectory(parent, name)
	if err != nil {
		return err
	}
	actual, err := entryOf(f, expected.CanonicalPath)
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	if actual != expected {
		return ErrChanged
	}
	return nil
}

func verifyNamespace(ctx context.Context, entries []Entry) (err error) {
	var parent *os.File
	defer func() {
		if parent != nil {
			err = errors.Join(err, parent.Close())
		}
	}()
	for i, expected := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := expected.CanonicalPath
		if i > 0 {
			name = filepath.Base(name)
		}
		f, err := openDirectory(parent, name)
		if err != nil {
			return err
		}
		actual, observeErr := entryOf(f, expected.CanonicalPath)
		var closeErr error
		if parent != nil {
			closeErr = parent.Close()
		}
		parent = f
		if err := errors.Join(observeErr, closeErr); err != nil {
			return err
		}
		if actual != expected {
			return ErrChanged
		}
	}
	return nil
}
