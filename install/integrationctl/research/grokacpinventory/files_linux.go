//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type fileID struct {
	Dev, Ino, Links uint64
	Mode, UID, GID  uint32
	Size            int64
	Mtime, Ctime    syscall.Timespec
}
type heldFile struct {
	file   *os.File
	path   string
	id     fileID
	digest string
}

func identity(info os.FileInfo) (fileID, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}, errors.New("file-stat-refused")
	}
	return fileID{st.Dev, st.Ino, st.Nlink, st.Mode, st.Uid, st.Gid, st.Size, st.Mtim, st.Ctim}, nil
}
func anchoredInfo(path string) (os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("absolute-path-refused")
	}
	root, e := os.OpenRoot("/")
	if e != nil {
		return nil, e
	}
	defer func() { _ = root.Close() }()
	relative := strings.TrimPrefix(path, "/")
	if relative == "" {
		relative = "."
	}
	return root.Lstat(relative)
}
func (h heldFile) unchanged() error {
	info, e := h.file.Stat()
	linked, le := anchoredInfo(h.path)
	if e != nil || le != nil {
		return errors.New("held-file-refused")
	}
	a, e := identity(info)
	b, be := identity(linked)
	if e != nil || be != nil || a != h.id || b != h.id {
		return errors.New("held-file-changed")
	}
	return nil
}
func openHeld(path string, uid int, limit int64) (heldFile, error) {
	info, e := anchoredInfo(path)
	if e != nil {
		return heldFile{}, e
	}
	id, e := identity(info)
	if e != nil || !info.Mode().IsRegular() || int64(id.UID) != int64(uid) || id.Links != 1 || id.Size < 0 || id.Size > limit || info.Mode().Perm()&022 != 0 {
		return heldFile{}, errors.New("file-authority-refused")
	}
	fd, e := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if e != nil {
		return heldFile{}, e
	}
	h := heldFile{file: os.NewFile(uintptr(fd), path), path: path, id: id}
	if e = h.unchanged(); e != nil {
		return heldFile{}, errors.Join(e, h.file.Close())
	}
	return h, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(body []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.reader.Read(body)
}
func digestFile(ctx context.Context, h heldFile) (string, error) {
	hash := sha256.New()
	n, e := io.Copy(hash, contextReader{ctx, io.NewSectionReader(h.file, 0, h.id.Size)})
	if e != nil || n != h.id.Size {
		return "", errors.New("file-read-refused")
	}
	if e = h.unchanged(); e != nil {
		return "", e
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func (h heldFile) verified(ctx context.Context) error {
	if e := h.unchanged(); e != nil {
		return e
	}
	if h.digest != "" {
		sum, e := digestFile(ctx, h)
		if e != nil || sum != h.digest {
			return errors.New("held-content-changed")
		}
	}
	return nil
}
func immutable(path string) error {
	current := "/"
	parts := append([]string{""}, strings.Split(strings.TrimPrefix(path, "/"), "/")...)
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, e := anchoredInfo(current)
		if e != nil {
			return e
		}
		id, e := identity(info)
		if e != nil || !info.IsDir() || id.UID != 0 || info.Mode().Perm()&022 != 0 {
			return errors.New("system-ancestor-refused")
		}
	}
	return nil
}
func safeTemporaryParent() error {
	if e := immutable("/"); e != nil {
		return e
	}
	info, e := anchoredInfo("/tmp")
	if e != nil {
		return e
	}
	id, e := identity(info)
	if e != nil || !info.IsDir() || id.UID != 0 || info.Mode()&os.ModeSticky == 0 {
		return errors.New("temporary-parent-refused")
	}
	return nil
}
func systemFile(ctx context.Context, path string) (heldFile, error) {
	canonical, e := filepath.EvalSymlinks(path)
	if e != nil {
		return heldFile{}, e
	}
	if e = immutable(filepath.Dir(canonical)); e != nil {
		return heldFile{}, e
	}
	h, e := openHeld(canonical, 0, 128<<20)
	if e != nil {
		return h, e
	}
	if h.id.Mode&0111 == 0 {
		return heldFile{}, errors.Join(errors.New("system-executable-refused"), h.file.Close())
	}
	h.digest, e = digestFile(ctx, h)
	if e != nil {
		return heldFile{}, errors.Join(e, h.file.Close())
	}
	return h, nil
}
func sealFile(ctx context.Context, source, destination, want string, uid int, mode os.FileMode) (heldFile, error) {
	h, e := openHeld(source, uid, 256<<20)
	if e != nil {
		return h, e
	}
	h.digest = want
	file, e := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return h, e
	}
	hash := sha256.New()
	n, e := io.Copy(io.MultiWriter(file, hash), contextReader{ctx, io.NewSectionReader(h.file, 0, h.id.Size)})
	closeErr := file.Close()
	if e != nil || closeErr != nil || n != h.id.Size || hex.EncodeToString(hash.Sum(nil)) != want {
		return h, errors.New("sealed-copy-refused")
	}
	if e = h.unchanged(); e != nil {
		return h, e
	}
	return h, os.Chmod(destination, mode)
}
func pinnedRuntime(ctx context.Context, path, want string, uid int) (heldFile, error) {
	h, e := openHeld(path, uid, 256<<20)
	if e != nil {
		return h, e
	}
	h.digest = want
	if e = h.verified(ctx); e != nil {
		return heldFile{}, errors.Join(e, h.file.Close())
	}
	return h, nil
}

type directoryAuthority struct {
	file            *os.File
	path            string
	dev, ino, mount uint64
	mode            os.FileMode
	uid, gid        int
}

func mountID(fd int, path string) (uint64, error) {
	var st unix.Statx_t
	flags := unix.AT_SYMLINK_NOFOLLOW
	if path == "" {
		flags |= unix.AT_EMPTY_PATH
	}
	if e := unix.Statx(fd, path, flags, unix.STATX_MNT_ID, &st); e != nil {
		return 0, e
	}
	if st.Mask&unix.STATX_MNT_ID == 0 || st.Mnt_id == 0 {
		return 0, errors.New("mount-authority-refused")
	}
	return st.Mnt_id, nil
}
func holdDirectory(path string, uid, gid int) (*directoryAuthority, error) {
	fd, e := unix.Open(path, unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	file := os.NewFile(uintptr(fd), path)
	fail := func(e error) (*directoryAuthority, error) { return nil, errors.Join(e, file.Close()) }
	info, e := file.Stat()
	if e != nil {
		return fail(e)
	}
	id, e := identity(info)
	if e != nil {
		return fail(e)
	}
	mount, e := mountID(fd, "")
	if e != nil {
		return fail(e)
	}
	d := &directoryAuthority{file, path, id.Dev, id.Ino, mount, info.Mode(), uid, gid}
	if e = d.verify(); e != nil {
		return fail(e)
	}
	return d, nil
}
func (d *directoryAuthority) verify() error {
	info, e := d.file.Stat()
	linked, le := anchoredInfo(d.path)
	if e != nil || le != nil {
		return errors.New("sealed-directory-refused")
	}
	for _, item := range []os.FileInfo{info, linked} {
		id, e := identity(item)
		if e != nil || !item.IsDir() || id.Dev != d.dev || id.Ino != d.ino || int64(id.UID) != int64(d.uid) || int64(id.GID) != int64(d.gid) || item.Mode() != d.mode {
			return errors.New("sealed-directory-changed")
		}
	}
	held, e := mountID(int(d.file.Fd()), "")
	current, ce := mountID(unix.AT_FDCWD, d.path)
	if e != nil || ce != nil || held != d.mount || current != d.mount {
		return errors.New("sealed-mount-changed")
	}
	return nil
}
func (d *directoryAuthority) seal() error {
	if e := d.verify(); e != nil {
		return e
	}
	if e := os.Chmod(d.path, 0755); e != nil {
		return e
	}
	d.mode = d.mode&^os.ModePerm | 0755
	return d.verify()
}
func (d *directoryAuthority) remove() error {
	if e := d.verify(); e != nil {
		return e
	}
	if e := os.RemoveAll(d.path); e != nil {
		return e
	}
	if _, e := anchoredInfo(d.path); !os.IsNotExist(e) {
		return errors.New("sealed-deletion-unproved")
	}
	return nil
}

// This decoder is for the closed controller envelope. The collector separately
// applies the stricter native-frame bounds before constructing its result.
func jsonValue(d *json.Decoder, depth int, count *int) error {
	*count++
	if depth > 20 || *count > 8192 {
		return errors.New("json-structure-refused")
	}
	token, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return e
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("json-key-refused")
			}
			seen[name] = true
			if e = jsonValue(d, depth+1, count); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if e = jsonValue(d, depth+1, count); e != nil {
				return e
			}
		}
	default:
		return errors.New("json-delimiter-refused")
	}
	_, e = d.Token()
	return e
}
func closed(body []byte, target any, fields string) error {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	count := 0
	if e := jsonValue(d, 0, &count); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("json-trailing-refused")
	}
	var keys map[string]json.RawMessage
	if e := json.Unmarshal(body, &keys); e != nil {
		return e
	}
	want := strings.Fields(fields)
	if len(keys) != len(want) {
		return errors.New("json-fields-refused")
	}
	for _, key := range want {
		if _, ok := keys[key]; !ok {
			return errors.New("json-field-missing")
		}
	}
	d = json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	return d.Decode(target)
}
