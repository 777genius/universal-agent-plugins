//go:build linux

package pluginkitairepo_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type geminiCycleFileFrame struct {
	Record  geminiCycleRecord
	Parents map[string]geminiCycleRecord
}

func geminiCycleFile(ctx context.Context, path string) (frame geminiCycleFileFrame, err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || ctx.Err() != nil {
		return frame, fmt.Errorf("Gemini original file canonical authority/deadline")
	}
	edges, err := geminiCycleOpen(filepath.Dir(path))
	if err != nil {
		return frame, err
	}
	defer func() {
		if e := geminiCycleClose(edges); e != nil {
			frame = geminiCycleFileFrame{}
			err = fmt.Errorf("Gemini original file parent checked close")
		}
	}()
	parent := edges[len(edges)-1].file
	fd, err := unix.Openat(int(parent.Fd()), filepath.Base(path), unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return frame, fmt.Errorf("Gemini original file no-follow open")
	}
	file := os.NewFile(uintptr(fd), "original-authority-file")
	defer func() {
		if e := file.Close(); e != nil {
			frame = geminiCycleFileFrame{}
			err = fmt.Errorf("Gemini original file checked close")
		}
	}()
	before, err := geminiCycleStat(file)
	if err != nil || before.Mode&unix.S_IFMT != unix.S_IFREG || before.Links != 1 || before.UID != uint32(os.Getuid()) || before.GID != uint32(os.Getgid()) || before.Size < 0 || before.Size > 1<<20 {
		return frame, fmt.Errorf("Gemini original held ordinary owned file/bound")
	}
	body, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil || int64(len(body)) != before.Size {
		return frame, fmt.Errorf("Gemini original file whole bounded body")
	}
	after, err := geminiCycleStat(file)
	var linked unix.Stat_t
	if err != nil || before != after || unix.Fstatat(int(parent.Fd()), filepath.Base(path), &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || geminiCycleRecordOf(linked) != before || geminiCycleEdges(edges) != nil || ctx.Err() != nil {
		return frame, fmt.Errorf("Gemini original file/parent identity changed")
	}
	before.SHA256 = geminiCycleSHA(body)
	frame.Record = before
	frame.Parents = map[string]geminiCycleRecord{}
	current := "/"
	for index, edge := range edges {
		if index > 0 {
			current = filepath.Join(current, edge.name)
		}
		frame.Parents[current] = edge.initial
	}
	return frame, nil
}
func geminiCycleFileParents(before, after geminiCycleFileFrame) bool {
	if len(before.Parents) == 0 || len(before.Parents) != len(after.Parents) {
		return false
	}
	for path, record := range before.Parents {
		current, ok := after.Parents[path]
		if !ok || !geminiCycleSameDirectory(record, current) {
			return false
		}
	}
	return true
}
func (c *geminiProjectCycle) originalFiles() error {
	for path, want := range c.fileFrames {
		if c.open[path] {
			continue
		}
		actual, err := geminiCycleFile(c.f.ctx, path)
		if err != nil || want.Record != actual.Record || !geminiCycleFileParents(want, actual) {
			return fmt.Errorf("Gemini original file authority metadata/body changed")
		}
	}
	return nil
}
