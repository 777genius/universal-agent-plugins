//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func inertRequest() request {
	return request{1, strings.Repeat("a", 40), strings.Repeat("b", 40), 1000, 1000, strings.Repeat("c", 32), "a", 1, strings.Repeat("d", 64), ""}
}
func inertResult(t *testing.T, r request) []byte {
	t.Helper()
	proof, e := json.Marshal(isolation{r.UID, r.GID, r.UID, r.GID, []int{}, 2, 1, []string{"lo"}, true, 1, false})
	if e != nil {
		t.Fatal(e)
	}
	body, e := json.Marshal(result{1, r.Commit, r.Tree, r.Scope, r.Nonce, false, "protocol-refused", proof, []json.RawMessage{}, json.RawMessage(`{"initialize":null,"plugins":null,"notifications":[]}`), "", nil})
	if e != nil {
		t.Fatal(e)
	}
	return append(body, '\n')
}
func TestClosedJSONAndOriginalMonotonicDeadline(t *testing.T) {
	for _, body := range []string{`{"x":1,"x":2}`, `{"x":1} {}`, `{"x":1,"secret":"private"}`, `{"x":NaN}`} {
		var v map[string]any
		if e := closed([]byte(body), &v, "x"); e == nil {
			t.Fatal("bad JSON admitted")
		}
	}
	var v map[string]any
	if e := closed([]byte(`{"x":`+strings.Repeat("[", 22)+`0`+strings.Repeat("]", 22)+`}`), &v, "x"); e == nil {
		t.Fatal("depth admitted")
	}
	if _, e := remaining(1, 1+int64(30*time.Second), 30*time.Second); e == nil {
		t.Fatal("expired original budget admitted")
	}
	if _, e := remaining(2, 1, 30*time.Second); e == nil {
		t.Fatal("future start admitted")
	}
	if d, e := remaining(1, 1+int64(29*time.Second), 30*time.Second); e != nil || d != time.Second {
		t.Fatal(d, e)
	}
	if d, e := remaining(1, 1+int64(34*time.Second), 35*time.Second); e != nil || d != time.Second {
		t.Fatal("cleanup reserve reset", d, e)
	}
}
func TestStrictReturnIsOnlyRawPublicationAuthority(t *testing.T) {
	r := inertRequest()
	body := inertResult(t, r)
	for _, fail := range []bool{false, true} {
		called := false
		run := func(_ context.Context, _ ports.Command, exchange func(io.Writer, io.Reader) error) error {
			called = true
			if e := exchange(io.Discard, bytes.NewReader(body)); e != nil {
				return e
			}
			if fail {
				return errors.New("tree unknown")
			}
			return nil
		}
		raw, e := supervisedCapture(context.Background(), ports.Command{}, r, 1, run)
		if !called {
			t.Fatal("exchange not run")
		}
		if fail {
			if e == nil || raw != nil {
				t.Fatal("unproved cleanup released raw")
			}
		} else if e != nil || !bytes.Equal(raw, body) {
			t.Fatal("negative semantic capture lost", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := supervisedCapture(ctx, ports.Command{}, r, 1, func(context.Context, ports.Command, func(io.Writer, io.Reader) error) error {
		t.Fatal("pre-cancel spawned")
		return nil
	})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestResultIdentityIsolationAndWholeStream(t *testing.T) {
	r := inertRequest()
	body := inertResult(t, r)
	if e := validateResult(body, r, 1); e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{{`"network_namespace":2`, `"network_namespace":1`}, {`"capabilities_zero":true`, `"capabilities_zero":false`}, {`"no_new_privileges":1`, `"no_new_privileges":0`}, {`"scope":"a"`, `"scope":"b"`}, {`"semantic_complete":false`, `"semantic_complete":true`}, {`"groups":[]`, `"groups":[1000]`}} {
		changed := bytes.Replace(body, []byte(change[0]), []byte(change[1]), 1)
		if bytes.Equal(changed, body) {
			t.Fatal("mutation missed")
		}
		if e := validateResult(changed, r, 1); e == nil {
			t.Fatal("changed proof admitted")
		}
	}
	for _, tail := range [][]byte{[]byte("\n"), []byte(" "), body, []byte(`{"extra":true}`)} {
		if _, e := exchange(io.MultiReader(bytes.NewReader(body), bytes.NewReader(tail)), r, 1); e == nil {
			t.Fatal("trailing collector output admitted")
		}
	}
	if _, e := exchange(strings.NewReader(strings.Repeat("x", resultLimit+1)), r, 1); e == nil {
		t.Fatal("flood admitted")
	}
	if _, e := exchange(bytes.NewReader(bytes.TrimSpace(body)), r, 1); e == nil {
		t.Fatal("partial frame admitted")
	}
}
func TestParentEOFPropagatesAndNeverRenewsBudget(t *testing.T) {
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close() }()
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchParent(reader, cancel)
	child, stop := context.WithTimeout(parent, time.Second)
	defer stop()
	if e := writer.Close(); e != nil {
		t.Fatal(e)
	}
	<-child.Done()
	if !errors.Is(child.Err(), context.Canceled) {
		t.Fatal(child.Err())
	}
	final, end, e := finalBudget(parent, 1)
	if e == nil {
		end()
		t.Fatal("ancient budget renewed", final)
	}
}
func TestClosedRequestAndFixedLaunch(t *testing.T) {
	a, b, c := sourceCommit, sourceTree, collectorSHA
	defer func() { sourceCommit, sourceTree, collectorSHA = a, b, c }()
	r := inertRequest()
	sourceCommit, sourceTree, collectorSHA = r.Commit, r.Tree, strings.Repeat("e", 64)
	root := "/tmp/uap-grok-acp-input-" + r.Nonce
	if e := validateRequest(r, root, r.UID, r.GID); e != nil {
		t.Fatal(e)
	}
	if e := validateRequest(r, "/tmp/foreign", r.UID, r.GID); e == nil {
		t.Fatal("foreign root admitted")
	}
	changed := r
	changed.Scope = "../a"
	if e := validateRequest(changed, root, r.UID, r.GID); e == nil {
		t.Fatal("foreign scope admitted")
	}
	systems := map[string]systemFact{}
	for _, name := range []string{"unshare", "setpriv", "env", "python3"} {
		path := "/usr/bin/" + name
		if name == "python3" {
			path += ".12"
		}
		systems[name] = systemFact{path, strings.Repeat("e", 64)}
	}
	cmd, e := fixedCommand("/tmp/sealed", root, r, systems)
	if e != nil {
		t.Fatal(e)
	}
	joined := strings.Join(cmd.Argv, " ")
	for _, required := range []string{"/usr/bin/unshare --net -- /usr/bin/setpriv", "--clear-groups", "--inh-caps=-all", "--ambient-caps=-all", "--bounding-set=-all", "--no-new-privs", "/usr/bin/env -i", "/usr/bin/python3.12 -I /tmp/sealed/collector.py --collect /tmp/sealed/collector-request.json"} {
		if !strings.Contains(joined, required) {
			t.Fatal(required)
		}
	}
	if strings.Join(cmd.Env, " ") != "PATH=/usr/bin:/bin LANG=C.UTF-8" {
		t.Fatal("inherited env")
	}
}
func inertFile(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func TestHeldFilesAndSealedPinsRefuseChanges(t *testing.T) {
	root := t.TempDir()
	path := inertFile(t, root, "collector.py", "inert\n")
	sum := sha256.Sum256([]byte("inert\n"))
	h, e := pinnedRuntime(context.Background(), path, hex.EncodeToString(sum[:]), os.Getuid())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = h.file.Close() }()
	if e = os.WriteFile(path, []byte("other\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = h.verified(context.Background()); e == nil {
		t.Fatal("changed sealed content admitted")
	}
	if e = os.Rename(path, path+"-old"); e != nil {
		t.Fatal(e)
	}
	inertFile(t, root, "collector.py", "inert\n")
	if e = h.unchanged(); e == nil {
		t.Fatal("replacement admitted")
	}
	if e = os.Link(path, path+"-alias"); e != nil {
		t.Fatal(e)
	}
	if _, e = openHeld(path, os.Getuid(), 32); e == nil {
		t.Fatal("hardlink admitted")
	}
	if e = os.Symlink(path, filepath.Join(root, "link")); e != nil {
		t.Fatal(e)
	}
	if _, e = openHeld(filepath.Join(root, "link"), os.Getuid(), 32); e == nil {
		t.Fatal("symlink admitted")
	}
}
func TestDirectoryCleanupRefusesReplacementModeAndMount(t *testing.T) {
	for _, kind := range []string{"replacement", "mode", "mount"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			path := filepath.Join(parent, "sealed")
			if e := os.Mkdir(path, 0700); e != nil {
				t.Fatal(e)
			}
			d, e := holdDirectory(path, os.Getuid(), os.Getgid())
			if e != nil {
				t.Fatal(e)
			}
			defer func() { _ = d.file.Close() }()
			switch kind {
			case "replacement":
				if e = os.Rename(path, path+"-original"); e != nil {
					t.Fatal(e)
				}
				if e = os.Mkdir(path, 0700); e != nil {
					t.Fatal(e)
				}
			case "mode":
				if e = os.Chmod(path, 0755); e != nil {
					t.Fatal(e)
				}
			case "mount":
				d.mount++
			}
			state := &scopeState{directory: d, safeToDelete: true}
			if e = state.close(); e == nil {
				t.Fatal("unproved prepare cleanup admitted")
			}
			if _, e = os.Lstat(path); e != nil {
				t.Fatal("foreign or changed root deleted", e)
			}
		})
	}
}
func TestDirectoryCleanupRemovesOnlyProvenOwnedRoot(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "sealed")
	if e := os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	d, e := holdDirectory(path, os.Getuid(), os.Getgid())
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = d.file.Close() }()
	inertFile(t, path, "partial-copy", "inert")
	if e = d.seal(); e != nil {
		t.Fatal(e)
	}
	if e = d.remove(); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Lstat(path); !os.IsNotExist(e) {
		t.Fatal("owned root not removed", e)
	}
}

func TestInputDirectoryReplacementIsRefusedWithoutDeletion(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "input")
	if e := os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	input, e := holdDirectory(path, os.Getuid(), os.Getgid())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(path, path+"-original"); e != nil {
		t.Fatal(e)
	}
	if e = os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	state := &scopeState{input: input}
	if e = state.verify(context.Background(), heldFile{}); e == nil {
		t.Fatal("replaced input root admitted")
	}
	if e = state.close(); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{path, path + "-original"} {
		if _, e = os.Lstat(name); e != nil {
			t.Fatal("input root deleted", e)
		}
	}
	for _, systems := range []map[string]systemFact{nil, {"unshare": {Path: "/usr/bin/unshare"}}} {
		if _, e = fixedCommand("/tmp/sealed", path, inertRequest(), systems); e == nil {
			t.Fatal("incomplete system selection admitted")
		}
	}
}


func TestSessionModeKeepsMetadataCommandAndDropSuffix(t *testing.T) {
	r := inertRequest()
	systems := map[string]systemFact{}
	for _, name := range []string{"unshare", "setpriv", "env", "python3"} {
		systems[name] = systemFact{"/usr/bin/" + name, strings.Repeat("a", 64)}
	}
	old, e := fixedCommand("/tmp/sealed", "/tmp/input", r, systems)
	if e != nil {
		t.Fatal(e)
	}
	r.Mode = sessionMode
	next, e := fixedCommand("/tmp/sealed", "/tmp/input", r, systems, 42)
	if e != nil {
		t.Fatal(e)
	}
	prefix := []string{"/usr/bin/unshare", "--net", "--", "/usr/bin/python3", "-I", "-c", loopbackBootstrap, "42"}
	if !reflect.DeepEqual(next.Argv[:len(prefix)], prefix) || !reflect.DeepEqual(next.Argv[len(prefix):], old.Argv[3:]) {
		t.Fatal("root bootstrap changed the verified drop chain")
	}
	if !reflect.DeepEqual(next.Env, old.Env) || next.Dir != old.Dir {
		t.Fatal("environment or cwd changed")
	}
	if _, e := fixedCommand("/tmp/sealed", "/tmp/input", r, systems); e == nil {
		t.Fatal("missing parent admitted")
	}
	r.Mode = "enable"
	if _, e := fixedCommand("/tmp/sealed", "/tmp/input", r, systems, 42); e == nil {
		t.Fatal("arbitrary mode admitted")
	}
}
func TestSessionResultRequiresModeLoopbackAndJoinedProvider(t *testing.T) {
	r := inertRequest()
	var body map[string]any
	if e := json.Unmarshal(inertResult(t, r), &body); e != nil {
		t.Fatal(e)
	}
	r.Mode = sessionMode
	body["mode"] = sessionMode
	body["provider"] = map[string]any{"requests": 1, "models_get": 1, "root_get": 0, "forbidden": 0, "settled": true}
	body["isolation"].(map[string]any)["loopback_up"] = true
	body["rpc"].(map[string]any)["session"] = nil
	raw, _ := json.Marshal(body)
	if e := validateResult(raw, r, 1); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"settled", "requests", "private"} {
		p := body["provider"].(map[string]any)
		original, existed := p[key]
		p[key] = "PRIVATE"
		changed, _ := json.Marshal(body)
		if e := validateResult(changed, r, 1); e == nil {
			t.Fatal("provider schema admitted", key)
		}
		if existed {
			p[key] = original
		} else {
			delete(p, key)
		}
	}
	body["isolation"].(map[string]any)["loopback_up"] = false
	raw, _ = json.Marshal(body)
	if e := validateResult(raw, r, 1); e == nil {
		t.Fatal("down loopback admitted")
	}
}
