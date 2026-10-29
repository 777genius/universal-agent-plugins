//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// Pinned local-config loaders skip their exclude writer on these exact lines.
// Prepare only the newly initialized fixture Git file, before its first freeze.
func windGitExcludePrepare(directory string, want windProfileRecord) (record windProfileRecord, err error) {
	edges, err := windOpenDirectory(directory)
	if err != nil {
		return record, fmt.Errorf("Git exclude parent refused")
	}
	defer func() {
		if windCloseEdges(edges) != nil {
			err = fmt.Errorf("Git exclude parent close")
		}
	}()
	parent := int(edges[len(edges)-1].file.Fd())
	var before unix.Stat_t
	if unix.Fstatat(parent, "exclude", &before, unix.AT_SYMLINK_NOFOLLOW) != nil || !windOwned(before) || before.Mode != unix.S_IFREG|0644 || before.Nlink != 1 || before.Size < 0 || before.Size > 64<<20 {
		return record, fmt.Errorf("Git exclude authority refused")
	}
	fd, err := unix.Openat(parent, "exclude", unix.O_RDWR|unix.O_APPEND|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return record, fmt.Errorf("Git exclude open refused")
	}
	file := os.NewFile(uintptr(fd), "held-Git-exclude")
	defer func() {
		if file.Close() != nil {
			err = fmt.Errorf("Git exclude close")
		}
	}()
	opened, err := windFileStat(file)
	if err != nil || !windSameStat(before, opened) {
		return record, fmt.Errorf("Git exclude opened authority changed")
	}
	raw, err := io.ReadAll(io.LimitReader(file, (64<<20)+1))
	stable, statErr := windFileStat(file)
	var linked unix.Stat_t
	if err != nil || statErr != nil || !windSameStat(before, stable) || unix.Fstatat(parent, "exclude", &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || !windSameStat(before, linked) || windRecord(before, "file", windsurfConfigDigest(raw)) != want || int64(len(raw)) != before.Size || windCheckEdges(edges) != nil {
		return record, fmt.Errorf("Git exclude initial bytes/authority changed")
	}
	record, err = windGitExcludeAppend(file, before, raw)
	if err != nil {
		return record, err
	}
	if unix.Fstatat(parent, "exclude", &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || windRecord(linked, "file", record.SHA256) != record || windCheckEdges(edges) != nil {
		return record, fmt.Errorf("Git exclude linked authority changed")
	}
	return record, nil
}

func windGitExcludeAppend(file *os.File, before unix.Stat_t, original []byte) (windProfileRecord, error) {
	body := append([]byte{}, original...)
	for _, rule := range []string{".devin/config.local.json", ".devin/mcp_config.local.json"} {
		present := false
		for _, line := range strings.Split(string(original), "\n") {
			present = present || strings.TrimSuffix(line, "\r") == rule
		}
		if !present {
			if len(body) > 0 && body[len(body)-1] != '\n' {
				body = append(body, '\n')
			}
			body = append(body, []byte(rule+"\n")...)
		}
	}
	if len(body) > 64<<20 {
		return windProfileRecord{}, fmt.Errorf("Git exclude byte bound")
	}
	if len(body) != len(original) {
		if n, err := file.Write(body[len(original):]); err != nil || n != len(body)-len(original) {
			return windProfileRecord{}, fmt.Errorf("Git exclude append failed")
		}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return windProfileRecord{}, fmt.Errorf("Git exclude rewind failed")
	}
	written, err := windFileStat(file)
	if err != nil {
		return windProfileRecord{}, fmt.Errorf("Git exclude authored stat failed")
	}
	raw, err := io.ReadAll(io.LimitReader(file, (64<<20)+1))
	after, statErr := windFileStat(file)
	if err != nil || statErr != nil || !windSameStat(written, after) || !bytes.Equal(body, raw) || !windDirectoryIdentity(before, after) || after.Nlink != 1 || after.Size != int64(len(body)) {
		return windProfileRecord{}, fmt.Errorf("Git exclude authored bytes/authority changed")
	}
	if len(body) == len(original) && !windSameStat(before, after) {
		return windProfileRecord{}, fmt.Errorf("Git exclude no-write metadata changed")
	}
	return windRecord(after, "file", windsurfConfigDigest(raw)), nil
}
