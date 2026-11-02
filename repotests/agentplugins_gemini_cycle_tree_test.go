//go:build linux

package pluginkitairepo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

type geminiCycleRecord struct {
	Mode                 uint32 `json:"mode"`
	Device, Inode, Links uint64
	UID, GID             uint32
	Size                 int64 `json:"bytes"`
	Modified, Changed    [2]int64
	SHA256               string `json:"sha256,omitempty"`
}
type geminiCycleEdge struct {
	file    *os.File
	initial geminiCycleRecord
	name    string
}

func geminiCycleSHA(body []byte) string                        { h := sha256.Sum256(body); return hex.EncodeToString(h[:]) }
func geminiCycleLinkCount[T ~uint32 | ~uint64](count T) uint64 { return uint64(count) }
func geminiCycleRecordOf(s unix.Stat_t) geminiCycleRecord {
	return geminiCycleRecord{Mode: s.Mode, Device: s.Dev, Inode: s.Ino, Links: geminiCycleLinkCount(s.Nlink), UID: s.Uid, GID: s.Gid, Size: s.Size, Modified: [2]int64{s.Mtim.Sec, s.Mtim.Nsec}, Changed: [2]int64{s.Ctim.Sec, s.Ctim.Nsec}}
}
func geminiCycleStat(file *os.File) (geminiCycleRecord, error) {
	var s unix.Stat_t
	err := unix.Fstat(int(file.Fd()), &s)
	return geminiCycleRecordOf(s), err
}
func geminiCycleSameDirectory(a, b geminiCycleRecord) bool {
	return a.Mode == b.Mode && a.Mode&unix.S_IFMT == unix.S_IFDIR && a.Device == b.Device && a.Inode == b.Inode && a.UID == b.UID && a.GID == b.GID
}
func geminiCycleClose(edges []geminiCycleEdge) error {
	var result error
	for i := len(edges) - 1; i >= 0; i-- {
		result = errors.Join(result, edges[i].file.Close())
	}
	return result
}
func geminiCycleOpen(path string) (edges []geminiCycleEdge, err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "//") {
		return nil, fmt.Errorf("cycle canonical directory required")
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, geminiCycleClose(edges))
			edges = nil
		}
	}()
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("cycle directory open")
	}
	root := os.NewFile(uintptr(fd), "cycle-root")
	s, err := geminiCycleStat(root)
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("cycle directory stat")
	}
	edges = append(edges, geminiCycleEdge{root, s, ""})
	for _, name := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if name == "" {
			continue
		}
		parent := edges[len(edges)-1].file
		child, e := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			return edges, fmt.Errorf("cycle directory alias/open")
		}
		file := os.NewFile(uintptr(child), "cycle-directory")
		st, e := geminiCycleStat(file)
		if e != nil {
			_ = file.Close()
			return edges, fmt.Errorf("cycle directory stat")
		}
		edges = append(edges, geminiCycleEdge{file, st, name})
	}
	return edges, nil
}
func geminiCycleEdges(edges []geminiCycleEdge) error {
	for i, e := range edges {
		held, err := geminiCycleStat(e.file)
		if err != nil || !geminiCycleSameDirectory(e.initial, held) || (i == len(edges)-1 && e.initial != held) {
			return fmt.Errorf("cycle directory identity changed")
		}
		if i > 0 {
			var linked unix.Stat_t
			if unix.Fstatat(int(edges[i-1].file.Fd()), e.name, &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || geminiCycleRecordOf(linked) != held {
				return fmt.Errorf("cycle linked directory changed")
			}
		}
	}
	return nil
}

type geminiCycleWalker struct {
	ctx   context.Context
	tree  map[string]geminiCycleRecord
	bytes int64
	nodes int
}

func (w *geminiCycleWalker) visit(file *os.File, key string, depth int) error {
	w.nodes++
	if w.ctx.Err() != nil || depth > 64 || w.nodes > 4096 {
		return fmt.Errorf("cycle tree bound/deadline")
	}
	before, err := geminiCycleStat(file)
	if err != nil || before.UID != uint32(os.Getuid()) || before.GID != uint32(os.Getgid()) {
		return fmt.Errorf("cycle tree held owner")
	}
	switch before.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		if before.Links != 1 || before.Size < 0 || before.Size > 64<<20 || w.bytes+before.Size > 256<<20 {
			return fmt.Errorf("cycle tree ordinary file/bound")
		}
		h := sha256.New()
		n, e := io.Copy(h, io.LimitReader(file, (64<<20)+1))
		if e != nil || n != before.Size {
			return fmt.Errorf("cycle tree whole body")
		}
		before.SHA256 = hex.EncodeToString(h.Sum(nil))
		w.bytes += n
	case unix.S_IFDIR:
		names, e := file.Readdirnames(4097)
		if e != nil && !errors.Is(e, io.EOF) || len(names) > 4096 {
			return fmt.Errorf("cycle tree directory bound")
		}
		sort.Strings(names)
		for _, name := range names {
			if name == "." || name == ".." || name == "" || filepath.Base(name) != name {
				return fmt.Errorf("cycle tree leaf")
			}
			fd, e := unix.Openat(int(file.Fd()), name, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			if e != nil {
				return fmt.Errorf("cycle tree child open")
			}
			child := os.NewFile(uintptr(fd), "cycle-child")
			childErr := w.visit(child, filepath.Join(key, name), depth+1)
			held, statErr := geminiCycleStat(child)
			var linked unix.Stat_t
			linkErr := unix.Fstatat(int(file.Fd()), name, &linked, unix.AT_SYMLINK_NOFOLLOW)
			closeErr := child.Close()
			if childErr != nil || statErr != nil || linkErr != nil || closeErr != nil || held != geminiCycleRecordOf(linked) {
				return fmt.Errorf("cycle tree child identity/read/close")
			}
		}
	default:
		return fmt.Errorf("cycle tree special/alias entry")
	}
	after, e := geminiCycleStat(file)
	expected := before
	expected.SHA256 = ""
	if e != nil || after != expected || w.ctx.Err() != nil {
		return fmt.Errorf("cycle tree changed/deadline")
	}
	w.tree[key] = before
	return nil
}
func geminiCycleTree(ctx context.Context, path string) (result map[string]geminiCycleRecord, err error) {
	edges, err := geminiCycleOpen(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if e := geminiCycleClose(edges); e != nil {
			result = nil
			err = fmt.Errorf("cycle checked directory close")
		}
	}()
	w := geminiCycleWalker{ctx: ctx, tree: map[string]geminiCycleRecord{}}
	if err = w.visit(edges[len(edges)-1].file, ".", 0); err != nil {
		return nil, err
	}
	if err = geminiCycleEdges(edges); err != nil {
		return nil, err
	}
	return w.tree, nil
}
