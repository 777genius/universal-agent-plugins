//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestTrustedManifestCompleteClosureHasSeparateArrayBound(t *testing.T) {
	_, manifest := trustedContractFixture(t)
	for len(manifest.Files) < 257 {
		manifest.Files = append(manifest.Files, trustedToolFile{Path: fmt.Sprintf("driver/node_modules/playwright-core/entry-%d.js", len(manifest.Files)), Bytes: 1, SHA256: strings.Repeat("a", 64)})
	}
	body, _ := json.Marshal(manifest)
	if _, err := uniqueJSON(body); err == nil {
		t.Fatal("legacy observation bound changed")
	}
	if _, err := decodeTrustedManifest(body); err != nil {
		t.Fatalf("complete pinned closure refused: %v", err)
	}
	for len(manifest.Files) < 4097 {
		manifest.Files = append(manifest.Files, trustedToolFile{Path: fmt.Sprintf("driver/node_modules/playwright-core/entry-%d.js", len(manifest.Files)), Bytes: 1, SHA256: strings.Repeat("a", 64)})
	}
	body, _ = json.Marshal(manifest)
	if _, err := decodeTrustedManifest(body); err == nil {
		t.Fatal("oversized closure accepted")
	}
}

func TestTrustedDeadlineKeepsOriginalOriginAndClosedMode(t *testing.T) {
	var now unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &now); err != nil {
		t.Fatal(err)
	}
	legacy := inputRequest{StartNS: now.Nano() - int64(100*time.Second)}
	deadline, err := inputDeadline(legacy, "work")
	if err != nil || time.Until(deadline) > 321*time.Second || time.Until(deadline) < 319*time.Second {
		t.Fatal("legacy budget changed", err)
	}
	in := legacy
	in.Mode = trustedProjectMode
	in.SourceTree = strings.Repeat("a", 40)
	in.Trusted = &trustedInput{ToolsRoot: "/tmp/tools", ToolsManifestSHA256: strings.Repeat("a", 64), InstallerPath: "/tmp/installer", InstallerSHA256: strings.Repeat("b", 64), ProbePath: "/tmp/probe", ProbeSHA256: strings.Repeat("c", 64), ScannerPath: "/tmp/scanner", ScannerSHA256: strings.Repeat("d", 64), ScannerArchiveSHA256: trustedScannerArchive}
	deadline, err = inputDeadline(in, "work")
	if err != nil || time.Until(deadline) > 741*time.Second || time.Until(deadline) < 739*time.Second {
		t.Fatal("trusted budget origin was reset", err)
	}
	for _, kind := range []string{"unknown-mode", "legacy-input", "observer-substitution", "scanner-pin", "future-origin", "unknown-phase"} {
		changed := in
		phase := "work"
		switch kind {
		case "unknown-mode":
			changed.Mode = "trusted-project-v2"
		case "legacy-input":
			changed.Mode = ""
		case "observer-substitution":
			changed.ObserverRoot = "/tmp/other"
		case "scanner-pin":
			copy := *in.Trusted
			copy.ScannerArchiveSHA256 = strings.Repeat("e", 64)
			changed.Trusted = &copy
		case "future-origin":
			changed.StartNS = now.Nano() + int64(time.Hour)
		case "unknown-phase":
			phase = "extended"
		}
		if _, err := inputDeadline(changed, phase); err == nil {
			t.Fatal("closed deadline admitted", kind)
		}
	}
}

func TestTrustedAtomicResultDoesNotBorrowLegacyReadyMarker(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "driver.json")
	if err := os.WriteFile(filepath.Join(root, "observation.ready"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := trustedReadyResult(path); err != nil || ready {
		t.Fatal("legacy marker supplied trusted result", err)
	}
	body := []byte(`{"type":"driver-result"}`)
	if err := os.WriteFile(path+".pending", body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := trustedReadyResult(path); err != nil || ready {
		t.Fatal("unpublished temporary admitted", err)
	}
	if err := os.Rename(path+".pending", path); err != nil {
		t.Fatal(err)
	}
	got, ready, err := trustedReadyResult(path)
	if err != nil || !ready || !bytes.Equal(body, got) {
		t.Fatal("atomic driver result unavailable", err)
	}
}

func TestTrustedProtectedInventoryDetectsEmptyDirectoriesModesAndAliases(t *testing.T) {
	for _, kind := range []string{"bytes", "mode", "empty-directory", "alias"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "owned")
			if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			before, err := trustedSnapshot(root)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "bytes":
				err = os.WriteFile(path, []byte("changed"), 0600)
			case "mode":
				err = os.Chmod(path, 0640)
			case "empty-directory":
				err = os.Mkdir(filepath.Join(root, "added"), 0700)
			case "alias":
				err = os.Symlink(path, filepath.Join(root, "alias"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := trustedUnchanged(before); err == nil {
				t.Fatal("changed protected tree accepted")
			}
		})
	}
}

func TestTrustedNativeProbeEventsRequireFreshOrderedIndependentJoin(t *testing.T) {
	started := time.Now().Add(-time.Second)
	ended := time.Now()
	facts := map[string]any{"nonce": "nonce", "revision": "a-one", "cwd": "/owned/package", "argv": []any{"a"}, "plugin_root": "/owned/package", "plugin_data": "/owned/data", "literal": "literal", "once": "once", "marker": "retained", "pid": json.Number("123"), "session": "123-fresh"}
	for _, kind := range []string{"valid", "foreign-session", "wrong-pid", "old-time", "wrong-facts", "missing-initialize", "out-of-order", "duplicate-call", "unknown-field"} {
		t.Run(kind, func(t *testing.T) {
			var body bytes.Buffer
			methods := []string{"initialize", "tools/list", "tools/call"}
			if kind == "missing-initialize" {
				methods = methods[1:]
			}
			if kind == "out-of-order" {
				methods = []string{"tools/list", "initialize", "tools/call"}
			}
			if kind == "duplicate-call" {
				methods = append(methods, "tools/call")
			}
			for _, method := range methods {
				event := map[string]any{"method": method, "pid": facts["pid"], "session": facts["session"], "nonce": facts["nonce"], "timestamp": started.Add(time.Millisecond).UTC().Format(time.RFC3339Nano), "facts": nil}
				if method == "tools/call" {
					event["facts"] = facts
				}
				switch kind {
				case "foreign-session":
					event["session"] = "other"
				case "wrong-pid":
					event["pid"] = 124
				case "old-time":
					event["timestamp"] = started.Add(-time.Second).UTC().Format(time.RFC3339Nano)
				case "wrong-facts":
					if method == "tools/call" {
						event["facts"] = map[string]any{"nonce": "wrong"}
					}
				case "unknown-field":
					event["extra"] = true
				}
				if err := json.NewEncoder(&body).Encode(event); err != nil {
					t.Fatal(err)
				}
			}
			err := validateTrustedProbeEvents(body.Bytes(), facts, started, ended)
			if (err == nil) != (kind == "valid") {
				t.Fatal(kind, err)
			}
		})
	}
}

func TestTrustedQualificationCannotPromoteMissingOrUnreapedStages(t *testing.T) {
	stages := []string{"empty-a", "empty-b", "installed-a", "installed-b", "stable-a", "updated-a", "repaired-a", "restart-a", "removed-a", "remaining-b"}
	labels := append([]string{"Xvfb", "git-a", "git-b", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a"}, stages...)
	for _, kind := range []string{"complete", "missing-stage", "missing-child", "unreaped", "run-failure", "cleanup-failure"} {
		c := &controller{trustedState: &trustedLifecycle{Complete: true, Stages: append([]string(nil), stages...)}}
		for _, label := range labels {
			c.children = append(c.children, childRecord{Label: label, TreeReaped: true})
		}
		var runErr, cleanupErr error
		switch kind {
		case "missing-stage":
			c.trustedState.Stages = stages[1:]
		case "missing-child":
			c.children = c.children[1:]
		case "unreaped":
			c.children[0].TreeReaped = false
		case "run-failure":
			runErr = fmt.Errorf("required native failure")
		case "cleanup-failure":
			cleanupErr = fmt.Errorf("cleanup failure")
		}
		summary := map[string]any{}
		c.trustedSummary(summary, runErr, cleanupErr)
		for _, qualified := range summary["qualification"].(map[string]bool) {
			if qualified != (kind == "complete") {
				t.Fatal("qualification differs", kind)
			}
		}
		if summary["activation"] != "prepared" || summary["user_native_install_qualified"] != false || summary["real_model_qualified"] != false || summary["vendor_authentication_qualified"] != false {
			t.Fatal("evidence boundary promoted")
		}
	}
}
