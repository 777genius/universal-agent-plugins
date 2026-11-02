//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

func sameFile(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime()) {
		return false
	}
	aa, ok := a.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	bb, ok := b.Sys().(*syscall.Stat_t)
	return ok && aa.Nlink == bb.Nlink && aa.Ctim == bb.Ctim && aa.Uid == bb.Uid && aa.Gid == bb.Gid
}
func statUID(info os.FileInfo) (uint32, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("linux stat authority missing")
	}
	return st.Uid, nil
}
func regular(info os.FileInfo, limit int64) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode().IsRegular() && st.Nlink == 1 && info.Size() >= 0 && info.Size() <= limit
}
func anchoredInfo(path string) (os.FileInfo, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot("/")
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	relative := strings.TrimPrefix(absolute, "/")
	if relative == "" {
		relative = "."
	}
	return root.Lstat(relative)
}
func exactDirectory(path string, owner int) (os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("exact absolute directory required")
	}
	current := "/"
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		current = filepath.Join(current, part)
		info, err := anchoredInfo(current)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("directory ancestor is not ordinary")
		}
	}
	info, err := anchoredInfo(path)
	if err != nil {
		return nil, err
	}
	uid, err := statUID(info)
	if err != nil || int64(uid) != int64(owner) {
		return nil, fmt.Errorf("directory owner differs")
	}
	return info, nil
}
func openOrdinary(path string, limit int64) (*os.File, os.FileInfo, error) {
	before, err := anchoredInfo(path)
	if err != nil {
		return nil, nil, err
	}
	if !regular(before, limit) {
		return nil, nil, fmt.Errorf("ordinary single-link bounded file required")
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	opened, err := f.Stat()
	if err != nil || !sameFile(before, opened) {
		_ = f.Close()
		return nil, nil, fmt.Errorf("opened file identity differs")
	}
	return f, before, nil
}
func readOrdinary(path string, limit int64) ([]byte, error) {
	f, before, err := openOrdinary(path, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	body, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	after, e := f.Stat()
	current, pe := anchoredInfo(path)
	if e != nil || pe != nil || int64(len(body)) > limit || int64(len(body)) != before.Size() || !sameFile(before, after) || !sameFile(before, current) {
		return nil, fmt.Errorf("bounded file changed while reading")
	}
	return body, nil
}
func hashFile(path string, limit int64) (string, error) {
	f, before, err := openOrdinary(path, limit)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, limit+1))
	if err != nil {
		return "", err
	}
	after, e := f.Stat()
	current, pe := anchoredInfo(path)
	if e != nil || pe != nil || n != before.Size() || n > limit || !sameFile(before, after) || !sameFile(before, current) {
		return "", fmt.Errorf("hashed file changed")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func copyOrdinary(ctx context.Context, source, target string, uid int, executable bool) (string, error) {
	f, before, err := openOrdinary(source, 512<<20)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	owner, err := statUID(before)
	if err != nil || int64(owner) != int64(uid) {
		return "", fmt.Errorf("input file owner differs")
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	total, digest, err := copyBounded(ctx, f, out, before.Size())
	closeErr := out.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	after, e := f.Stat()
	current, pe := anchoredInfo(source)
	if e != nil || pe != nil || total != before.Size() || !sameFile(before, after) || !sameFile(before, current) {
		return "", fmt.Errorf("copied input changed")
	}
	mode := os.FileMode(0444)
	if executable {
		var head [4]byte
		if _, err = f.ReadAt(head[:], 0); err != nil || string(head[:]) != "\x7fELF" {
			return "", fmt.Errorf("allowlisted executable is not ELF")
		}
		mode = 0555
	}
	if err = os.Chmod(target, mode); err != nil {
		return "", err
	}
	return digest, nil
}
func copyBounded(ctx context.Context, f, out *os.File, limit int64) (int64, string, error) {
	h := sha256.New()
	var err error
	buf := make([]byte, 64<<10)
	var total int64
	for {
		if err = ctx.Err(); err != nil {
			break
		}
		var n int
		n, err = f.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > limit {
				err = fmt.Errorf("input file grew")
				break
			}
			if _, err = out.Write(buf[:n]); err != nil {
				break
			}
			h.Write(buf[:n])
		}
		if errors.Is(err, io.EOF) {
			err = nil
			break
		}
		if err != nil {
			break
		}
	}
	return total, hex.EncodeToString(h.Sum(nil)), err
}
func copyPublisher(ctx context.Context, source, target string, uid int) (map[string]string, error) {
	if _, err := exactDirectory(source, uid); err != nil {
		return nil, err
	}
	hashes := map[string]string{}
	entries := 0
	var bytes int64
	root, err := os.OpenRoot(source)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	err = fs.WalkDir(root.FS(), ".", func(rel string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > 100000 {
			return fmt.Errorf("publisher entry bound")
		}
		if strings.Count(rel, string(os.PathSeparator)) > 32 {
			return fmt.Errorf("publisher path depth")
		}
		path := filepath.Join(source, rel)
		info, err := anchoredInfo(path)
		if err != nil {
			return err
		}
		owner, err := statUID(info)
		if err != nil || int64(owner) != int64(uid) {
			return fmt.Errorf("publisher input owner differs")
		}
		if info.IsDir() {
			return os.Mkdir(filepath.Join(target, rel), 0755)
		}
		if !regular(info, 512<<20) {
			return fmt.Errorf("publisher input not ordinary")
		}
		bytes += info.Size()
		if bytes > 2<<30 {
			return fmt.Errorf("publisher byte bound")
		}
		return copyPublisherEntry(ctx, path, filepath.Join(target, rel), rel, uid, hashes)
	})
	if err != nil {
		return nil, err
	}
	if len(hashes) != len(requiredPublisher) {
		return nil, fmt.Errorf("allowlisted publisher resource missing")
	}
	return hashes, nil
}
func copyPublisherEntry(ctx context.Context, source, target, relative string, uid int, hashes map[string]string) error {
	digest, err := copyOrdinary(ctx, source, target, uid, relative == "code" || relative == rgExecutable)
	if err != nil {
		return err
	}
	for _, required := range requiredPublisher {
		if relative == required {
			hashes[relative] = digest
		}
	}
	return nil
}
func snapshot(paths []string) (map[string]string, error) {
	result := map[string]string{}
	for _, root := range paths {
		if err := snapshotSource(root, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func snapshotSource(root string, result map[string]string) error {
	if _, err := exactDirectory(filepath.Dir(root), 0); err != nil {
		return err
	}
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if err := snapshotCapacity(result); err != nil {
			return err
		}
		info, err := anchoredInfo(path)
		if err != nil {
			return err
		}
		uid, err := statUID(info)
		if err != nil || uid != 0 || info.Mode().Perm()&022 != 0 {
			return fmt.Errorf("sealed source writable or foreign")
		}
		if info.IsDir() {
			result[path] = fmt.Sprintf("directory:%o", info.Mode())
			return nil
		}
		if !regular(info, 1<<20) {
			return fmt.Errorf("sealed source is not ordinary")
		}
		digest, err := hashFile(path, 1<<20)
		if err != nil {
			return err
		}
		result[path] = fmt.Sprintf("%o:%d:%s", info.Mode(), info.Size(), digest)
		return nil
	})
	if err != nil {
		return err
	}
	_, err = exactDirectory(filepath.Dir(root), 0)
	return err
}
func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func uniqueJSON(body []byte) (any, error) {
	if !utf8.Valid(body) {
		return nil, fmt.Errorf("invalid UTF-8 JSON")
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	value, err := readJSONValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err = dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("trailing JSON data")
	}
	return value, nil
}
func readJSONValue(dec *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, fmt.Errorf("JSON nesting bound")
	}
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delimiter {
	case '{':
		return readJSONObject(dec, depth)
	case '[':
		return readJSONArray(dec, depth)
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter")
	}
}
func readJSONObject(dec *json.Decoder, depth int) (any, error) {
	result := map[string]any{}
	seen := map[string]bool{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		fold := strings.ToLower(name)
		if !ok || seen[fold] {
			return nil, fmt.Errorf("duplicate/ambiguous JSON key")
		}
		seen[fold] = true
		value, err := readJSONValue(dec, depth+1)
		if err != nil {
			return nil, err
		}
		result[name] = value
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') {
		return nil, fmt.Errorf("invalid object end")
	}
	return result, nil
}
func readJSONArray(dec *json.Decoder, depth int) (any, error) {
	result := []any{}
	for dec.More() {
		if len(result) >= 256 {
			return nil, fmt.Errorf("JSON array bound")
		}
		value, err := readJSONValue(dec, depth+1)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if end, err := dec.Token(); err != nil || end != json.Delim(']') {
		return nil, fmt.Errorf("invalid array end")
	}
	return result, nil
}
func immutableDirectory(info os.FileInfo, owner uint32, stickyException bool) bool {
	uid, err := statUID(info)
	if err != nil || uid != owner || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if info.Mode().Perm()&0022 == 0 {
		return true
	}
	return stickyException && info.Mode()&os.ModeSticky != 0
}
func immutableAncestors(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("immutable ancestor path is not exact")
	}
	current := "/"
	info, err := anchoredInfo(current)
	if err != nil || !immutableDirectory(info, 0, false) {
		return fmt.Errorf("root ancestor authority differs")
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err = anchoredInfo(current)
		if err != nil || !immutableDirectory(info, 0, current == "/tmp") {
			return fmt.Errorf("immutable executable ancestor refused: %s", current)
		}
	}
	return nil
}

func snapshotCapacity(result map[string]string) error {
	if len(result) >= 256 {
		return fmt.Errorf("sealed source entry bound")
	}
	return nil
}
