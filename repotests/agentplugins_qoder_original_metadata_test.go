//go:build linux

package pluginkitairepo_test

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// Private, independently authored anonymous metadata, never account credentials.
// The pinned standalone may still rewrite it or create other unknown profile data.
type qoderMetadataFile struct {
	info   os.FileInfo
	digest string
}
type qoderOriginalMetadata struct {
	root          string
	profile, auth os.FileInfo
	files         [3]qoderMetadataFile
}

var qoderMetadataRecipe = [...]struct {
	name string
	mode os.FileMode
	size int64
}{
	{"installation_id", 0644, 36},
	{".auth/machine_id", 0600, 36},
	{".auth/.credential-transaction", 0600, 0},
}

func qoderMetadataRefused() error {
	return fmt.Errorf("Qoder authored anonymous metadata authority differs")
}

func qoderMetadataUUID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6], raw[8] = raw[6]&15|64, raw[8]&63|128
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[:4], raw[4:6], raw[6:8], raw[8:10], raw[10:]), nil
}

func qoderMetadataSame(before, after os.FileInfo, directory bool) bool {
	if before == nil || after == nil || !qoderLogOwned(after, directory) || before.Mode() != after.Mode() || !os.SameFile(before, after) {
		return false
	}
	a, okA := before.Sys().(*syscall.Stat_t)
	b, okB := after.Sys().(*syscall.Stat_t)
	return okA && okB && a.Uid == b.Uid && a.Gid == b.Gid && (directory || (a.Nlink == b.Nlink && before.Size() == after.Size()))
}

func qoderMetadataCreate(parent *os.File, name string, body []byte, mode os.FileMode) (err error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode))
	if err != nil {
		return qoderMetadataRefused()
	}
	file := os.NewFile(uintptr(fd), "qoder-authored-anonymous-metadata")
	defer func() { err = errors.Join(err, file.Close()) }()
	if err := file.Chmod(mode); err != nil {
		return qoderMetadataRefused()
	}
	if n, err := file.Write(body); err != nil || n != len(body) {
		return qoderMetadataRefused()
	}
	return nil
}

func qoderMetadataRead(parent *os.File, name string, mode os.FileMode, size int64) (result qoderMetadataFile, err error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return result, qoderMetadataRefused()
	}
	file := os.NewFile(uintptr(fd), "qoder-held-anonymous-metadata")
	defer func() { err = errors.Join(err, file.Close()) }()
	before, err := file.Stat()
	if err != nil || !qoderLogOwned(before, false) || !before.Mode().IsRegular() || before.Mode() != mode || before.Size() != size {
		return result, qoderMetadataRefused()
	}
	body, err := io.ReadAll(io.LimitReader(file, 37))
	after, statErr := file.Stat()
	var linked unix.Stat_t
	linkErr := unix.Fstatat(int(parent.Fd()), name, &linked, unix.AT_SYMLINK_NOFOLLOW)
	if err != nil || statErr != nil || linkErr != nil || int64(len(body)) != size || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) || !qoderMetadataLinked(before, &linked) {
		return result, qoderMetadataRefused()
	}
	sum := sha256.Sum256(body)
	return qoderMetadataFile{before, hex.EncodeToString(sum[:])}, nil
}

func qoderMetadataLinked(info os.FileInfo, linked *unix.Stat_t) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Dev == linked.Dev && stat.Ino == linked.Ino && stat.Mode == linked.Mode && stat.Nlink == linked.Nlink && stat.Uid == linked.Uid && stat.Gid == linked.Gid && stat.Size == linked.Size && stat.Mtim.Sec == linked.Mtim.Sec && stat.Mtim.Nsec == linked.Mtim.Nsec && stat.Ctim.Sec == linked.Ctim.Sec && stat.Ctim.Nsec == linked.Ctim.Nsec
}

func qoderMetadataAbsent(parent *os.File, name string) bool {
	var stat unix.Stat_t
	return errors.Is(unix.Fstatat(int(parent.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW), unix.ENOENT)
}

func qoderAuthorOriginalMetadata(root string, protected map[string]string) (result *qoderOriginalMetadata, err error) {
	installation, err := qoderMetadataUUID()
	if err != nil {
		return nil, err
	}
	machine, err := qoderMetadataUUID()
	if err != nil || machine == installation || protected == nil {
		return nil, qoderMetadataRefused()
	}
	profile, err := qoderLogOpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, profile.Close()) }()
	info, err := profile.Stat()
	if err != nil || !qoderLogOwned(info, true) || !qoderMetadataAbsent(profile, "installation_id") || !qoderMetadataAbsent(profile, ".auth") {
		return nil, qoderMetadataRefused()
	}
	if err := unix.Mkdirat(int(profile.Fd()), ".auth", 0700); err != nil {
		return nil, qoderMetadataRefused()
	}
	auth, err := qoderLogOpenAt(profile, ".auth", true)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, auth.Close()) }()
	bodies := []string{installation, machine, ""}
	for i, body := range bodies {
		parent := auth
		if i == 0 {
			parent = profile
		}
		if err := qoderMetadataCreate(parent, filepath.Base(qoderMetadataRecipe[i].name), []byte(body), qoderMetadataRecipe[i].mode); err != nil {
			return nil, err
		}
	}
	result = &qoderOriginalMetadata{root: root}
	if err := result.capture(profile, auth); err != nil {
		return nil, err
	}
	for i, body := range bodies {
		sum := sha256.Sum256([]byte(body))
		if result.files[i].digest != hex.EncodeToString(sum[:]) {
			return nil, qoderMetadataRefused()
		}
	}
	if err := result.check(); err != nil {
		return nil, err
	}
	for i, item := range qoderMetadataRecipe {
		protected[filepath.Join(root, item.name)] = result.files[i].digest
	}
	return result, nil
}

func (r *qoderOriginalMetadata) capture(profile, auth *os.File) error {
	var err error
	r.profile, err = profile.Stat()
	if err != nil || !qoderLogOwned(r.profile, true) {
		return qoderMetadataRefused()
	}
	r.auth, err = auth.Stat()
	if err != nil || !qoderLogOwned(r.auth, true) || r.auth.Mode() != os.ModeDir|0700 {
		return qoderMetadataRefused()
	}
	for i, item := range qoderMetadataRecipe {
		parent := auth
		if i == 0 {
			parent = profile
		}
		r.files[i], err = qoderMetadataRead(parent, filepath.Base(item.name), item.mode, item.size)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *qoderOriginalMetadata) check() (err error) {
	profile, err := qoderLogOpenRoot(r.root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, profile.Close()) }()
	auth, err := qoderLogOpenAt(profile, ".auth", true)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, auth.Close()) }()
	current := &qoderOriginalMetadata{root: r.root}
	if err := current.capture(profile, auth); err != nil {
		return err
	}
	if !qoderMetadataSame(r.profile, current.profile, true) || !qoderMetadataSame(r.auth, current.auth, true) {
		return qoderMetadataRefused()
	}
	for i, before := range r.files {
		if !qoderMetadataSame(before.info, current.files[i].info, false) || before.digest != current.files[i].digest {
			return qoderMetadataRefused()
		}
	}
	return current.linked(profile, auth)
}

func (r *qoderOriginalMetadata) linked(profile, auth *os.File) (err error) {
	current, err := qoderLogOpenRoot(r.root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, current.Close()) }()
	root, err := current.Stat()
	var linked unix.Stat_t
	if err != nil || !qoderMetadataSame(r.profile, root, true) || unix.Fstatat(int(profile.Fd()), ".auth", &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || !qoderMetadataDirectoryLinked(r.auth, &linked) {
		return qoderMetadataRefused()
	}
	held, err := auth.Stat()
	if err != nil || !qoderMetadataSame(r.auth, held, true) {
		return qoderMetadataRefused()
	}
	for i, item := range qoderMetadataRecipe {
		parent := auth
		if i == 0 {
			parent = profile
		}
		if unix.Fstatat(int(parent.Fd()), filepath.Base(item.name), &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || !qoderMetadataLinked(r.files[i].info, &linked) {
			return qoderMetadataRefused()
		}
	}
	return nil
}

func qoderMetadataDirectoryLinked(info os.FileInfo, linked *unix.Stat_t) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Dev == linked.Dev && stat.Ino == linked.Ino && stat.Mode == linked.Mode && stat.Uid == linked.Uid && stat.Gid == linked.Gid
}
