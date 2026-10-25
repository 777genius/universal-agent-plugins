//go:build linux

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var shaPattern = regexp.MustCompile("^[0-9a-f]{64}$")
var commitPattern = regexp.MustCompile("^[0-9a-f]{40}$")

func hostedRoot() error {
	for key, want := range map[string]string{"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"} {
		if os.Getenv(key) != want {
			return fmt.Errorf("genuine disposable hosted admission required")
		}
	}
	if os.Getuid() != 0 || os.Geteuid() != 0 || !commitPattern.MatchString(buildCommit) || os.Getenv("EXPECTED_COMMIT") != buildCommit {
		return fmt.Errorf("fixed exact-source privileged controller required")
	}
	return nil
}
func callerIDs() (int, int, error) {
	uid, e := strconv.Atoi(os.Getenv("SUDO_UID"))
	gid, ge := strconv.Atoi(os.Getenv("SUDO_GID"))
	if e != nil || ge != nil || uid <= 0 || gid <= 0 || uid > 1<<30 || gid > 1<<30 {
		return 0, 0, fmt.Errorf("trusted original sudo caller required")
	}
	return uid, gid, nil
}
func decodeInput(root string) (inputRequest, error) {
	var req inputRequest
	uid, gid, err := callerIDs()
	if err != nil {
		return req, err
	}
	info, err := exactDirectory(root, uid)
	if err != nil {
		return req, err
	}
	if filepath.Dir(root) != "/tmp" || !strings.HasPrefix(filepath.Base(root), "vscode-workspace-input-") || info.Mode().Perm() != 0700 {
		return req, fmt.Errorf("task-only input authority refused")
	}
	body, err := readOrdinary(filepath.Join(root, "request.json"), 16<<10)
	if err != nil {
		return req, err
	}
	if _, err = uniqueJSON(body); err != nil {
		return req, err
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&req); err != nil {
		return req, err
	}
	if req.UID != uid || req.EUID != uid || req.GID != gid || req.EGID != gid || req.SourceCommit != buildCommit || !shaPattern.MatchString(req.HelperSHA) {
		return req, fmt.Errorf("request identity or source differs from trusted caller")
	}
	deadline, err := deadlineAt(req.StartNS, 420*time.Second)
	if err != nil || time.Until(deadline) <= 0 {
		return req, fmt.Errorf("work budget exhausted before initialization")
	}
	if _, err = exactDirectory(req.ObserverRoot, uid); err != nil {
		return req, err
	}
	if _, err = exactDirectory(req.OutputRoot, uid); err != nil {
		return req, err
	}
	return req, nil
}
func trustedSystem(path string) error {
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(canonical) {
		return fmt.Errorf("absolute system executable required")
	}
	info, err := os.Lstat(canonical)
	if err != nil {
		return err
	}
	uid, err := statUID(info)
	if err != nil || uid != 0 || !regular(info, 128<<20) || info.Mode().Perm()&022 != 0 || info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("ordinary root-owned system executable required")
	}
	return immutableAncestors(filepath.Dir(canonical))
}
func bootstrap(root string) (resultErr error) {
	req, err := decodeInput(root)
	if err != nil {
		return err
	}
	deadline, err := deadlineAt(req.StartNS, 420*time.Second)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	base, err := os.MkdirTemp("/tmp", "uap-vscode-sealed-")
	if err != nil {
		return err
	}
	failed := true
	defer func() {
		if failed {
			resultErr = errors.Join(resultErr, cleanSealed(base))
		}
	}()
	if err = prepareSealed(ctx, root, base, req); err != nil {
		return err
	}
	env := []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "GITHUB_ACTIONS=true", "RUNNER_ENVIRONMENT=github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED=1", "EXPECTED_COMMIT=" + buildCommit, "SUDO_UID=" + strconv.Itoa(req.UID), "SUDO_GID=" + strconv.Itoa(req.GID)}
	failed = false
	err = unix.Exec(filepath.Join(base, "control", "controller"), []string{filepath.Join(base, "control", "controller"), "--sealed-root", base}, env)
	failed = true
	return err
}
func prepareSealed(ctx context.Context, root, base string, req inputRequest) error {
	if err := os.Mkdir(filepath.Join(base, "control"), 0700); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(base, "publisher"), 0755); err != nil {
		return err
	}
	hashes, err := copyPublisher(ctx, filepath.Join(root, "publisher", publisherPrefix), filepath.Join(base, "publisher", publisherPrefix), req.UID)
	if err != nil {
		return err
	}
	if err := copyObserver(ctx, req.ObserverRoot, base, req.UID); err != nil {
		return err
	}
	source, err := os.Executable()
	if err != nil {
		return err
	}
	digest, err := copyOrdinary(ctx, source, filepath.Join(base, "control", "controller"), req.UID, true)
	if err != nil {
		return err
	}
	if digest != req.HelperSHA {
		return fmt.Errorf("controller binary digest differs")
	}
	body, err := encode(sealedRequest{Input: req, Base: base, ExecutableHashes: hashes}, 16<<10)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(base, "control", "request.json"), body, 0600); err != nil {
		return err
	}
	return os.Chmod(base, 0755)
}
func copyObserver(ctx context.Context, source, base string, uid int) error {
	if len(observerPins) != 4 {
		return fmt.Errorf("exact reviewed observer pins missing")
	}
	observer := filepath.Join(base, "observer")
	if err := os.Mkdir(observer, 0755); err != nil {
		return err
	}
	for name, want := range observerPins {
		if filepath.Base(name) != name || !shaPattern.MatchString(want) {
			return fmt.Errorf("observer pin is not closed")
		}
		digest, err := copyOrdinary(ctx, filepath.Join(source, name), filepath.Join(observer, name), uid, false)
		if err != nil {
			return err
		}
		if digest != want {
			return fmt.Errorf("observer source digest differs")
		}
	}
	return nil
}

const cleanupCode = `
import os,stat,sys
path,device,inode=sys.argv[1:]
parent,name=os.path.split(path)
pfd=os.open(parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
root=os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=pfd)
identity=os.fstat(root)
if (identity.st_dev,identity.st_ino)!=(int(device),int(inode)):raise SystemExit("cleanup root identity changed")
def clear(fd):
    for entry in os.listdir(fd):
        item=os.stat(entry,dir_fd=fd,follow_symlinks=False)
        if stat.S_ISDIR(item.st_mode):
            child=os.open(entry,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
            try:
                if not os.path.samestat(item,os.fstat(child)):raise RuntimeError("cleanup directory changed")
                clear(child)
            finally:os.close(child)
            os.rmdir(entry,dir_fd=fd)
        else:os.unlink(entry,dir_fd=fd)
clear(root)
if not os.path.samestat(identity,os.stat(name,dir_fd=pfd,follow_symlinks=False)):raise SystemExit("cleanup root replaced")
os.close(root)
os.rmdir(name,dir_fd=pfd)
os.close(pfd)
`

type liveBuffer struct {
	mu       sync.Mutex
	body     []byte
	limit    int
	overflow bool
	cancel   context.CancelFunc
}

func (b *liveBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	allow := b.limit - len(b.body)
	if n > allow {
		b.body = append(b.body, p[:allow]...)
		b.overflow = true
		b.cancel()
		return allow, fmt.Errorf("bounded child output exceeded")
	}
	b.body = append(b.body, p...)
	return n, nil
}
func (b *liveBuffer) bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.body...)
}
func boundedSystem(parent context.Context, path string, args []string, input []byte, limit int, timeout time.Duration) ([]byte, []byte, error) {
	if err := trustedSystem(path); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	out, stderr := &liveBuffer{limit: limit, cancel: cancel}, &liveBuffer{limit: limit, cancel: cancel}
	cmd, err := fixedSystemCommand(ctx, path, args)
	if err != nil {
		return nil, nil, err
	}
	cmd.Dir = "/"
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
	cmd.Stdin = strings.NewReader(string(input))
	cmd.Stdout = out
	cmd.Stderr = stderr
	cmd.WaitDelay = 100 * time.Millisecond
	err = cmd.Run()
	if ctx.Err() != nil {
		err = errors.Join(err, ctx.Err())
	}
	return out.bytes(), stderr.bytes(), err
}
func fixedSystemCommand(ctx context.Context, path string, args []string) (*exec.Cmd, error) {
	switch path {
	case "/usr/bin/python3":
		if len(args) != 6 || args[0] != "-I" || args[1] != "-c" || args[2] != cleanupCode || !sealedRootOperand(args[3]) || !decimalOperand(args[4]) || !decimalOperand(args[5]) {
			return nil, fmt.Errorf("cleanup argv is outside the fixed table")
		}
		cmd := exec.CommandContext(ctx, "/usr/bin/python3", "-I", "-c", cleanupCode)
		cmd.Args = append(cmd.Args, args[3], args[4], args[5])
		return cmd, nil
	case "/usr/bin/xauth":
		if len(args) != 4 || args[0] != "-f" || args[2] != "source" || args[3] != "-" || filepath.Base(args[1]) != "authority" || filepath.Base(filepath.Dir(args[1])) != "auth" || !sealedRootOperand(filepath.Dir(filepath.Dir(args[1]))) {
			return nil, fmt.Errorf("xauth argv is outside the fixed table")
		}
		cmd := exec.CommandContext(ctx, "/usr/bin/xauth", "-f")
		cmd.Args = append(cmd.Args, args[1], "source", "-")
		return cmd, nil
	default:
		return nil, fmt.Errorf("system command is outside the fixed table")
	}
}
func sealedRootOperand(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && filepath.Dir(path) == "/tmp" && strings.HasPrefix(filepath.Base(path), "uap-vscode-sealed-")
}
func decimalOperand(value string) bool {
	number, err := strconv.ParseUint(value, 10, 64)
	return err == nil && strconv.FormatUint(number, 10) == value
}
func cleanSealedCapture(base string) ([]byte, []byte, error) {
	info, err := exactDirectory(base, 0)
	if err != nil {
		return nil, nil, err
	}
	if !sealedRootOperand(base) {
		return nil, nil, fmt.Errorf("cleanup authority refused")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, nil, fmt.Errorf("cleanup Linux identity unavailable")
	}
	return boundedSystem(context.Background(), "/usr/bin/python3", []string{"-I", "-c", cleanupCode, base, strconv.FormatUint(st.Dev, 10), strconv.FormatUint(st.Ino, 10)}, nil, controlLimit, 4500*time.Millisecond)
}
func cleanSealed(base string) error {
	out, stderr, err := cleanSealedCapture(base)
	if err != nil {
		return fmt.Errorf("held-root cleanup failed: %w; stdout=%q stderr=%q", err, out, stderr)
	}
	return nil
}
func randomCookie() (string, error) {
	var body [16]byte
	if _, err := rand.Read(body[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(body[:]), nil
}
func runSealed(base string) (err error) {
	if !sealedRootOperand(base) {
		return fmt.Errorf("sealed task root refused")
	}
	if _, err = exactDirectory(base, 0); err != nil {
		return err
	}
	if err = immutableAncestors(base); err != nil {
		return err
	}
	cleaned := false
	defer func() {
		if !cleaned {
			err = errors.Join(err, cleanSealed(base))
		}
	}()
	req, err := loadSealedRequest(base)
	if err != nil {
		return err
	}
	work, cancel, final, err := sealedWork(req)
	if err != nil {
		return err
	}
	defer cancel()
	c, err := newSealedController(work, req, cancel, final)
	if err != nil {
		return err
	}
	defer func() { _ = c.output.Close() }()
	runErr := c.execute()
	out, stderr, baseErr := cleanSealedCapture(base)
	cleanupErr := errors.Join(baseErr, c.displayCleanupErr)
	cleaned = true
	return c.finish(runErr, cleanupErr, out, stderr)
}
func loadSealedRequest(base string) (sealedRequest, error) {
	body, err := readOrdinary(filepath.Join(base, "control", "request.json"), 16<<10)
	if err != nil {
		return sealedRequest{}, err
	}
	var req sealedRequest
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&req); err != nil {
		return sealedRequest{}, err
	}
	uid, gid, err := callerIDs()
	if err != nil || req.Base != base || req.Input.UID != uid || req.Input.GID != gid || req.Input.SourceCommit != buildCommit {
		return sealedRequest{}, fmt.Errorf("sealed request identity differs")
	}
	return req, nil
}
func sealedWork(req sealedRequest) (context.Context, context.CancelFunc, time.Time, error) {
	final, err := deadlineAt(req.Input.StartNS, 440*time.Second)
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	deadline, err := deadlineAt(req.Input.StartNS, 420*time.Second)
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	work, cancel := context.WithDeadline(context.Background(), deadline)
	go func() { var one [1]byte; _, _ = os.Stdin.Read(one[:]); cancel() }()
	signals, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case <-signals.Done():
			cancel()
		case <-work.Done():
		}
	}()
	return work, func() { cancel(); stop() }, final, nil
}
func newSealedController(work context.Context, req sealedRequest, cancel context.CancelFunc, final time.Time) (*controller, error) {
	fd, err := unix.Open(req.Input.OutputRoot, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	output := os.NewFile(uintptr(fd), req.Input.OutputRoot)
	c := &controller{request: req, deadline: final, work: work, cancelWork: cancel, runner: process.OS{}, files: map[string][]byte{}, output: output, hashes: req.ExecutableHashes}
	var ns unix.Stat_t
	if err := unix.Stat("/proc/self/ns/net", &ns); err != nil {
		_ = output.Close()
		return nil, err
	}
	c.parentNS = ns.Ino
	return c, nil
}
func (c *controller) finish(runErr, cleanupErr error, out, stderr []byte) error {
	if e := c.keep("root-cleanup.stdout.log", out); e != nil {
		cleanupErr = errors.Join(cleanupErr, e)
	}
	if e := c.keep("root-cleanup.stderr.log", stderr); e != nil {
		cleanupErr = errors.Join(cleanupErr, e)
	}
	outHash, errHash := sha256.Sum256(out), sha256.Sum256(stderr)
	summary := map[string]any{"evidence_kind": "vscode-pinned-untrusted-workspace-research", "source_commit": buildCommit, "version": publisherVersion, "publisher_commit": publisherCommit, "collection_complete": runErr == nil && cleanupErr == nil, "native_project_qualified": false, "native_mcp_initialization_qualified": false, "native_runtime_qualified": false, "children": c.children, "observations": c.observations, "executable_hashes": c.hashes, "error": clipped(runErr), "cleanup": map[string]any{"complete": cleanupErr == nil, "error": clipped(cleanupErr), "stdout_file": "root-cleanup.stdout.log", "stderr_file": "root-cleanup.stderr.log", "stdout_bytes": len(out), "stderr_bytes": len(stderr), "stdout_sha256": hex.EncodeToString(outHash[:]), "stderr_sha256": hex.EncodeToString(errHash[:])}, "diagnostics": "verifier-prefixed merged streams; planned teardown may truncate trailing diagnostics; API result is separate"}
	body, e := encode(summary, 1<<20)
	if e != nil {
		return errors.Join(runErr, cleanupErr, e)
	}
	if e = c.keep("vscode-workspace-controller.json", body); e != nil {
		return errors.Join(runErr, cleanupErr, e)
	}
	if e = c.publish(); e != nil {
		return errors.Join(runErr, cleanupErr, e)
	}
	public, e := encode(map[string]any{"collection_complete": runErr == nil && cleanupErr == nil, "cleanup_complete": cleanupErr == nil, "error": clipped(errors.Join(runErr, cleanupErr))}, controlLimit)
	if e != nil {
		return errors.Join(runErr, cleanupErr, e)
	}
	_, e = os.Stdout.Write(public)
	return errors.Join(runErr, cleanupErr, e)
}
func (c *controller) publish() error {
	before, err := c.output.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(c.request.Input.OutputRoot)
	if err != nil || !os.SameFile(before, current) || !current.IsDir() {
		return fmt.Errorf("evidence output root changed")
	}
	for _, name := range sortedKeys(c.files) {
		deadline, e := deadlineAt(c.request.Input.StartNS, 450*time.Second)
		if e != nil || time.Until(deadline) <= 0 {
			return fmt.Errorf("controller publication deadline")
		}
		fd, err := unix.Openat(int(c.output.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
		if err != nil {
			return err
		}
		f := os.NewFile(uintptr(fd), name)
		_, err = f.Write(c.files[name])
		if err == nil {
			err = f.Chown(c.request.Input.UID, c.request.Input.GID)
		}
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return errors.Join(err, closeErr)
		}
	}
	return nil
}
func main() {
	if err := hostedRoot(); err != nil {
		fmt.Fprintln(os.Stderr, clipped(err))
		os.Exit(2)
	}
	var err error
	if len(os.Args) != 3 {
		err = fmt.Errorf("fixed controller entry required")
	} else {
		switch os.Args[1] {
		case "--input-root":
			err = bootstrap(os.Args[2])
		case "--sealed-root":
			err = runSealed(os.Args[2])
		default:
			err = fmt.Errorf("fixed controller entry required")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, clipped(err))
		os.Exit(1)
	}
}
