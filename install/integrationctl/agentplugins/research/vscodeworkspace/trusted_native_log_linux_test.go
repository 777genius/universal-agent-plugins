//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
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
	if baseline.collect(false, deadline, "owned").Status != "unavailable" || baseline.collect(true, time.Now(), "owned").Status != "unavailable" {
		t.Fatal("unsettled/late read admitted")
	}
	if facts := baseline.collect(true, deadline, "owned"); facts.Status != "observed" || !facts.TransportStart {
		t.Fatalf("settled projection %+v", facts)
	}
	if err := os.Link(leaf, filepath.Join(profile, "alias")); err != nil {
		t.Fatal(err)
	}
	if baseline.collect(true, deadline, "owned").Status != "unavailable" {
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
	if baseline.collect(true, deadline, "owned").Status != "unavailable" {
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
	if baseline.collect(true, time.Now().Add(time.Second), "owned").Status != "unavailable" {
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
	if baseline.collect(true, time.Now().Add(time.Second), "owned").Status != "unavailable" {
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
	if baseline.collect(true, time.Now().Add(time.Second), "owned").Status != "unavailable" {
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
