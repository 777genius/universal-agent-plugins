package transaction

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type heldNativeRoot struct {
	ancestry       *directoryidentity.Authority
	path, identity string
	root           *os.Root
	info           os.FileInfo
}

func holdNativeRoot(path string) (*heldNativeRoot, error) {
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || canonical != path || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("native root directory is aliased or unavailable")
	}
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("native root ordinary directory unavailable")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return finishHeldNativeRoot(path, root, before)
}

func finishHeldNativeRoot(path string, root *os.Root, before os.FileInfo) (held *heldNativeRoot, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, root.Close())
		}
	}()
	held = &heldNativeRoot{path: path, root: root, info: before}
	if err := held.verifyNamed(); err != nil {
		return nil, err
	}
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	identity, identityErr := directoryidentity.LegacyHandleIdentity(file)
	if err := errors.Join(identityErr, file.Close()); err != nil {
		return nil, err
	}
	held.identity = identity
	resultErr = held.verifyNamed()
	return held, resultErr
}

func (held *heldNativeRoot) verifyNamed() error {
	if held.ancestry != nil {
		if err := directoryidentity.Revalidate(context.Background(), *held.ancestry); err != nil {
			return err
		}
	}
	canonical, err := filepath.EvalSymlinks(held.path)
	if err != nil || canonical != held.path {
		return fmt.Errorf("native root named directory became aliased")
	}
	named, err := os.Lstat(held.path)
	if err != nil || !sameNativeRoot(held.info, named) {
		return fmt.Errorf("native root named directory changed")
	}
	return held.verifyHeld()
}

func (held *heldNativeRoot) verifyHeld() error {
	current, err := held.root.Stat(".")
	if err != nil || !sameNativeRoot(held.info, current) {
		return fmt.Errorf("native root held directory changed")
	}
	return nil
}

func sameNativeRoot(before, after os.FileInfo) bool {
	return before.IsDir() && after.IsDir() && after.Mode()&os.ModeSymlink == 0 &&
		os.SameFile(before, after) && before.Mode() == after.Mode()
}

func (held *heldNativeRoot) child(name string) (*heldNativeRoot, error) {
	if err := held.verifyNamed(); err != nil {
		return nil, err
	}
	before, err := held.root.Lstat(name)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("native root child is unavailable or aliased")
	}
	child, err := held.root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	return finishHeldNativeRoot(filepath.Join(held.path, name), child, before)
}

func (held *heldNativeRoot) readMarker(name string) (body []byte, resultErr error) {
	if err := held.verifyNamed(); err != nil {
		return nil, err
	}
	body, err := held.readHeldMarker(name)
	return body, errors.Join(err, held.verifyNamed())
}

func (held *heldNativeRoot) readHeldMarker(name string) (body []byte, resultErr error) {
	const limit int64 = 4096
	if err := held.verifyHeld(); err != nil {
		return nil, err
	}
	before, err := held.root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Size() > limit {
		return nil, fmt.Errorf("native marker is missing, aliased or oversized")
	}
	file, err := atomicfile.OpenRootReadFile(held.root, name)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !sameNativeMarker(before, opened) {
		return nil, fmt.Errorf("native marker changed before bounded read")
	}
	body, err = io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, fmt.Errorf("native marker read exceeded bound")
	}
	after, err := file.Stat()
	named, namedErr := held.root.Lstat(name)
	if err != nil || namedErr != nil || !sameNativeMarker(opened, after) || !sameNativeMarker(opened, named) {
		return nil, fmt.Errorf("native marker changed during read")
	}
	return body, held.verifyHeld()
}

func sameNativeMarker(before, after os.FileInfo) bool {
	return before.Mode().IsRegular() && after.Mode().IsRegular() && os.SameFile(before, after) &&
		before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func digestNativeRoot(child, parent *heldNativeRoot, logical, markerDigest string) string {
	return fmt.Sprintf("native-root-v1:%x", sha256.Sum256([]byte(strings.Join([]string{logical, parent.identity, child.identity, markerDigest}, "\x00"))))
}

type preparedNativeRoot struct {
	base, active, staging, digest string
	baseRoot, staged, selected    *heldNativeRoot
	markerName                    string
	markerBody                    []byte
	markerInfo                    os.FileInfo
	writtenMarker                 []byte
}

func prepareNativeRoot(ctx context.Context, spec ports.NativeProjectRoot) (prepared *preparedNativeRoot, resultErr error) {
	parent := filepath.Dir(spec.Path)
	base, active := parent, spec.Path
	if _, err := os.Lstat(parent); os.IsNotExist(err) {
		base, active = filepath.Dir(parent), parent
	} else if err != nil {
		return nil, err
	}
	baseRoot, err := holdNativeRoot(base)
	if err != nil {
		return nil, err
	}
	prepared = &preparedNativeRoot{base: base, active: active, baseRoot: baseRoot,
		markerName: filepath.Base(spec.Marker.Path), markerBody: spec.MarkerBody}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, prepared.cleanup(), prepared.close())
		}
	}()
	if err := baseRoot.captureParent(ctx); err != nil {
		return prepared, err
	}
	if err := prepared.create(spec.Path); err != nil {
		return prepared, err
	}
	parentRoot := prepared.baseRoot
	if active == parent {
		parentRoot = prepared.staged
	}
	prepared.digest = digestNativeRoot(prepared.selected, parentRoot, spec.Path, spec.Marker.ManagedDigest)
	return prepared, nil
}

func (held *heldNativeRoot) captureParent(ctx context.Context) error {
	original, err := held.root.Open(".")
	if err != nil {
		return err
	}
	authority, captureErr := directoryidentity.CaptureHeld(ctx, held.path, original)
	if err := errors.Join(captureErr, original.Close()); err != nil {
		return err
	}
	held.ancestry = &authority
	return errors.Join(ctx.Err(), held.verifyNamed())
}

func (prepared *preparedNativeRoot) create(selectedPath string) error {
	if err := prepared.baseRoot.verifyNamed(); err != nil {
		return err
	}
	name := ".uap-native-root-" + rand.Text()
	if err := prepared.baseRoot.root.Mkdir(name, 0o700); err != nil {
		return err
	}
	prepared.staging = filepath.Join(prepared.base, name)
	staged, err := prepared.baseRoot.child(name)
	if err != nil {
		return err
	}
	prepared.staged, prepared.selected = staged, staged
	if prepared.active == filepath.Dir(selectedPath) {
		if err := staged.root.Mkdir(filepath.Base(selectedPath), 0o700); err != nil {
			return err
		}
		selected, err := staged.child(filepath.Base(selectedPath))
		if err != nil {
			return err
		}
		prepared.selected = selected
	}
	return prepared.writeMarker()
}

func (prepared *preparedNativeRoot) writeMarker() error {
	if err := prepared.verifyNamed(); err != nil {
		return err
	}
	file, err := prepared.selected.root.OpenFile(prepared.markerName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	prepared.markerInfo, err = file.Stat()
	if err != nil {
		return errors.Join(err, file.Close())
	}
	written, writeErr := file.Write(prepared.markerBody)
	prepared.writtenMarker = append([]byte(nil), prepared.markerBody[:written]...)
	if writeErr == nil && written != len(prepared.markerBody) {
		writeErr = io.ErrShortWrite
	}
	if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
		return err
	}
	return prepared.verifyNamed()
}

func (prepared *preparedNativeRoot) close() error {
	var result error
	if prepared.selected != nil && prepared.selected != prepared.staged {
		result = prepared.selected.root.Close()
	}
	if prepared.staged != nil {
		result = errors.Join(result, prepared.staged.root.Close())
	}
	return errors.Join(result, prepared.baseRoot.root.Close())
}

func (prepared *preparedNativeRoot) cleanup() error {
	if prepared.staging == "" {
		return nil
	}
	if prepared.staged == nil || prepared.selected == nil {
		return fmt.Errorf("unpublished staging identity unavailable; retained without adopting its contents")
	}
	if err := prepared.cleanupMarker(); err != nil {
		return err
	}
	if prepared.selected != prepared.staged {
		if err := removeEmptyNativeChild(prepared.baseRoot, prepared.staged, prepared.selected); err != nil {
			return err
		}
	}
	return removeEmptyNativeChild(prepared.baseRoot, prepared.baseRoot, prepared.staged)
}

func (prepared *preparedNativeRoot) verifyNamed() error {
	return errors.Join(prepared.baseRoot.verifyNamed(), prepared.staged.verifyNamed(), prepared.selected.verifyNamed())
}

func (prepared *preparedNativeRoot) cleanupMarker() error {
	if err := prepared.verifyNamed(); err != nil {
		return err
	}
	root := prepared.selected
	info, err := root.root.Lstat(prepared.markerName)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || prepared.markerInfo == nil || !info.Mode().IsRegular() || !os.SameFile(prepared.markerInfo, info) {
		return fmt.Errorf("unpublished marker ownership changed; retained")
	}
	body, err := root.readMarker(prepared.markerName)
	if err != nil || string(body) != string(prepared.writtenMarker) {
		return errors.Join(err, fmt.Errorf("unpublished marker changed; retained"))
	}
	return prepared.removeMarker(info)
}

func (prepared *preparedNativeRoot) removeMarker(before os.FileInfo) error {
	if err := prepared.verifyNamed(); err != nil {
		return err
	}
	named, err := prepared.selected.root.Lstat(prepared.markerName)
	if err != nil || !sameNativeMarker(before, named) {
		return fmt.Errorf("unpublished marker changed before removal; retained")
	}
	if err := prepared.verifyNamed(); err != nil {
		return err
	}
	return errors.Join(prepared.selected.root.Remove(prepared.markerName), prepared.verifyNamed())
}

func removeEmptyNativeChild(base, parent, child *heldNativeRoot) error {
	verify := func() error { return errors.Join(base.verifyNamed(), parent.verifyNamed(), child.verifyNamed()) }
	if err := verify(); err != nil {
		return err
	}
	named, err := parent.root.Lstat(filepath.Base(child.path))
	if err != nil || !sameNativeRoot(child.info, named) {
		return fmt.Errorf("unpublished staging link changed; retained")
	}
	if err := verify(); err != nil {
		return err
	}
	// Remove is intentionally nonrecursive: unexpected children always survive.
	return errors.Join(parent.root.Remove(filepath.Base(child.path)), base.verifyNamed(), parent.verifyNamed())
}
