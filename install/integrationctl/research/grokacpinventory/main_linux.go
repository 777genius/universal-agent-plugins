//go:build linux

// Fixed research controller; it provides cleanup authority, never qualification.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var sourceCommit, sourceTree, collectorSHA string
var hex40 = regexp.MustCompile(`^[0-9a-f]{40}$`)
var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)
var noncePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

const nativeSHA = "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9"
const resultLimit = 2 << 20

type request struct {
	Schema    int    `json:"schema_version"`
	Commit    string `json:"source_commit"`
	Tree      string `json:"source_tree"`
	UID       int    `json:"uid"`
	GID       int    `json:"gid"`
	Nonce     string `json:"nonce"`
	Scope     string `json:"scope"`
	Start     int64  `json:"start_monotonic_ns"`
	HelperSHA string `json:"helper_sha256"`
}
type collectorRequest struct {
	Schema   int    `json:"schema_version"`
	Commit   string `json:"source_commit"`
	Tree     string `json:"source_tree"`
	UID      int    `json:"uid"`
	GID      int    `json:"gid"`
	Nonce    string `json:"nonce"`
	Scope    string `json:"scope"`
	Root     string `json:"input_root"`
	ParentNS uint64 `json:"parent_network_namespace"`
	Start    int64  `json:"start_monotonic_ns"`
}
type isolation struct {
	UID        int      `json:"uid"`
	GID        int      `json:"gid"`
	EUID       int      `json:"euid"`
	EGID       int      `json:"egid"`
	Groups     []int    `json:"groups"`
	Network    uint64   `json:"network_namespace"`
	Parent     uint64   `json:"parent_network_namespace"`
	Interfaces []string `json:"interfaces"`
	Caps       bool     `json:"capabilities_zero"`
	NNP        int      `json:"no_new_privileges"`
}
type result struct {
	Schema    int               `json:"schema_version"`
	Commit    string            `json:"source_commit"`
	Tree      string            `json:"source_tree"`
	Scope     string            `json:"scope"`
	Nonce     string            `json:"nonce"`
	Complete  bool              `json:"semantic_complete"`
	Reason    string            `json:"reason"`
	Isolation json.RawMessage   `json:"isolation"`
	Commands  []json.RawMessage `json:"commands"`
	RPC       json.RawMessage   `json:"rpc"`
}
type systemFact struct {
	Path string `json:"path"`
	SHA  string `json:"sha256"`
}
type receipt struct {
	Schema      int                   `json:"schema_version"`
	Commit      string                `json:"source_commit"`
	Tree        string                `json:"source_tree"`
	Scope       string                `json:"scope"`
	Nonce       string                `json:"nonce"`
	Clean       bool                  `json:"cleanup_complete"`
	SealedClean bool                  `json:"sealed_cleanup_complete"`
	Result      json.RawMessage       `json:"result"`
	Systems     map[string]systemFact `json:"system_executables"`
}

func monoNow() (int64, error) {
	var t unix.Timespec
	e := unix.ClockGettime(unix.CLOCK_MONOTONIC, &t)
	return t.Nano(), e
}
func remaining(start, now int64, limit time.Duration) (time.Duration, error) {
	if start <= 0 || now < start || now-start >= int64(limit) {
		return 0, errors.New("original-deadline-refused")
	}
	return limit - time.Duration(now-start), nil
}
func validateRequest(r request, root string, uid, gid int) error {
	if r.Schema != 1 || r.Commit != sourceCommit || r.Tree != sourceTree || !hex40.MatchString(sourceCommit) || !hex40.MatchString(sourceTree) || !hex64.MatchString(collectorSHA) || !hex64.MatchString(r.HelperSHA) || !noncePattern.MatchString(r.Nonce) || r.UID != uid || r.GID != gid || uid <= 0 || gid <= 0 || uid > 1<<30 || gid > 1<<30 {
		return errors.New("request-identity-refused")
	}
	if r.Scope != "user" && r.Scope != "a" && r.Scope != "b" {
		return errors.New("request-scope-refused")
	}
	if root != "/tmp/uap-grok-acp-input-"+r.Nonce {
		return errors.New("request-root-refused")
	}
	return nil
}
func readRequest(root string, uid, gid int) (request, heldFile, error) {
	var r request
	h, e := openHeld(filepath.Join(root, "request.json"), uid, 16<<10)
	if e != nil {
		return r, h, e
	}
	body, e := io.ReadAll(io.LimitReader(h.file, 16<<10+1))
	if e == nil {
		e = h.unchanged()
	}
	if e == nil {
		e = closed(body, &r, "schema_version source_commit source_tree uid gid nonce scope start_monotonic_ns helper_sha256")
	}
	if e == nil {
		e = validateRequest(r, root, uid, gid)
	}
	return r, h, e
}
func validReason(reason string) bool {
	switch reason {
	case "complete", "protocol-refused", "help-refused", "isolation-refused", "collector-refused":
		return true
	}
	return false
}
func validateIsolation(body []byte, r request, parent uint64) error {
	var proof isolation
	if e := closed(body, &proof, "uid gid euid egid groups network_namespace parent_network_namespace interfaces capabilities_zero no_new_privileges"); e != nil {
		return e
	}
	if proof.UID != r.UID || proof.EUID != r.UID || proof.GID != r.GID || proof.EGID != r.GID || proof.Groups == nil || len(proof.Groups) != 0 || proof.Network == 0 || proof.Network == parent || proof.Parent != parent || !reflect.DeepEqual(proof.Interfaces, []string{"lo"}) || !proof.Caps || proof.NNP != 1 {
		return errors.New("result-isolation-refused")
	}
	return nil
}
func validateResult(body []byte, r request, parent uint64) error {
	var got result
	if len(body) > resultLimit {
		return errors.New("result-bound-refused")
	}
	if e := closed(body, &got, "schema_version source_commit source_tree scope nonce semantic_complete reason isolation commands rpc"); e != nil {
		return e
	}
	if got.Schema != 1 || got.Commit != r.Commit || got.Tree != r.Tree || got.Scope != r.Scope || got.Nonce != r.Nonce || len(got.Commands) > 5 {
		return errors.New("result-identity-refused")
	}
	if !validReason(got.Reason) || got.Complete != (got.Reason == "complete") {
		return errors.New("result-reason-refused")
	}
	if e := validateIsolation(got.Isolation, r, parent); e != nil {
		return e
	}
	var rpc map[string]json.RawMessage
	return closed(got.RPC, &rpc, "initialize plugins notifications")
}
func exchange(reader io.Reader, r request, parent uint64) ([]byte, error) {
	body, e := io.ReadAll(io.LimitReader(reader, resultLimit+1))
	if e != nil || len(body) > resultLimit || !bytes.HasSuffix(body, []byte{'\n'}) || bytes.Count(body, []byte{'\n'}) != 1 {
		return nil, errors.New("collector-frame-refused")
	}
	if e = validateResult(body, r, parent); e != nil {
		return nil, e
	}
	return body, nil
}
func watchParent(control io.Reader, cancel context.CancelFunc) {
	go func() { var one [1]byte; _, _ = control.Read(one[:]); cancel() }()
}
func supervisedCapture(ctx context.Context, command ports.Command, r request, parent uint64, run func(context.Context, ports.Command, func(io.Writer, io.Reader) error) error) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	var raw []byte
	e := run(ctx, command, func(_ io.Writer, reader io.Reader) error {
		var captureErr error
		raw, captureErr = exchange(reader, r, parent)
		return captureErr
	})
	if e != nil {
		return nil, errors.New("strict-cleanup-unproved")
	}
	if len(raw) == 0 {
		return nil, errors.New("strict-exchange-missing")
	}
	return raw, nil
}
func main() {
	if len(os.Args) != 3 || os.Args[1] != "--input-root" || runtime.GOARCH != "amd64" || os.Getuid() != 0 || os.Geteuid() != 0 || os.Getgid() != 0 {
		os.Exit(2)
	}
	if e := run(os.Args[2], os.Stdin, os.Stdout); e != nil {
		_, _ = fmt.Fprintln(os.Stderr, "grok-acp-controller-refused")
		os.Exit(2)
	}
}
func inputAuthority(root string, uid int) error {
	if e := safeTemporaryParent(); e != nil {
		return e
	}
	info, e := anchoredInfo(root)
	if e != nil {
		return e
	}
	id, e := identity(info)
	if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 || int64(id.UID) != int64(uid) || filepath.Dir(root) != "/tmp" {
		return errors.New("input-root-refused")
	}
	return nil
}
func run(root string, control io.Reader, output io.Writer) (resultErr error) {
	for key, want := range map[string]string{"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1", "EXPECTED_COMMIT": sourceCommit} {
		if os.Getenv(key) != want {
			return errors.New("host-admission-refused")
		}
	}
	uid, e := strconv.Atoi(os.Getenv("SUDO_UID"))
	gid, ge := strconv.Atoi(os.Getenv("SUDO_GID"))
	if e != nil || ge != nil {
		return errors.New("caller-refused")
	}
	if e = inputAuthority(root, uid); e != nil {
		return e
	}
	r, requestFile, e := readRequest(root, uid, gid)
	if requestFile.file != nil {
		defer func() { resultErr = errors.Join(resultErr, requestFile.file.Close()) }()
	}
	if e != nil {
		return e
	}
	now, e := monoNow()
	if e != nil {
		return e
	}
	budget, e := remaining(r.Start, now, 30*time.Second)
	if e != nil {
		return e
	}
	signalParent, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	parent, cancelParent := context.WithCancel(signalParent)
	defer cancelParent()
	watchParent(control, cancelParent)
	ctx, cancel := context.WithTimeout(parent, budget)
	defer cancel()
	return runScope(ctx, parent, root, r, requestFile, output)
}

type scopeState struct {
	held         []heldFile
	sealed       string
	parent       uint64
	safeToDelete bool
	directory    *directoryAuthority
	input        *directoryAuthority
	systems      map[string]systemFact
}

func (s *scopeState) close() error {
	var e error
	if s.safeToDelete && s.directory != nil {
		e = errors.Join(e, s.directory.remove())
	}
	for _, h := range s.held {
		e = errors.Join(e, h.file.Close())
	}
	if s.directory != nil {
		e = errors.Join(e, s.directory.file.Close())
	}
	if s.input != nil {
		e = errors.Join(e, s.input.file.Close())
	}
	return e
}
func (s *scopeState) prepareExecutables(ctx context.Context, r request) error {
	s.systems = map[string]systemFact{}
	for _, path := range []string{"/usr/bin/unshare", "/usr/bin/setpriv", "/usr/bin/env", "/usr/bin/python3"} {
		h, e := systemFile(ctx, path)
		if e != nil {
			return e
		}
		s.held = append(s.held, h)
		s.systems[filepath.Base(path)] = systemFact{h.path, h.digest}
	}
	self, e := os.Executable()
	if e != nil {
		return e
	}
	h, e := pinnedRuntime(ctx, self, r.HelperSHA, r.UID)
	if e != nil {
		return e
	}
	s.held = append(s.held, h)
	return (process.OS{}).DuplexCapability()
}
func (s *scopeState) prepareCopies(ctx context.Context, root string, r request) error {
	for _, item := range []struct {
		name, want string
		mode       os.FileMode
	}{{"grok", nativeSHA, 0555}, {"collector.py", collectorSHA, 0444}} {
		h, e := sealFile(ctx, filepath.Join(root, item.name), filepath.Join(s.sealed, item.name), item.want, r.UID, item.mode)
		if h.file != nil {
			s.held = append(s.held, h)
		}
		if e != nil {
			return e
		}
		sealedFile, e := pinnedRuntime(ctx, filepath.Join(s.sealed, item.name), item.want, 0)
		if e != nil {
			return e
		}
		s.held = append(s.held, sealedFile)
	}
	return nil
}
func (s *scopeState) prepare(ctx context.Context, root string, r request) error {
	if e := s.prepareExecutables(ctx, r); e != nil {
		return e
	}
	sealed, e := os.MkdirTemp("/tmp", "uap-grok-acp-sealed-")
	if e != nil {
		return e
	}
	s.sealed = sealed
	s.directory, e = holdDirectory(sealed, 0, 0)
	if e != nil {
		return e
	}
	if e = s.prepareCopies(ctx, root, r); e != nil {
		return e
	}
	ns, e := os.Stat("/proc/self/ns/net")
	if e != nil {
		return e
	}
	parent, e := identity(ns)
	if e != nil {
		return e
	}
	s.parent = parent.Ino
	req := collectorRequest{1, r.Commit, r.Tree, r.UID, r.GID, r.Nonce, r.Scope, root, parent.Ino, r.Start}
	body, e := json.Marshal(req)
	if e != nil {
		return e
	}
	path := filepath.Join(sealed, "collector-request.json")
	if e = os.WriteFile(path, body, 0444); e != nil {
		return e
	}
	sum := sha256.Sum256(body)
	h, e := pinnedRuntime(ctx, path, hex.EncodeToString(sum[:]), 0)
	if e != nil {
		return e
	}
	s.held = append(s.held, h)
	return s.directory.seal()
}
func (s *scopeState) verify(ctx context.Context, requestFile heldFile) error {
	if e := s.input.verify(); e != nil {
		return e
	}
	if e := s.directory.verify(); e != nil {
		return e
	}
	for _, h := range append(s.held, requestFile) {
		if e := h.verified(ctx); e != nil {
			return e
		}
	}
	return nil
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
func runScope(ctx, parent context.Context, root string, r request, requestFile heldFile, output io.Writer) (resultErr error) {
	s := &scopeState{safeToDelete: true}
	defer func() { resultErr = errors.Join(resultErr, s.close()) }()
	var e error
	s.input, e = holdDirectory(root, r.UID, r.GID)
	if e != nil {
		return e
	}
	if s.input.mode != os.ModeDir|0700 {
		return errors.New("input-root-mode-refused")
	}
	if e := s.prepare(ctx, root, r); e != nil {
		return e
	}
	if e := s.verify(ctx, requestFile); e != nil {
		return e
	}
	command, e := fixedCommand(s.sealed, root, r, s.systems)
	if e != nil {
		return e
	}
	s.safeToDelete = false
	raw, e := supervisedCapture(ctx, command, r, s.parent, (process.OS{}).RunDuplexWithPlannedShutdown)
	if e != nil {
		return e
	}
	s.safeToDelete = true
	final, cancel, e := finalBudget(parent, r.Start)
	if e != nil {
		return e
	}
	defer cancel()
	if e = s.verify(final, requestFile); e != nil {
		return e
	}
	if e = s.directory.remove(); e != nil {
		return errors.New("sealed-cleanup-refused")
	}
	s.safeToDelete = false
	body, e := json.Marshal(receipt{1, r.Commit, r.Tree, r.Scope, r.Nonce, true, true, raw, s.systems})
	if e != nil {
		return e
	}
	if _, e = output.Write(append(body, '\n')); e != nil {
		return errors.New("receipt-output-refused")
	}
	return nil
}
func fixedCommand(sealed, root string, r request, systems map[string]systemFact) (ports.Command, error) {
	if len(systems) != 4 {
		return ports.Command{}, errors.New("system-selection-refused")
	}
	for _, key := range []string{"unshare", "setpriv", "env", "python3"} {
		fact, ok := systems[key]
		if !ok || fact.Path == "" || !filepath.IsAbs(fact.Path) || filepath.Clean(fact.Path) != fact.Path || !hex64.MatchString(fact.SHA) {
			return ports.Command{}, errors.New("system-selection-refused")
		}
	}

	argv := []string{systems["unshare"].Path, "--net", "--", systems["setpriv"].Path, "--reuid=" + strconv.Itoa(r.UID), "--regid=" + strconv.Itoa(r.GID), "--clear-groups", "--inh-caps=-all", "--ambient-caps=-all", "--bounding-set=-all", "--no-new-privs", "--", systems["env"].Path, "-i", "PATH=/usr/bin:/bin", "LANG=C.UTF-8", systems["python3"].Path, "-I", filepath.Join(sealed, "collector.py"), "--collect", filepath.Join(sealed, "collector-request.json")}
	return ports.Command{Argv: argv, Env: []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}, Dir: root}, nil
}
