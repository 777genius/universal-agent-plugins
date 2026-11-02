//go:build linux

package pluginkitairepo_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type windProfileRecord struct {
	Kind              string
	Mode              uint32
	Device, Inode     uint64
	UID, GID          uint32
	Links             uint64
	Size              int64
	Modified, Changed [2]int64
	SHA256            string
}
type windFrameRefusal struct{ Root, Reason string }

func (e *windFrameRefusal) Error() string {
	return "Windsurf profile frame refused: " + e.Root + "/" + e.Reason
}

type windHeldEdge struct {
	file    *os.File
	parent  int
	name    string
	initial unix.Stat_t
}
type windFrameWalk struct {
	ctx     context.Context
	root    string
	records map[string]windProfileRecord
	entries int
	bytes   int64
}

var windProfileRoles = []string{"home", "tmp", "xdg-config", "xdg-cache", "xdg-data", "xdg-state"}

func windLinkCount[T ~uint32 | ~uint64](count T) uint64 { return uint64(count) }

func windRecord(s unix.Stat_t, kind, digest string) windProfileRecord {
	return windProfileRecord{Kind: kind, Mode: s.Mode, Device: s.Dev, Inode: s.Ino, UID: s.Uid, GID: s.Gid, Links: windLinkCount(s.Nlink), Size: s.Size,
		Modified: [2]int64{s.Mtim.Sec, s.Mtim.Nsec}, Changed: [2]int64{s.Ctim.Sec, s.Ctim.Nsec}, SHA256: digest}
}
func windSameStat(a, b unix.Stat_t) bool { return windRecord(a, "", "") == windRecord(b, "", "") }
func windOwned(s unix.Stat_t) bool {
	return s.Uid == uint32(os.Getuid()) && s.Gid == uint32(os.Getgid())
}
func windDirectoryIdentity(a, b unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Uid == b.Uid && a.Gid == b.Gid
}
func windFileStat(f *os.File) (unix.Stat_t, error) {
	var s unix.Stat_t
	err := unix.Fstat(int(f.Fd()), &s)
	return s, err
}

// Hold every directory edge from /; no pathname component is followed through an alias.
func windOpenDirectory(path string) (edges []windHeldEdge, err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "//") {
		return nil, fmt.Errorf("directory spelling")
	}
	defer func() {
		if err != nil {
			if windCloseEdges(edges) != nil {
				err = fmt.Errorf("directory close")
			}
			edges = nil
		}
	}()
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, fmt.Errorf("directory open")
	}
	root := os.NewFile(uintptr(fd), "held-root")
	s, e := windFileStat(root)
	if e != nil {
		if root.Close() != nil {
			return nil, fmt.Errorf("directory close")
		}
		return nil, fmt.Errorf("directory stat")
	}
	edges = append(edges, windHeldEdge{file: root, parent: -1, initial: s})
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		parent := edges[len(edges)-1].file
		var before unix.Stat_t
		if unix.Fstatat(int(parent.Fd()), part, &before, unix.AT_SYMLINK_NOFOLLOW) != nil || before.Mode&unix.S_IFMT != unix.S_IFDIR {
			return edges, fmt.Errorf("directory edge")
		}
		child, e := unix.Openat(int(parent.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			return edges, fmt.Errorf("directory open")
		}
		file := os.NewFile(uintptr(child), "held-directory")
		edges = append(edges, windHeldEdge{file: file, parent: int(parent.Fd()), name: part, initial: before})
		held, e := windFileStat(file)
		if e != nil || !windSameStat(before, held) {
			return edges, fmt.Errorf("directory identity")
		}
	}
	return edges, nil
}
func windCheckEdges(edges []windHeldEdge) error {
	for i, e := range edges {
		held, err := windFileStat(e.file)
		if err != nil || !windDirectoryIdentity(e.initial, held) || (i == len(edges)-1 && !windSameStat(e.initial, held)) {
			return fmt.Errorf("held directory changed")
		}
		if e.parent >= 0 {
			var linked unix.Stat_t
			if unix.Fstatat(e.parent, e.name, &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || !windSameStat(held, linked) {
				return fmt.Errorf("linked directory changed")
			}
		}
	}
	return nil
}
func windCloseEdges(edges []windHeldEdge) error {
	var err error
	for i := len(edges) - 1; i >= 0; i-- {
		if edges[i].file.Close() != nil {
			err = fmt.Errorf("directory close")
		}
	}
	return err
}
func windProfileSnapshot(ctx context.Context, fixture string) (result map[string]windProfileRecord, err error) {
	if ctx == nil || !filepath.IsAbs(fixture) || filepath.Clean(fixture) != fixture {
		return nil, &windFrameRefusal{"frame", "root"}
	}
	w := &windFrameWalk{ctx: ctx, records: map[string]windProfileRecord{}}
	held := [][]windHeldEdge{}
	defer func() {
		for _, edges := range held {
			if windCloseEdges(edges) != nil {
				result = nil
				err = &windFrameRefusal{"frame", "close"}
			}
		}
	}()
	for _, role := range windProfileRoles {
		w.root = role
		edges, e := windOpenDirectory(filepath.Join(fixture, role))
		if e != nil {
			return nil, &windFrameRefusal{role, "root-open"}
		}
		held = append(held, edges)
		last := edges[len(edges)-1]
		if !windOwned(last.initial) {
			return nil, &windFrameRefusal{role, "owner"}
		}
		if e = w.directory(last.file, role+"\x00.", 0); e != nil {
			return nil, &windFrameRefusal{role, e.Error()}
		}
		if windCheckEdges(edges) != nil {
			return nil, &windFrameRefusal{role, "root-changed"}
		}
	}
	for i, edges := range held {
		if windCheckEdges(edges) != nil {
			return nil, &windFrameRefusal{windProfileRoles[i], "root-changed"}
		}
	}
	if ctx.Err() != nil {
		return nil, &windFrameRefusal{"frame", "deadline"}
	}
	return w.records, nil
}
func (w *windFrameWalk) directory(file *os.File, key string, depth int) error {
	if w.ctx.Err() != nil {
		return fmt.Errorf("deadline")
	}
	before, err := windFileStat(file)
	if err != nil || !windOwned(before) || before.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("directory")
	}
	w.entries++
	if depth > 64 || w.entries > 4096 {
		return fmt.Errorf("entries-depth")
	}
	w.records[key] = windRecord(before, "directory", "")
	names, err := file.Readdirnames(4097)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("directory-read")
	}
	if len(names) > 4096 {
		return fmt.Errorf("entries-depth")
	}
	for _, name := range names {
		if name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
			return fmt.Errorf("entry-name")
		}
		childKey := strings.TrimSuffix(key, ".") + name
		if !strings.HasSuffix(key, "\x00.") {
			childKey = key + "/" + name
		}
		if err := w.child(file, name, childKey, depth+1); err != nil {
			return err
		}
	}
	after, err := windFileStat(file)
	if err != nil || !windSameStat(before, after) {
		return fmt.Errorf("directory-changed")
	}
	return nil
}
func (w *windFrameWalk) child(parent *os.File, name, key string, depth int) (err error) {
	if w.ctx.Err() != nil {
		return fmt.Errorf("deadline")
	}
	var before unix.Stat_t
	if unix.Fstatat(int(parent.Fd()), name, &before, unix.AT_SYMLINK_NOFOLLOW) != nil || !windOwned(before) {
		return fmt.Errorf("entry-owner-stat")
	}
	kind := before.Mode & unix.S_IFMT
	if kind != unix.S_IFDIR && (kind != unix.S_IFREG || before.Nlink != 1) {
		return fmt.Errorf("entry-type-links")
	}
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	if kind == unix.S_IFDIR {
		flags |= unix.O_DIRECTORY
	}
	fd, e := unix.Openat(int(parent.Fd()), name, flags, 0)
	if e != nil {
		return fmt.Errorf("entry-open")
	}
	file := os.NewFile(uintptr(fd), "held-profile-entry")
	defer func() {
		if file.Close() != nil {
			err = fmt.Errorf("entry-close")
		}
	}()
	opened, e := windFileStat(file)
	if e != nil || !windSameStat(before, opened) {
		return fmt.Errorf("entry-identity")
	}
	if kind == unix.S_IFDIR {
		err = w.directory(file, key, depth)
	} else {
		err = w.regular(file, key, depth, before)
	}
	if err != nil {
		return err
	}
	after, e := windFileStat(file)
	var linked unix.Stat_t
	if e != nil || !windSameStat(before, after) || unix.Fstatat(int(parent.Fd()), name, &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || !windSameStat(before, linked) {
		return fmt.Errorf("entry-changed")
	}
	return nil
}
func (w *windFrameWalk) regular(file *os.File, key string, depth int, before unix.Stat_t) error {
	w.entries++
	if depth > 64 || w.entries > 4096 || before.Size < 0 || before.Size > 64<<20 {
		return fmt.Errorf("file-bound")
	}
	w.bytes += before.Size
	if w.bytes > 256<<20 {
		return fmt.Errorf("total-bytes")
	}
	body, err := io.ReadAll(io.LimitReader(file, (64<<20)+1))
	if err != nil || int64(len(body)) != before.Size {
		return fmt.Errorf("file-read")
	}
	w.records[key] = windRecord(before, "file", windsurfConfigDigest(body))
	return nil
}
