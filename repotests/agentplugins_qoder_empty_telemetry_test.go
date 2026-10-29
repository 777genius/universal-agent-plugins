//go:build linux

package pluginkitairepo_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

var qoderTelemetryNames = [...]string{"tmp", "telemetry", "active-runs"}

type qoderEmptyTelemetry struct {
	ctx         context.Context
	root        string
	parents     []os.FileInfo
	profile     os.FileInfo
	directories [3]os.FileInfo
}

// System ancestors are identity authorities, not inventories of ambient HOME.
func qoderTelemetryRoot(ctx context.Context, root string) (file *os.File, parents []os.FileInfo, err error) {
	parts := strings.Split(strings.TrimPrefix(root, "/"), "/")
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != ".qoder" || len(root) > 4096 || len(parts) > 64 {
		return nil, nil, qoderMetadataRefused()
	}
	fd, err := unix.Open("/", unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, qoderMetadataRefused()
	}
	file = os.NewFile(uintptr(fd), "qoder-telemetry-original-ancestor")
	defer func() {
		if err != nil && file != nil {
			err = errors.Join(err, file.Close())
			file = nil
		}
	}()
	for _, part := range parts {
		info, statErr := file.Stat()
		if ctx.Err() != nil || statErr != nil {
			return file, nil, qoderMetadataRefused()
		}
		parents = append(parents, info)
		next, openErr := qoderLogOpenAt(file, part, true)
		closeErr := file.Close()
		file = next
		if openErr != nil || closeErr != nil {
			return file, nil, qoderMetadataRefused()
		}
	}
	return file, parents, nil
}

func qoderTelemetrySameParents(before, after []os.FileInfo) bool {
	if len(before) != len(after) {
		return false
	}
	for i, old := range before {
		now := after[i]
		a, okA := old.Sys().(*syscall.Stat_t)
		b, okB := now.Sys().(*syscall.Stat_t)
		if !okA || !okB || !os.SameFile(old, now) || old.Mode() != now.Mode() || a.Uid != b.Uid || a.Gid != b.Gid {
			return false
		}
	}
	return true
}

func qoderTelemetryCreate(parent *os.File, name string) (file *os.File, err error) {
	if !qoderMetadataAbsent(parent, name) || unix.Mkdirat(int(parent.Fd()), name, 0700) != nil {
		return nil, qoderMetadataRefused()
	}
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, qoderMetadataRefused()
	}
	file = os.NewFile(uintptr(fd), "qoder-authored-empty-telemetry-directory")
	if err := file.Chmod(0700); err != nil {
		return nil, errors.Join(qoderMetadataRefused(), file.Close())
	}
	return file, nil
}

func qoderAuthorEmptyTelemetry(ctx context.Context, root string) (result *qoderEmptyTelemetry, err error) {
	profile, parents, err := qoderTelemetryRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, profile.Close()) }()
	info, err := profile.Stat()
	if err != nil || !qoderLogOwned(info, true) || info.Mode() != os.ModeDir|0700 {
		return nil, qoderMetadataRefused()
	}
	held := []*os.File{}
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			err = errors.Join(err, held[i].Close())
		}
	}()
	parent := profile
	for _, name := range qoderTelemetryNames {
		if ctx.Err() != nil {
			return nil, qoderMetadataRefused()
		}
		child, createErr := qoderTelemetryCreate(parent, name)
		if createErr != nil {
			return nil, createErr
		}
		held, parent = append(held, child), child
	}
	result = &qoderEmptyTelemetry{ctx: ctx, root: root, parents: parents}
	result.profile, err = profile.Stat()
	if err != nil {
		return nil, qoderMetadataRefused()
	}
	for i, directory := range held {
		result.directories[i], err = directory.Stat()
		if err != nil || !qoderLogOwned(result.directories[i], true) || result.directories[i].Mode() != os.ModeDir|0700 {
			return nil, qoderMetadataRefused()
		}
	}
	if err := result.check(); err != nil {
		return nil, err
	}
	return result, nil
}

// Enumerate only the fixed expected name, or exact emptiness. Unknown child
// bodies are never opened. A fresh descriptor starts every check at offset zero.
func qoderTelemetryMembers(directory *os.File, expected os.FileInfo, child string) (err error) {
	fd, err := unix.Openat(int(directory.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return qoderMetadataRefused()
	}
	readable := os.NewFile(uintptr(fd), "qoder-held-empty-telemetry-membership")
	defer func() { err = errors.Join(err, readable.Close()) }()
	before, err := readable.Stat()
	if err != nil || !projectNativeRuntimeStableStat(expected.Sys(), before.Sys()) {
		return qoderMetadataRefused()
	}
	count := 0
	if child != "" {
		count = 1
	}
	names, err := readable.Readdirnames(count + 1)
	if (err != nil && !errors.Is(err, io.EOF)) || len(names) != count || count == 1 && names[0] != child {
		return qoderMetadataRefused()
	}
	extra, err := readable.Readdirnames(1)
	after, statErr := readable.Stat()
	if len(extra) != 0 || !errors.Is(err, io.EOF) || statErr != nil || !projectNativeRuntimeStableStat(expected.Sys(), after.Sys()) {
		return qoderMetadataRefused()
	}
	return nil
}

func (r *qoderEmptyTelemetry) check() (err error) {
	profile, parents, err := qoderTelemetryRoot(r.ctx, r.root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, profile.Close()) }()
	info, err := profile.Stat()
	if err != nil || !qoderMetadataSame(r.profile, info, true) || !qoderTelemetrySameParents(r.parents, parents) {
		return qoderMetadataRefused()
	}
	edges := []qoderLogEdge{}
	defer func() {
		for i := len(edges) - 1; i >= 0; i-- {
			err = errors.Join(err, edges[i].file.Close())
		}
	}()
	parent := profile
	for i, name := range qoderTelemetryNames {
		if r.ctx.Err() != nil {
			return qoderMetadataRefused()
		}
		directory, openErr := qoderLogOpenAt(parent, name, true)
		if openErr != nil {
			return openErr
		}
		current, statErr := directory.Stat()
		edges = append(edges, qoderLogEdge{parent: parent, file: directory, name: name, info: current})
		if statErr != nil || !qoderMetadataSame(r.directories[i], current, true) || current.Mode() != os.ModeDir|0700 {
			return qoderMetadataRefused()
		}
		child := ""
		if i+1 < len(qoderTelemetryNames) {
			child = qoderTelemetryNames[i+1]
		}
		if err := qoderTelemetryMembers(directory, current, child); err != nil {
			return err
		}
		parent = directory
	}
	if err := qoderLogCheckEdges(edges); err != nil {
		return err
	}
	current, again, err := qoderTelemetryRoot(r.ctx, r.root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, current.Close()) }()
	linked, err := current.Stat()
	if err != nil || !projectNativeRuntimeStableStat(info.Sys(), linked.Sys()) || !qoderTelemetrySameParents(parents, again) {
		return qoderMetadataRefused()
	}
	return nil
}
