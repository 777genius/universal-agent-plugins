//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestNativeLogParserPrivacy(t *testing.T) {
	prefix := "2026-11-13 18:00:00.001 [info] "
	good := prefix + "Starting server owned\n" + prefix + "Connection state: Starting\n" + prefix + "Connection state: Running\n"
	facts, err := parseNativeLog([]byte(good), "owned")
	if err != nil || !facts.TransportStart || facts.Starting != 1 || facts.Running != 1 || facts.Last != "Running" {
		t.Fatalf("finite native facts: %+v %v", facts, err)
	}
	secret := "/private/config TOKEN=secret"
	facts, err = parseNativeLog([]byte(prefix+"Connection state: Error "+secret+"\n"), "owned")
	if err != nil || facts.Last != "Error" || facts.Error != 1 {
		t.Fatal("fixed error state refused")
	}
	for _, body := range []string{strings.TrimSuffix(good, "\n"), prefix + "Starting server foreign\n", prefix + "[server stderr] " + secret + "\n" + good, prefix + "Connection state: Error " + secret + "\n" + good, strings.Repeat(good, 256), "2026-99-99 18:00:00.001 [info] Connection state: Running\n"} {
		if _, err := parseNativeLog([]byte(body), "owned"); err == nil {
			t.Fatal("unattributed or injected native log accepted")
		}
	}
}
func TestNativeLogSettledAuthority(t *testing.T) {
	profile := t.TempDir()
	uid := os.Getuid()
	baseline, err := beginNativeLog(profile, uid)
	if err != nil {
		t.Fatal(err)
	}
	defer baseline.close()
	session := filepath.Join(profile, "logs", "20261113T180000", "window1")
	if err := os.MkdirAll(session, 0700); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(session, "mcpServer.mcp.config.ws0.owned.log")
	if err := os.WriteFile(leaf, []byte("2026-11-13 18:00:00.001 [info] Starting server owned\n"), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	unsettled, _ := baseline.collectChecked(false, deadline, "owned")
	late, _ := baseline.collectChecked(true, time.Now(), "owned")
	if unsettled.Status != "unavailable" || late.Status != "unavailable" {
		t.Fatal("unsettled/late read admitted")
	}
	if facts, reason := baseline.collectChecked(true, deadline, "owned"); facts.Status != "observed" || !facts.TransportStart {
		t.Fatalf("settled projection refused at own predicate %s", reason)
	}
	if err := os.Link(leaf, filepath.Join(profile, "alias")); err != nil {
		t.Fatal(err)
	}
	if facts, _ := baseline.collectChecked(true, deadline, "owned"); facts.Status != "unavailable" {
		t.Fatal("foreign hardlink admitted")
	}
	if err := os.Remove(filepath.Join(profile, "alias")); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(profile, "logs", "20261113T180000", "old-window")
	if err := os.Rename(session, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(old, session); err != nil {
		t.Fatal(err)
	}
	if facts, _ := baseline.collectChecked(true, deadline, "owned"); facts.Status != "unavailable" {
		t.Fatal("aliased window admitted")
	}
}
func TestNativeLogNoPriorSessionAdoption(t *testing.T) {
	profile := t.TempDir()
	session := filepath.Join(profile, "logs", "20261113T180000", "window1")
	if err := os.MkdirAll(session, 0700); err != nil {
		t.Fatal(err)
	}
	baseline, err := beginNativeLog(profile, os.Getuid())
	if err != nil {
		t.Fatal(err)
	}
	defer baseline.close()
	if facts, _ := baseline.collectChecked(true, time.Now().Add(time.Second), "owned"); facts.Status != "unavailable" {
		t.Fatal("old session adopted")
	}
	if err := os.Rename(filepath.Join(profile, "logs"), filepath.Join(profile, "original-logs")); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(profile, "logs", "20261113T180100", "window1")
	if err := os.MkdirAll(replacement, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(replacement, "mcpServer.mcp.config.ws0.owned.log"), []byte("2026-11-13 18:00:00.001 [info] Starting server owned\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if facts, _ := baseline.collectChecked(true, time.Now().Add(time.Second), "owned"); facts.Status != "unavailable" {
		t.Fatal("replacement logs root adopted")
	}
	original := profile + "-original"
	if err := os.Rename(profile, original); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Rename(original, profile) }()
	if err := os.Mkdir(profile, 0700); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(profile) }()
	if facts, _ := baseline.collectChecked(true, time.Now().Add(time.Second), "owned"); facts.Status != "unavailable" {
		t.Fatal("replacement profile adopted")
	}
}

type nativeLogReadTripwire struct{ t *testing.T }

func (r nativeLogReadTripwire) Read(_ []byte) (int, error) {
	r.t.Fatal("unsettled/changed authority reached bytes")
	return 0, nil
}
func TestNativeLogBodyAuthorityPrecedesRead(t *testing.T) {
	if _, err := readNativeLogBody(nativeLogReadTripwire{t}, 1, func() bool { return false }); err == nil {
		t.Fatal("refused guard accepted")
	}
	calls := 0
	if _, err := readNativeLogBody(strings.NewReader("safe"), 4, func() bool { calls++; return calls == 1 }); err == nil {
		t.Fatal("postread drift accepted")
	}
	if body, err := readNativeLogBody(strings.NewReader("safe"), 4, func() bool { return true }); err != nil || string(body) != "safe" {
		t.Fatal("stable bounded body refused")
	}
}

type nativeLogStatView struct {
	os.FileInfo
	value syscall.Stat_t
}

func (v nativeLogStatView) Sys() any { return &v.value }
func TestNativeLogPrivatePredicateNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-cookie-path")
	if err := os.WriteFile(path, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	original, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"links", "ctime", "uid", "gid"} {
		value := *original.Sys().(*syscall.Stat_t)
		switch field {
		case "links":
			value.Nlink++
		case "ctime":
			value.Ctim.Sec++
		case "uid":
			value.Uid++
		case "gid":
			value.Gid++
		}
		if code := nativeLogDifference(original, nativeLogStatView{original, value}); code != nativeLogCheck(field) {
			t.Fatalf("wrong fixed predicate %s", code)
		}
	}
	if nativeLogCode(fmt.Errorf("private-cookie-path secret")) != "io" {
		t.Fatal("arbitrary error escaped")
	}
	baseline, err := beginNativeLog(filepath.Dir(path), os.Getuid())
	if err != nil {
		t.Fatal(err)
	}
	defer baseline.close()
	facts, code := baseline.collectChecked(false, time.Now().Add(time.Second), "owned")
	if code != "initial" {
		t.Fatal("unsettled fixed predicate differs")
	}
	body, err := json.Marshal(facts)
	if err != nil || strings.Contains(string(body), "initial") || strings.Contains(string(body), "predicate") {
		t.Fatal("private reason entered public facts")
	}
	if actual, _ := baseline.collectChecked(false, time.Now().Add(time.Second), "owned"); actual != facts {
		t.Fatal("runtime refusal semantics changed")
	}
}

func TestNativeLogRefusalFrameIsClosed(t *testing.T) {
	const prefix = "UAP_VSCODE_NATIVE_LOG_REFUSAL_V1 "
	if frame := nativeLogRefusalFrame("logs-held-ctime"); string(frame) != prefix+"logs-held-ctime\n" {
		t.Fatal("fixed refusal frame differs")
	}
	for _, field := range []string{"stat", "stat-type", "identity", "mode", "size", "mtime", "links", "ctime", "uid", "gid", "identity-type", "none"} {
		if frame := nativeLogRefusalFrame(nativeLogCheck("leaf-held-" + field)); string(frame) != prefix+"leaf-held-"+field+"\n" {
			t.Fatal("fixed metadata predicate refused")
		}
	}
	for _, code := range []nativeLogCheck{"none", "PRIVATE_COOKIE", "/private/profile", "logs-held-ctime\nPRIVATE", "logs-held-ctime:PRIVATE", nativeLogCheck(strings.Repeat("x", 97))} {
		if frame := nativeLogRefusalFrame(code); len(frame) != 0 {
			t.Fatal("unadmitted text entered stderr frame")
		}
	}
}
