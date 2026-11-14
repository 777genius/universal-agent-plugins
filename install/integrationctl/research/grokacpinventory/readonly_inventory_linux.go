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
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const readonlyMode = "readonly-inventory-v1"

var readonlyRoots = []string{".", "home", "home/.grok", "user", "a", "b", "tmp", "xdg-config", "xdg-data", "xdg-cache", "xdg-state", "readonly-witness"}
var readonlyFiles = []string{"home/.grok/config.toml", "startup-witness.py"}
var readonlyNamespaces = []string{"user", "pid", "net", "ipc", "uts", "mnt"}
var readonlyDevices = []string{"null", "zero", "full", "random", "urandom", "tty"}

type readonlyObject struct {
	Path     string  `json:"path"`
	Identity []int64 `json:"identity"`
	SHA      string  `json:"sha256,omitempty"`
}
type readonlyGrant struct {
	Roots       map[string]readonlyObject `json:"roots"`
	Files       map[string]readonlyObject `json:"files"`
	Runtime     map[string]readonlyObject `json:"runtime"`
	Devices     map[string]readonlyObject `json:"devices"`
	Namespaces  map[string]uint64         `json:"namespaces"`
	OverflowUID int                       `json:"overflow_uid"`
	OverflowGID int                       `json:"overflow_gid"`
	InputSHA    string                    `json:"input_sha256"`
	InputCount  int                       `json:"input_count"`
}

// Existing deadlines retain their original semantics for all earlier modes.
func remaining(start, now int64, limit time.Duration) (time.Duration, error) {
	if start <= 0 || now < start || now-start >= int64(limit) {
		return 0, errors.New("original-deadline-refused")
	}
	return limit - time.Duration(now-start), nil
}
func finalBudget(parent context.Context, start int64) (context.Context, context.CancelFunc, error) {
	now, e := monoNow()
	if e != nil {
		return nil, nil, e
	}
	budget, e := remaining(start, now, 35*time.Second)
	if e != nil {
		return nil, nil, e
	}
	ctx, cancel := context.WithTimeout(parent, budget)
	return ctx, cancel, nil
}

func researchActiveLimit(mode string) time.Duration {
	if mode == readonlyMode {
		return 25 * time.Second
	}
	return 30 * time.Second
}
func researchFinalBudget(parent context.Context, r request) (context.Context, context.CancelFunc, error) {
	if r.Mode != readonlyMode {
		return finalBudget(parent, r.Start)
	}
	now, e := monoNow()
	if e != nil {
		return nil, nil, e
	}
	budget, e := remaining(r.Start, now, 30*time.Second)
	if e != nil {
		return nil, nil, e
	}
	ctx, cancel := context.WithTimeout(parent, budget)
	return ctx, cancel, nil
}
func readonlyFinalCheck(r request) error {
	if r.Mode != readonlyMode {
		return nil
	}
	now, e := monoNow()
	if e != nil {
		return e
	}
	_, e = remaining(r.Start, now, 30*time.Second)
	return e
}
func researchSystemNames(mode string) []string {
	names := []string{"unshare", "setpriv", "env", "python3"}
	if mode == readonlyMode {
		names = append(names, "bwrap")
	}
	return names
}
func (s *scopeState) prepareReadonlyExecutable(ctx context.Context, r request) error {
	if r.Mode != readonlyMode {
		return nil
	}
	h, e := systemFile(ctx, "/usr/bin/bwrap")
	if e != nil {
		return e
	}
	s.held = append(s.held, h)
	if h.path != "/usr/bin/bwrap" || h.id.Mode&(unix.S_ISUID|unix.S_ISGID) != 0 {
		return errors.New("ordinary-bwrap-refused")
	}
	if _, e = unix.Fgetxattr(int(h.file.Fd()), "security.capability", nil); !errors.Is(e, unix.ENODATA) {
		return errors.New("bwrap-capability-refused")
	}
	s.systems["bwrap"] = systemFact{h.path, h.digest}
	return h.unchanged()
}
func readonlyIdentity(info os.FileInfo, directory bool) ([]int64, error) {
	id, e := identity(info)
	if e != nil || id.Dev > 1<<63-1 || id.Ino > 1<<63-1 || id.Links > 1<<63-1 {
		return nil, errors.New("readonly-identity-refused")
	}
	if directory {
		if !info.IsDir() {
			return nil, errors.New("readonly-directory-refused")
		}
		return []int64{int64(id.Dev), int64(id.Ino), int64(id.Mode), int64(id.UID), int64(id.GID)}, nil
	}
	return []int64{int64(id.Dev), int64(id.Ino), int64(id.Mode), int64(id.Links), int64(id.UID), int64(id.GID), id.Size, id.Mtime.Nano(), id.Ctime.Nano()}, nil
}
func readonlyDirectory(path string) (readonlyObject, error) {
	before, e := anchoredInfo(path)
	if e != nil {
		return readonlyObject{}, e
	}
	root, e := os.OpenRoot(path)
	if e != nil {
		return readonlyObject{}, e
	}
	defer func() { _ = root.Close() }()
	held, he := root.Stat(".")
	after, ae := anchoredInfo(path)
	if he != nil || ae != nil || !os.SameFile(before, held) || !os.SameFile(before, after) {
		return readonlyObject{}, errors.New("readonly-directory-join-refused")
	}
	id, e := readonlyIdentity(held, true)
	return readonlyObject{Path: path, Identity: id}, e
}
func readonlyOverflow(path string) (int, error) {
	file, e := os.Open(path)
	if e != nil {
		return 0, e
	}
	body, e := io.ReadAll(io.LimitReader(file, 33))
	e = errors.Join(e, file.Close())
	if e != nil || len(body) == 0 || len(body) > 32 {
		return 0, errors.New("overflow-id-refused")
	}
	value, e := strconv.ParseUint(strings.TrimSpace(string(body)), 10, 32)
	if e != nil || value == 0 || value > math.MaxInt32 || strconv.FormatUint(value, 10)+"\n" != string(body) {
		return 0, errors.New("overflow-id-refused")
	}
	return int(value), nil
}
func readonlyInputInfo(root *os.Root, path string, uid, gid int) (os.FileInfo, fileID, error) {
	before, e := root.Lstat(path)
	if e != nil {
		return nil, fileID{}, e
	}
	id, e := identity(before)
	if e != nil || int64(id.UID) != int64(uid) || int64(id.GID) != int64(gid) || before.Mode().Perm()&022 != 0 ||
		!before.IsDir() && (!before.Mode().IsRegular() || id.Links != 1 || id.Size < 0 || id.Size > 512<<20) {
		return nil, fileID{}, errors.New("readonly-input-authority")
	}
	return before, id, nil
}
func readonlyInputChildren(ctx context.Context, root *os.Root, path string, file *os.File, uid, gid int, rows map[string][]int64) error {
	var e error
	for {
		if e = ctx.Err(); e != nil {
			return e
		}
		entries, readErr := file.ReadDir(1)
		if readErr == io.EOF {
			break
		}
		if readErr != nil || len(entries) != 1 {
			return errors.New("readonly-input-directory-read")
		}
		if e = readonlyInputRows(ctx, root, filepath.Join(path, entries[0].Name()), uid, gid, rows); e != nil {
			return e
		}
	}
	return nil
}
func readonlyInputRows(ctx context.Context, root *os.Root, path string, uid, gid int, rows map[string][]int64) (resultErr error) {
	if e := ctx.Err(); e != nil {
		return e
	}
	if len(rows) >= 4096 || len(path) > 4096 || strings.Count(path, "/") > 32 || strings.ContainsAny(path, "\x00\r\n") {
		return errors.New("readonly-input-bound")
	}
	before, id, e := readonlyInputInfo(root, path, uid, gid)
	if e != nil {
		return e
	}
	rows[path], e = readonlyIdentity(before, before.IsDir())
	if e != nil || !before.IsDir() {
		return e
	}
	file, e := root.Open(path)
	if e != nil {
		return e
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	held, e := file.Stat()
	if e != nil || !os.SameFile(before, held) {
		return errors.New("readonly-input-directory-open")
	}
	if e = readonlyInputChildren(ctx, root, path, file, uid, gid, rows); e != nil {
		return e
	}
	after, e := root.Lstat(path)
	current, he := file.Stat()
	if e != nil || he != nil {
		return errors.New("readonly-input-directory-final")
	}
	afterID, ae := identity(after)
	heldID, he := identity(current)
	if ae != nil || he != nil || afterID != id || heldID != id {
		return errors.New("readonly-input-directory-changed")
	}
	return nil
}
func readonlyInput(ctx context.Context, root *os.Root, uid, gid int) (string, int, error) {
	rows := make(map[string][]int64)
	if e := readonlyInputRows(ctx, root, ".", uid, gid, rows); e != nil {
		return "", 0, e
	}
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	hash := sha256.New()
	for _, key := range keys {
		if e := ctx.Err(); e != nil {
			return "", 0, e
		}
		_, _ = fmt.Fprint(hash, key, "\x00")
		for index, value := range rows[key] {
			if index > 0 {
				_, _ = fmt.Fprint(hash, ",")
			}
			_, _ = fmt.Fprint(hash, value)
		}
		_, _ = fmt.Fprint(hash, "\n")
	}
	return hex.EncodeToString(hash.Sum(nil)), len(rows), nil
}

func (s *scopeState) prepareReadonly(ctx context.Context, root string, r request) (*readonlyGrant, error) {
	if r.Mode != readonlyMode {
		return nil, nil
	}
	if e := s.directory.seal(); e != nil {
		return nil, e
	}
	grant := &readonlyGrant{Roots: map[string]readonlyObject{}, Files: map[string]readonlyObject{},
		Runtime: map[string]readonlyObject{}, Devices: map[string]readonlyObject{}, Namespaces: map[string]uint64{}}
	for _, name := range readonlyRoots {
		value, e := readonlyDirectory(filepath.Join(root, name))
		if e != nil || value.Identity[3] != int64(r.UID) || value.Identity[4] != int64(r.GID) {
			return nil, errors.New("readonly-root-owner")
		}
		grant.Roots[name] = value
	}
	sealed, e := readonlyDirectory(s.sealed)
	if e != nil {
		return nil, e
	}
	grant.Roots["sealed"] = sealed
	if e = s.readonlyFiles(ctx, root, r, grant); e != nil {
		return nil, e
	}
	if e = readonlyRuntimeFacts(grant); e != nil {
		return nil, e
	}
	grant.OverflowUID, e = readonlyOverflow("/proc/sys/kernel/overflowuid")
	if e != nil {
		return nil, e
	}
	grant.OverflowGID, e = readonlyOverflow("/proc/sys/kernel/overflowgid")
	if e != nil || grant.OverflowUID == r.UID || grant.OverflowGID == r.GID {
		return nil, errors.New("readonly-overflow-ambiguity")
	}
	held, e := os.OpenRoot(root)
	if e != nil {
		return nil, e
	}
	grant.InputSHA, grant.InputCount, e = readonlyInput(ctx, held, r.UID, r.GID)
	e = errors.Join(e, held.Close(), s.input.verify())
	if e != nil || len(grant.Files) != 6 {
		return nil, errors.New("readonly-input-proof")
	}
	s.readonly = grant
	return grant, nil
}
func (s *scopeState) readonlyFiles(ctx context.Context, root string, r request, grant *readonlyGrant) error {
	for _, name := range readonlyFiles {
		h, e := openHeld(filepath.Join(root, name), r.UID, 16<<10)
		if e != nil {
			return e
		}
		s.held = append(s.held, h)
		h.digest, e = digestFile(ctx, h)
		s.held[len(s.held)-1] = h
		if e != nil {
			return e
		}
	}
	for _, h := range s.held {
		if h.path != filepath.Join(s.sealed, "grok") && h.path != filepath.Join(s.sealed, "collector.py") &&
			h.path != filepath.Join(root, readonlyFiles[0]) && h.path != filepath.Join(root, readonlyFiles[1]) &&
			h.path != s.systems["bwrap"].Path && h.path != s.systems["python3"].Path {
			continue
		}
		info, e := h.file.Stat()
		if e != nil {
			return e
		}
		id, e := readonlyIdentity(info, false)
		if e != nil || h.digest == "" {
			return errors.New("readonly-file-proof")
		}
		if _, capErr := unix.Fgetxattr(int(h.file.Fd()), "security.capability", nil); !errors.Is(capErr, unix.ENODATA) || h.id.Mode&(unix.S_ISUID|unix.S_ISGID) != 0 {
			return errors.New("readonly-file-capability-refused")
		}
		grant.Files[h.path] = readonlyObject{h.path, id, h.digest}
	}
	return nil
}

func readonlyRuntimeFacts(grant *readonlyGrant) error {
	for _, path := range []string{"/usr", "/bin", "/lib", "/lib64"} {
		canonical, e := filepath.EvalSymlinks(path)
		if e != nil || canonical != "/usr" && !strings.HasPrefix(canonical, "/usr/") {
			return errors.New("readonly-runtime-root")
		}
		if e = immutable(canonical); e != nil {
			return e
		}
		value, e := readonlyDirectory(canonical)
		if e != nil {
			return e
		}
		grant.Runtime[path] = value
	}
	for _, name := range readonlyDevices {
		path := filepath.Join("/dev", name)
		info, e := anchoredInfo(path)
		if e != nil || info.Mode()&os.ModeCharDevice == 0 {
			return errors.New("readonly-device")
		}
		id, e := readonlyIdentity(info, false)
		if e != nil {
			return e
		}
		grant.Devices[name] = readonlyObject{Path: path, Identity: id}
	}
	for _, name := range readonlyNamespaces {
		info, e := os.Stat("/proc/self/ns/" + name)
		if e != nil {
			return e
		}
		id, e := identity(info)
		if e != nil || id.Ino == 0 {
			return errors.New("readonly-parent-namespace")
		}
		grant.Namespaces[name] = id.Ino
	}
	return nil
}

func validateReadonlyResult(body []byte, got result, r request) error {
	if r.Mode != readonlyMode {
		return nil
	}
	if got.Mode != readonlyMode || got.Provider != nil || got.LockHeld != nil || got.Phase != "" ||
		len(got.Commands) > 2 || got.Complete && len(got.Commands) != 2 {
		return errors.New("readonly-result-refused")
	}
	var envelope struct {
		Proof    json.RawMessage `json:"readonly"`
		Complete *bool           `json:"semantic_complete"`
	}
	if e := json.Unmarshal(body, &envelope); e != nil || envelope.Complete == nil {
		return errors.New("readonly-proof-type-refused")
	}
	var proof struct {
		Authority json.RawMessage `json:"authority"`
		Stage     string          `json:"stage"`
		Control   bool            `json:"control_complete"`
		Original  json.RawMessage `json:"original"`
		Marker    json.RawMessage `json:"control_marker"`
	}
	if e := closed(envelope.Proof, &proof, "authority stage control_complete original control_marker"); e != nil {
		return e
	}
	if proof.Stage != "isolation" && proof.Stage != "runtime" && proof.Stage != "control" && proof.Stage != "native" && proof.Stage != "complete" {
		return errors.New("readonly-stage-refused")
	}
	if got.Complete && (!proof.Control || proof.Stage != "complete") {
		return errors.New("readonly-completion-refused")
	}
	return nil
}
func (s *scopeState) verifyReadonlyResult(raw []byte, r request) error {
	if r.Mode != readonlyMode {
		return nil
	}
	if s.readonly == nil {
		return errors.New("readonly-original-authority-missing")
	}
	var got struct {
		Readonly struct {
			Authority readonlyGrant `json:"authority"`
		} `json:"readonly"`
	}
	if e := json.Unmarshal(raw, &got); e != nil || !reflect.DeepEqual(got.Readonly.Authority, *s.readonly) {
		return errors.New("readonly-original-authority-changed")
	}
	return readonlyFinalCheck(r)
}
