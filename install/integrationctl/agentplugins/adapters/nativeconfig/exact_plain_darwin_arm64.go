//go:build darwin && arm64

package nativeconfig

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/unix"
)

type plainDarwinGuard struct {
	path, name                    string
	parent                        *os.File
	parentImage                   plainImage
	original, current             plainImage
	produced                      *os.File
	published, restored, poisoned bool
}

type plainStage struct {
	file  *os.File
	name  string
	image plainImage
}

func capturePlainOS(path string) (plainExactGuard, FileSnapshot, error) {
	fd, err := unix.Open(filepath.Dir(path), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, FileSnapshot{}, err
	}
	g := &plainDarwinGuard{path: path, name: filepath.Base(path), parent: os.NewFile(uintptr(fd), "plain-parent")}
	g.parentImage, err = plainCapture(g.parent, true)
	if err == nil {
		g.original, err = g.readName(g.name)
	}
	if err == nil {
		err = g.checkParent(false, 0)
	}
	if err != nil {
		return nil, FileSnapshot{}, errors.Join(err, g.close())
	}
	g.current = g.original
	return g, g.original.snapshot(), nil
}

func (g *plainDarwinGuard) readName(name string) (plainImage, error) {
	defer runtime.KeepAlive(g.parent)
	fd, err := unix.Openat(int(g.parent.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) {
		return plainImage{}, nil
	}
	if err != nil {
		return plainImage{}, err
	}
	f := os.NewFile(uintptr(fd), "plain-child")
	image, err := plainCapture(f, false)
	if err == nil && (image.stat.Dev != g.parentImage.stat.Dev || image.fs != g.parentImage.fs) {
		err = errors.New("plain exact child differs from held parent filesystem")
	}
	if err == nil {
		var named unix.Stat_t
		err = unix.Fstatat(int(g.parent.Fd()), name, &named, unix.AT_SYMLINK_NOFOLLOW)
		if err == nil && !plainSameStat(image.stat, named, true) {
			err = ErrConcurrentChange
		}
	}
	return image, errors.Join(err, f.Close())
}

// Directory aliases resolve compatibly, but the selected parent must still be
// that held directory. Only our immediately bracketed namespace operations may
// rebind directory epochs; identity and security never relax.
func (g *plainDarwinGuard) checkParent(ownNamespaceChange bool, childDelta int) error {
	image, err := plainCapture(g.parent, true)
	if err != nil {
		return err
	}
	if !plainSameParent(g.parentImage, image, !ownNamespaceChange, childDelta) {
		return ErrConcurrentChange
	}
	fd, err := unix.Open(filepath.Dir(g.path), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	var st unix.Stat_t
	err = unix.Fstat(fd, &st)
	err = errors.Join(err, unix.Close(fd))
	if err != nil {
		return err
	}
	if !plainSameStat(image.stat, st, true) {
		return ErrConcurrentChange
	}
	if ownNamespaceChange {
		g.parentImage = image
	}
	return nil
}

func (g *plainDarwinGuard) checkCurrent() error {
	if g.poisoned {
		return errors.New("plain exact publication/durability is uncertain")
	}
	image, err := g.readName(g.name)
	if err != nil {
		return err
	}
	if !plainSame(g.current, image, true) {
		return ErrConcurrentChange
	}
	if g.produced != nil {
		held, err := plainCapture(g.produced, false)
		if err != nil {
			return err
		}
		if !plainSame(image, held, true) {
			return ErrConcurrentChange
		}
	}
	return g.checkParent(false, 0)
}

func (g *plainDarwinGuard) effect() (FileEffect, error) {
	if err := g.checkCurrent(); err != nil {
		return FileUncertain, err
	}
	if !g.published || g.restored {
		return FileUnchanged, nil
	}
	return FileCommitted, nil
}

func (g *plainDarwinGuard) apply(body []byte) (FileEffect, error) {
	if err := g.checkCurrent(); err != nil {
		return FileUncertain, err
	}
	if g.original.exists && bytes.Equal(body, g.original.body) {
		return FileUnchanged, nil
	}
	stage, err := g.stage(body)
	if err == nil {
		err = g.publish(stage, !g.original.exists)
	}
	if stage != nil {
		err = errors.Join(err, g.cleanup(stage))
	}
	effect, observed := g.effect()
	return effect, errors.Join(err, observed)
}

func (g *plainDarwinGuard) stage(body []byte) (*plainStage, error) {
	if err := g.checkCurrent(); err != nil {
		return nil, err
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	stage := &plainStage{name: ".plain-exact-" + hex.EncodeToString(token[:])}
	fd, err := unix.Openat(int(g.parent.Fd()), stage.name, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	runtime.KeepAlive(g.parent)
	if err != nil {
		return nil, err
	}
	stage.file = os.NewFile(uintptr(fd), "plain-stage")
	if err = g.checkParent(true, 1); err != nil {
		return stage, err
	}
	if _, err = stage.file.Write(body); err != nil {
		return stage, err
	}
	mode := os.FileMode(0600)
	if g.original.exists {
		mode = os.FileMode(g.original.stat.Mode & 0777)
	}
	if err = stage.file.Chmod(mode); err != nil {
		return stage, err
	}
	if err = stage.file.Sync(); err != nil {
		return stage, err
	}
	stage.image, err = plainCapture(stage.file, false)
	if err != nil {
		return stage, err
	}
	if !bytes.Equal(body, stage.image.body) || stage.image.stat.Dev != g.parentImage.stat.Dev || stage.image.fs != g.parentImage.fs {
		return stage, ErrConcurrentChange
	}
	if g.original.exists && (stage.image.stat.Mode != g.original.stat.Mode || stage.image.stat.Uid != g.original.stat.Uid ||
		stage.image.stat.Gid != g.original.stat.Gid || !plainMetadataEqual(stage.image, g.original)) {
		return stage, errors.New("naturally created stage metadata differs from original")
	}
	named, err := g.readName(stage.name)
	if err != nil || !plainSame(stage.image, named, true) {
		return stage, errors.Join(err, ErrConcurrentChange)
	}
	return stage, g.checkParent(false, 0)
}

// Staging precedes this final original/output guard. There is still a final
// external syscall race: descriptor/name checks do not supply linearizable CAS.
func (g *plainDarwinGuard) publish(stage *plainStage, exclusive bool) error {
	if err := g.checkCurrent(); err != nil {
		return err
	}
	var err error
	childDelta := -1
	if exclusive {
		childDelta = 0
		err = unix.RenameatxNp(int(g.parent.Fd()), stage.name, int(g.parent.Fd()), g.name, unix.RENAME_EXCL)
	} else {
		err = unix.Renameat(int(g.parent.Fd()), stage.name, int(g.parent.Fd()), g.name)
	}
	runtime.KeepAlive(g.parent)
	if err != nil {
		return err
	}
	g.published, g.poisoned = true, true
	// No output ownership exists until independent fd/name verification and sync.
	image, err := g.certifyPublished(stage)
	if err != nil {
		return err
	}
	if err = g.checkParent(true, childDelta); err != nil {
		return err
	}
	if err = g.parent.Sync(); err != nil {
		return err
	}
	if err = g.checkParent(false, 0); err != nil {
		return err
	}
	after, err := g.certifyPublished(stage)
	if err != nil || !plainSame(image, after, true) {
		return errors.Join(err, ErrConcurrentChange)
	}
	if g.produced != nil {
		if err = g.produced.Close(); err != nil {
			return err
		}
	}
	g.produced, stage.file = stage.file, nil
	g.current, g.poisoned = after, false
	return nil
}

func (g *plainDarwinGuard) certifyPublished(stage *plainStage) (plainImage, error) {
	held, err := plainCapture(stage.file, false)
	if err != nil {
		return held, err
	}
	// Rename establishes a legitimate new ctime; all security, identity and
	// bytes remain equal to the independently certified actual stage.
	if !plainSame(stage.image, held, false) || stage.image.stat.Size != held.stat.Size || stage.image.stat.Mtim != held.stat.Mtim {
		return held, ErrConcurrentChange
	}
	named, err := g.readName(g.name)
	if err != nil || !plainSame(held, named, true) {
		return held, errors.Join(err, ErrConcurrentChange)
	}
	return held, nil
}

func (g *plainDarwinGuard) rollback() (FileEffect, error) {
	if err := g.checkCurrent(); err != nil {
		return FileUncertain, err
	}
	if !g.published || g.restored {
		return FileUnchanged, nil
	}
	if !g.original.exists {
		return g.removeProduced()
	}
	stage, err := g.stage(g.original.body)
	if err == nil {
		err = g.publish(stage, false)
	}
	if stage != nil {
		err = errors.Join(err, g.cleanup(stage))
	}
	if err != nil {
		return FileUncertain, err
	}
	g.restored = true
	return g.effect()
}

func (g *plainDarwinGuard) removeProduced() (FileEffect, error) {
	// The produced fd/name/epoch/bytes/security check is immediately before unlink.
	if err := g.checkCurrent(); err != nil {
		return FileUncertain, err
	}
	err := unix.Unlinkat(int(g.parent.Fd()), g.name, 0)
	runtime.KeepAlive(g.parent)
	if err != nil {
		return FileUncertain, err
	}
	g.poisoned = true
	if err = g.checkParent(true, -1); err != nil {
		return FileUncertain, err
	}
	if err = g.parent.Sync(); err != nil {
		return FileUncertain, err
	}
	image, err := g.readName(g.name)
	if err != nil || image.exists {
		return FileUncertain, errors.Join(err, ErrConcurrentChange)
	}
	if err = g.produced.Close(); err != nil {
		return FileUncertain, err
	}
	g.produced, g.current = nil, plainImage{}
	g.restored, g.poisoned = true, false
	return g.effect()
}

// Even failed staging never removes a foreign replacement of our temporary
// name. Query failures leave the temporary artifact; Close releases descriptors.
func (g *plainDarwinGuard) cleanup(stage *plainStage) (result error) {
	if stage.file == nil {
		return nil
	}
	defer func() { result = errors.Join(result, stage.file.Close()) }()
	var held, named unix.Stat_t
	if err := unix.Fstat(int(stage.file.Fd()), &held); err != nil {
		return err
	}
	err := unix.Fstatat(int(g.parent.Fd()), stage.name, &named, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	if !plainSameStat(held, named, true) {
		return ErrConcurrentChange
	}
	if err = g.checkParent(false, 0); err != nil {
		return err
	}
	if err = unix.Unlinkat(int(g.parent.Fd()), stage.name, 0); err != nil {
		return err
	}
	runtime.KeepAlive(g.parent)
	return g.checkParent(true, -1)
}

func (g *plainDarwinGuard) close() error {
	var err error
	if g.produced != nil {
		err = g.produced.Close()
		g.produced = nil
	}
	if g.parent != nil {
		err = errors.Join(err, g.parent.Close())
		g.parent = nil
	}
	return err
}
