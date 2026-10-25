//go:build linux

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"time"
)

const proofPrefix = "UAP_VSCODE_ISOLATION_V1 "
const isolationVerifier = `
import json,os,socket,sys
parent,uid,gid,env,argv=sys.argv[1:]
p={"pid":os.getpid(),"network_namespace":os.stat("/proc/self/ns/net").st_ino,"parent_network_namespace":int(parent),"uid":os.getuid(),"gid":os.getgid(),"euid":os.geteuid(),"egid":os.getegid(),"groups":os.getgroups(),"interfaces":[name for _,name in socket.if_nameindex()]}
if p["network_namespace"]==int(parent) or p["uid"]!=int(uid) or p["gid"]!=int(gid) or p["uid"]==0 or p["gid"]==0 or p["euid"]!=int(uid) or p["egid"]!=int(gid) or p["groups"] or p["interfaces"]!=["lo"]:raise SystemExit("isolated process admission failed")
with open("/proc/self/status") as f:status=dict(line.rstrip().split(":",1) for line in f if ":" in line)
if any(int(status[k].strip(),16) for k in ("CapInh","CapPrm","CapEff","CapBnd","CapAmb")) or status["NoNewPrivs"].strip()!="1":raise SystemExit("native privilege drop failed")
p["capabilities_zero"]=True;p["no_new_privileges"]=True
os.write(1,("UAP_VSCODE_ISOLATION_V1 "+json.dumps(p,separators=(",",":"))+"\n").encode())
os.dup2(1,2)
os.execve(json.loads(argv)[0],json.loads(argv),json.loads(env))
`

type mergedCapture struct {
	mu       sync.Mutex
	body     []byte
	overflow bool
	err      error
	proof    isolationProof
	ready    chan struct{}
	done     chan struct{}
	cancel   context.CancelFunc
	planned  bool
}

func (m *mergedCapture) append(p []byte) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	remaining := streamLimit - len(m.body)
	if len(p) > remaining {
		m.body = append(m.body, p[:remaining]...)
		m.overflow = true
		m.err = fmt.Errorf("merged diagnostic limit exceeded")
		m.cancel()
		return false
	}
	m.body = append(m.body, p...)
	return true
}
func (m *mergedCapture) fail(err error) {
	m.mu.Lock()
	m.err = errors.Join(m.err, err)
	m.mu.Unlock()
	m.cancel()
}
func (m *mergedCapture) drain(reader io.Reader, uid, gid int, parent uint64) {
	defer close(m.done)
	r := bufio.NewReaderSize(reader, 4097)
	line, err := r.ReadSlice('\n')
	m.append(line)
	if err != nil || len(line) > 4096 || !bytes.HasPrefix(line, []byte(proofPrefix)) {
		m.fail(errors.Join(fmt.Errorf("same-process proof frame missing/oversized"), err))
		close(m.ready)
		return
	}
	body := bytes.TrimSuffix(line[len(proofPrefix):], []byte{'\n'})
	var proof isolationProof
	if _, err = uniqueJSON(body); err == nil {
		err = json.Unmarshal(body, &proof)
	}
	m.mu.Lock()
	m.proof = proof
	m.mu.Unlock()
	if proofErr := validateIsolationProof(proof, uid, gid, parent, err); proofErr != nil {
		m.fail(proofErr)
		close(m.ready)
		return
	}
	close(m.ready)
	buffer := make([]byte, 64<<10)
	for {
		n, e := r.Read(buffer)
		if n > 0 && !m.append(buffer[:n]) {
			return
		}
		if e != nil {
			m.mu.Lock()
			planned := m.planned
			m.mu.Unlock()
			if !errors.Is(e, io.EOF) && (!planned || !errors.Is(e, os.ErrClosed)) {
				m.fail(fmt.Errorf("merged diagnostic read failed: %w", e))
			}
			return
		}
	}
}
func validateIsolationProof(proof isolationProof, uid, gid int, parent uint64, err error) error {
	if err != nil || proof.PID <= 0 || uid <= 0 || gid <= 0 || parent == 0 || proof.Namespace == 0 || proof.UID != uid || proof.GID != gid || proof.EUID != uid || proof.EGID != gid || proof.Namespace == parent || proof.ParentNamespace != parent || len(proof.Groups) != 0 || !reflect.DeepEqual(proof.Interfaces, []string{"lo"}) || !proof.CapabilitiesZero || !proof.NoNewPrivileges {
		return errors.Join(fmt.Errorf("same-process identity/namespace/privilege proof differs"), err)
	}
	return nil
}
func (c *controller) isolationCommand(argv []string, cwd string, env map[string]string) (ports.Command, error) {
	for _, path := range []string{"/usr/bin/unshare", "/usr/bin/setpriv", "/usr/bin/env", "/usr/bin/python3"} {
		if err := trustedSystem(path); err != nil {
			return ports.Command{}, err
		}
	}
	environment, err := json.Marshal(env)
	if err != nil {
		return ports.Command{}, err
	}
	args, err := json.Marshal(argv)
	if err != nil {
		return ports.Command{}, err
	}
	command := []string{"/usr/bin/unshare", "--net", "--", "/usr/bin/setpriv", "--reuid=" + strconv.Itoa(c.request.Input.UID), "--regid=" + strconv.Itoa(c.request.Input.GID), "--clear-groups", "--bounding-set=-all", "--inh-caps=-all", "--ambient-caps=-all", "--no-new-privs", "/usr/bin/env", "-i", "/usr/bin/python3", "-I", "-c", isolationVerifier, strconv.FormatUint(c.parentNS, 10), strconv.Itoa(c.request.Input.UID), strconv.Itoa(c.request.Input.GID), string(environment), string(args)}
	return ports.Command{Argv: command, Env: []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}, Dir: cwd}, nil
}
func (c *controller) strict(ctx context.Context, label string, argv []string, cwd string, env map[string]string, exchange func(context.Context, isolationProof) error) (childRecord, error) {
	if err := ctx.Err(); err != nil {
		return childRecord{Label: label, Error: clipped(err)}, err
	}
	record := childRecord{Label: label, Argv: append([]string(nil), argv...), Cwd: cwd, Diagnostics: "verifier-prefixed merged stdout/stderr; planned teardown may truncate final diagnostic tail"}
	command, err := c.isolationCommand(argv, cwd, env)
	if err != nil {
		return record, err
	}
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	capture := &mergedCapture{ready: make(chan struct{}), done: make(chan struct{}), cancel: cancel}
	if label == "Xvfb" {
		capture.cancel = c.cancelWork
	}
	started := false
	err = c.runner.RunDuplexWithPlannedShutdown(childCtx, command, func(_ io.Writer, r io.Reader) error {
		started = true
		go capture.drain(r, c.request.Input.UID, c.request.Input.GID, c.parentNS)
		select {
		case <-capture.ready:
		case <-childCtx.Done():
			return childCtx.Err()
		}
		capture.mu.Lock()
		proofErr := capture.err
		proof := capture.proof
		capture.mu.Unlock()
		if proofErr != nil {
			return proofErr
		}
		exchangeErr := exchange(childCtx, proof)
		if exchangeErr == nil {
			capture.mu.Lock()
			capture.planned = true
			capture.mu.Unlock()
		}
		return exchangeErr
	})
	if started {
		select {
		case <-capture.done:
		case <-time.After(time.Second):
			err = errors.Join(err, fmt.Errorf("merged drain did not join after strict pipe close"))
		}
	}
	return c.finishCapture(label, record, capture, err)
}
func (c *controller) finishCapture(label string, record childRecord, capture *mergedCapture, err error) (childRecord, error) {
	capture.mu.Lock()
	body := append([]byte(nil), capture.body...)
	record.Proof = capture.proof
	record.MergedOverflow = capture.overflow
	err = errors.Join(err, capture.err)
	capture.mu.Unlock()
	if c.cookie != "" {
		body = bytes.ReplaceAll(body, []byte(c.cookie), []byte("[task-cookie-redacted]"))
		record.Diagnostics += "; task-cookie bytes redacted"
	}
	sum := sha256.Sum256(body)
	record.MergedFile = label + ".merged.log"
	record.MergedBytes = len(body)
	record.MergedSHA = hex.EncodeToString(sum[:])
	record.PlannedStop = err == nil
	record.TreeReaped = err == nil
	record.Error = clipped(err)
	if e := c.keep(record.MergedFile, body); e != nil {
		err = errors.Join(err, e)
		record.Error = clipped(err)
	}
	c.children = append(c.children, record)
	return record, err
}
func (c *controller) environment(folder, result, profile string) map[string]string {
	base := c.request.Base
	home := filepath.Join(base, "home")
	env := map[string]string{"HOME": home, "USERPROFILE": home, "PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "TMPDIR": filepath.Join(base, "tmp"), "DISPLAY": display, "XAUTHORITY": filepath.Join(base, "auth", "authority"), "UAP_VSCODE_WORKSPACE_FOLDER": folder, "UAP_VSCODE_WORKSPACE_RESULT": result}
	for key, dir := range map[string]string{"XDG_CONFIG_HOME": "config", "XDG_CACHE_HOME": "cache", "XDG_STATE_HOME": "state", "XDG_DATA_HOME": "data", "NPM_CONFIG_CACHE": "npm-cache"} {
		env[key] = filepath.Join(home, dir)
	}
	for key, name := range map[string]string{"NPM_CONFIG_USERCONFIG": "npm-user-config", "NPM_CONFIG_GLOBALCONFIG": "npm-global-config", "GIT_CONFIG_GLOBAL": "git-config"} {
		env[key] = filepath.Join(home, name)
	}
	env["GIT_CONFIG_NOSYSTEM"] = "1"
	env["GIT_TERMINAL_PROMPT"] = "0"
	_ = profile
	return env
}
func guiArgv(base, folder, profile string) []string {
	return []string{filepath.Join(base, "publisher", publisherPrefix, "code"), "--new-window", "--disable-crash-reporter", "--user-data-dir", profile, "--extensions-dir", filepath.Join(filepath.Dir(profile), "extensions"), "--extensionDevelopmentPath", filepath.Join(base, "observer"), "--extensionTestsPath", filepath.Join(base, "observer", "runner.cjs"), "--enable-proposed-api", "uap-research.workspace-observer", folder}
}
func (c *controller) system(label, path string, args []string, input []byte, redact bool) error {
	stdout, stderr, err := boundedSystem(c.work, path, args, input, controlLimit, 5*time.Second)
	if redact {
		stdout = bytes.ReplaceAll(stdout, []byte(c.cookie), []byte("[task-cookie-redacted]"))
		stderr = bytes.ReplaceAll(stderr, []byte(c.cookie), []byte("[task-cookie-redacted]"))
	}
	if e := c.keep(label+".stdout.log", stdout); e != nil {
		err = errors.Join(err, e)
	}
	if e := c.keep(label+".stderr.log", stderr); e != nil {
		err = errors.Join(err, e)
	}
	outHash, errHash := sha256.Sum256(stdout), sha256.Sum256(stderr)
	record := map[string]any{"argv": append([]string{path}, args...), "error": clipped(err), "redacted": redact, "stdout_bytes": len(stdout), "stderr_bytes": len(stderr), "stdout_sha256": hex.EncodeToString(outHash[:]), "stderr_sha256": hex.EncodeToString(errHash[:])}
	body, e := encode(record, controlLimit)
	if e == nil {
		e = c.keep(label+".json", body)
	}
	return errors.Join(err, e)
}
func fixedResultPath(base, label string) string {
	return filepath.Join(base, "probes", label, "results", "observation.json")
}
