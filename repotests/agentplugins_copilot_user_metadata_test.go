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

const copilotUserMetadataEntries = 1024

type copilotUserMetadataNode struct {
	stat unix.Stat_t
	kind string
}
type copilotUserMetadata struct {
	parents []unix.Stat_t
	nodes   map[string]copilotUserMetadataNode
}

func copilotUserMetadataRefused() error {
	return fmt.Errorf("Copilot user profile metadata observation refused")
}
func copilotUserMetadataIdentity(a, b unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Uid == b.Uid && a.Gid == b.Gid
}
func copilotUserMetadataStable(a, b unix.Stat_t) bool {
	return copilotUserMetadataIdentity(a, b) && a.Nlink == b.Nlink && a.Size == b.Size && a.Mtim == b.Mtim && a.Ctim == b.Ctim
}
func copilotUserMetadataOwned(info unix.Stat_t) (string, bool) {
	if info.Uid != uint32(os.Getuid()) || info.Gid != uint32(os.Getgid()) {
		return "", false
	}
	switch info.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		return "directory", true
	case unix.S_IFREG:
		return "file", info.Nlink == 1
	default:
		return "", false
	}
}

// Open only the selected ordinary profile chain. System ancestors are identity
// checks, not an inventory or content read of ambient HOME.
func copilotUserMetadataRoot(ctx context.Context, root string) (file *os.File, parents []unix.Stat_t, err error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != ".copilot" || len(root) > 4096 {
		return nil, nil, copilotUserMetadataRefused()
	}
	parts := strings.Split(strings.TrimPrefix(root, "/"), "/")
	if len(parts) > 64 {
		return nil, nil, copilotUserMetadataRefused()
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, copilotUserMetadataRefused()
	}
	file = os.NewFile(uintptr(fd), "copilot-held-profile-parent")
	defer func() {
		if err != nil && file != nil {
			err = errors.Join(err, file.Close())
			file = nil
		}
	}()
	for _, part := range parts {
		if ctx.Err() != nil {
			return file, nil, copilotUserMetadataRefused()
		}
		var before unix.Stat_t
		if unix.Fstat(int(file.Fd()), &before) != nil {
			return file, nil, copilotUserMetadataRefused()
		}
		parents = append(parents, before)
		fd, openErr := unix.Openat(int(file.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if openErr != nil {
			return file, nil, copilotUserMetadataRefused()
		}
		next := os.NewFile(uintptr(fd), "copilot-held-profile-parent")
		closeErr := file.Close()
		file = next
		if closeErr != nil {
			return file, nil, copilotUserMetadataRefused()
		}
	}
	return file, parents, nil
}

func copilotUserMetadataLeaf(parent *os.File, name string, expected unix.Stat_t) (err error) {
	// O_PATH obtains metadata only; no unknown file body is opened for reading.
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return copilotUserMetadataRefused()
	}
	file := os.NewFile(uintptr(fd), "copilot-held-profile-metadata")
	defer func() { err = errors.Join(err, file.Close()) }()
	var held, linked unix.Stat_t
	if unix.Fstat(fd, &held) != nil || !copilotUserMetadataStable(expected, held) || unix.Fstatat(int(parent.Fd()), name, &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || !copilotUserMetadataStable(expected, linked) {
		return copilotUserMetadataRefused()
	}
	return nil
}

func (snapshot *copilotUserMetadata) directory(ctx context.Context, directory *os.File, relative string, depth int) error {
	if ctx.Err() != nil || depth > 32 || len(snapshot.nodes) >= copilotUserMetadataEntries {
		return copilotUserMetadataRefused()
	}
	var before unix.Stat_t
	if unix.Fstat(int(directory.Fd()), &before) != nil {
		return copilotUserMetadataRefused()
	}
	kind, owned := copilotUserMetadataOwned(before)
	if !owned || kind != "directory" {
		return copilotUserMetadataRefused()
	}
	snapshot.nodes[relative] = copilotUserMetadataNode{before, kind}
	names, err := directory.Readdirnames(copilotUserMetadataEntries + 1)
	if (err != nil && !errors.Is(err, io.EOF)) || len(names)+len(snapshot.nodes) > copilotUserMetadataEntries {
		return copilotUserMetadataRefused()
	}
	children := make(map[string]unix.Stat_t, len(names))
	for _, name := range names {
		if ctx.Err() != nil || name == "" || name == "." || name == ".." || filepath.Base(name) != name || len(snapshot.nodes) >= copilotUserMetadataEntries {
			return copilotUserMetadataRefused()
		}
		var info unix.Stat_t
		if unix.Fstatat(int(directory.Fd()), name, &info, unix.AT_SYMLINK_NOFOLLOW) != nil {
			return copilotUserMetadataRefused()
		}
		kind, owned := copilotUserMetadataOwned(info)
		if !owned {
			return copilotUserMetadataRefused()
		}
		child := filepath.Join(relative, name)
		if _, duplicate := snapshot.nodes[child]; duplicate {
			return copilotUserMetadataRefused()
		}
		children[name] = info
		if kind == "directory" {
			if err := snapshot.child(ctx, directory, name, child, depth+1, info); err != nil {
				return err
			}
		} else {
			if err := copilotUserMetadataLeaf(directory, name, info); err != nil {
				return err
			}
			snapshot.nodes[child] = copilotUserMetadataNode{info, kind}
		}
	}
	var after unix.Stat_t
	if unix.Fstat(int(directory.Fd()), &after) != nil || !copilotUserMetadataStable(before, after) {
		return copilotUserMetadataRefused()
	}
	for name, info := range children {
		var current unix.Stat_t
		if unix.Fstatat(int(directory.Fd()), name, &current, unix.AT_SYMLINK_NOFOLLOW) != nil || !copilotUserMetadataStable(info, current) {
			return copilotUserMetadataRefused()
		}
	}
	return nil
}

func (snapshot *copilotUserMetadata) child(ctx context.Context, parent *os.File, name, relative string, depth int, expected unix.Stat_t) (err error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return copilotUserMetadataRefused()
	}
	child := os.NewFile(uintptr(fd), "copilot-held-profile-directory")
	defer func() { err = errors.Join(err, child.Close()) }()
	var opened unix.Stat_t
	if unix.Fstat(fd, &opened) != nil || !copilotUserMetadataStable(expected, opened) {
		return copilotUserMetadataRefused()
	}
	return snapshot.directory(ctx, child, relative, depth)
}

func copilotUserMetadataCapture(ctx context.Context, root string) (snapshot *copilotUserMetadata, err error) {
	directory, parents, err := copilotUserMetadataRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	snapshot = &copilotUserMetadata{parents: parents, nodes: map[string]copilotUserMetadataNode{}}
	if err := snapshot.directory(ctx, directory, ".", 0); err != nil {
		return nil, err
	}
	linked, again, err := copilotUserMetadataRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, linked.Close()) }()
	var current unix.Stat_t
	if len(parents) != len(again) || unix.Fstat(int(linked.Fd()), &current) != nil || !copilotUserMetadataStable(snapshot.nodes["."].stat, current) {
		return nil, copilotUserMetadataRefused()
	}
	for i, before := range parents {
		if !copilotUserMetadataIdentity(before, again[i]) {
			return nil, copilotUserMetadataRefused()
		}
	}
	return snapshot, nil
}
