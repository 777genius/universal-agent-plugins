//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func windRuntimeAuthorLog(t *testing.T, w *windComponentFixture, pid string) {
	t.Helper()
	root := filepath.Join(w.f.base.Root, "xdg-data", "devin", "cli", "logs")
	projectNativeRuntimeWrite(t, filepath.Join(root, "devin_20261028-195926_"+pid+".log"), []byte("inert private log bytes\n"), 0644)
	if err := os.WriteFile(filepath.Join(root, ".log-maintenance.lock"), []byte(pid+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestWindsurfComponentStartupSeedsBeforeBaselinePreserveSafetyAndSharedRecipe(t *testing.T) {
	w := windComponentInert(t)
	path := filepath.Join(w.f.base.Root, "xdg-config", "devin", "config.json")
	raw, err := projectNativeRuntimeRead(path, 4096)
	if err != nil || w.f.protected[path] != windsurfConfigDigest(raw) {
		t.Fatal("component initial raw config guard")
	}
	value, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	object := value.(map[string]any)
	if len(object) != 3 || object["version"] != json.Number("1") {
		t.Fatal("component current schema recipe differs")
	}
	delete(object, "version")
	_, original := projectNativeCLIUpdater("windsurf")
	expected, err := projectNativeRuntimeJSON(original)
	if err != nil || !reflect.DeepEqual(object, expected) || bytes.Contains(original, []byte("version")) {
		t.Fatal("component startup seeds changed safety or historical recipe")
	}
	cache := filepath.Join(w.f.base.Root, "xdg-cache", "devin", "telemetry_state.json")
	if nativeSHA(t, cache) != "7445ff8769cba2e1b004c754a770a85293e7548228e4fa3d363263c96c9b6290" {
		t.Fatal("fixed source telemetry seed differs from captured bytes")
	}
	w.check(t)
}

func TestWindsurfComponentRuntimeClosureFreezesPreviousLogsAndSeeds(t *testing.T) {
	w := windComponentInert(t)
	start := time.Date(2026, 10, 28, 19, 59, 26, 100000000, time.UTC)
	end := start.Add(200 * time.Millisecond)
	before := w.profile(t)
	windRuntimeAuthorLog(t, w, "5805")
	after := w.profile(t)
	if err := w.runtimeClosure(before, after, start, end); err != nil {
		t.Fatal("source-backed first inert log/maintenance pair refused", err)
	}
	w.freezeRuntime(after)
	w.check(t)
	before = after
	windRuntimeAuthorLog(t, w, "5806")
	after = w.profile(t)
	if err := w.runtimeClosure(before, after, start, end); err != nil {
		t.Fatal("source-backed repeated inert log/maintenance pair refused", err)
	}
	for _, fault := range []string{"config", "cache", "uuid", "old-log", "old-log-mode", "log-dir", "unknown", "missing-old", "second-log", "size", "owner", "links", "lock-pid", "lock-mode", "lock-inode"} {
		bad := make(map[string]windProfileRecord, len(after))
		for key, record := range after {
			bad[key] = record
		}
		key := "xdg-config\x00devin/config.json"
		switch fault {
		case "cache":
			key = "xdg-cache\x00devin/telemetry_state.json"
		case "uuid":
			key = "xdg-data\x00devin/cli/installation_id"
		case "old-log", "old-log-mode", "missing-old":
			key = windRuntimeLogs + "/devin_20261028-195926_5805.log"
		case "log-dir":
			key = windRuntimeLogs
		case "size", "owner", "links":
			key = windRuntimeLogs + "/devin_20261028-195926_5806.log"
		case "lock-pid", "lock-mode", "lock-inode":
			key = windRuntimeLock
		}
		r := bad[key]
		switch fault {
		case "unknown":
			bad["xdg-data\x00devin/cli/foreign"] = r
		case "second-log":
			bad[windRuntimeLogs+"/devin_20261028-195926_5807.log"] = bad[windRuntimeLogs+"/devin_20261028-195926_5806.log"]
		case "missing-old":
			delete(bad, key)
		case "size":
			r.Size = 1<<20 + 1
		case "owner":
			r.UID++
		case "links":
			r.Links = 2
		case "old-log-mode", "lock-mode":
			r.Mode = 0100600
		case "log-dir", "lock-inode":
			r.Inode++
		default:
			r.SHA256 = strings.Repeat("0", 64)
		}
		if fault != "unknown" && fault != "second-log" && fault != "missing-old" {
			bad[key] = r
		}
		if err := w.runtimeClosure(before, bad, start, end); err == nil {
			t.Fatal("runtime exception widened original profile closure", fault)
		}
	}
}

func TestWindsurfComponentRuntimeDoesNotAdoptMutationAfterValidatedFrame(t *testing.T) {
	for _, fault := range []string{"log", "uuid", "unknown"} {
		w := windComponentInert(t)
		start := time.Date(2026, 10, 28, 19, 59, 26, 0, time.UTC)
		before := w.profile(t)
		windRuntimeAuthorLog(t, w, "5805")
		after := w.profile(t)
		if err := w.runtimeClosure(before, after, start, start.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(w.f.base.Root, "xdg-data", "devin", "cli")
		path := filepath.Join(root, "logs", "devin_20261028-195926_5805.log")
		switch fault {
		case "uuid":
			path = filepath.Join(root, "installation_id")
		case "unknown":
			path = filepath.Join(root, "unknown")
		}
		if err := os.WriteFile(path, []byte("late unvalidated bytes"), 0644); err != nil {
			t.Fatal(err)
		}
		w.freezeRuntime(after)
		if windFrozenTrees(w.f.ctx, w.trees) == nil {
			t.Fatal("late filesystem observation replaced the accepted authority", fault)
		}
	}
}

func TestWindsurfComponentRuntimeCumulativeLogBoundPreservesTheSameNewPair(t *testing.T) {
	w := windComponentInert(t)
	start := time.Date(2026, 10, 28, 19, 59, 26, 0, time.UTC)
	before := w.profile(t)
	windRuntimeAuthorLog(t, w, "5805")
	after := w.profile(t)
	record := after[windRuntimeLogs+"/devin_20261028-195926_5805.log"]
	record.Size = 1 << 20
	for i := range 16 {
		key := windRuntimeLogs + "/devin_20261028-195926_" + strconv.Itoa(10000+i) + ".log"
		before[key], after[key] = record, record
	}
	if w.runtimeClosure(before, after, start, start.Add(time.Second)) == nil {
		t.Fatal("cumulative log bound ignored while the exact new pair stayed unchanged")
	}
	delete(before, windRuntimeLogs+"/devin_20261028-195926_10000.log")
	delete(after, windRuntimeLogs+"/devin_20261028-195926_10000.log")
	if err := w.runtimeClosure(before, after, start, start.Add(time.Second)); err != nil {
		t.Fatal("bounded cumulative ledger refused", err)
	}
}

func TestWindsurfComponentRuntimeLogGrammarWindowAndLockAreExact(t *testing.T) {
	start := time.Date(2026, 10, 28, 19, 59, 26, 100000000, time.UTC)
	end := start.Add(200 * time.Millisecond)
	for _, name := range []string{"devin_20261028-195926_5805.log", "devin_20261028-195925_5805.log", "devin_20261028-195927_5805.log", "devin_20260230-195926_5805.log", "devin_20261028-195926_05805.log", "devin_20261028-195926_0.log", "devin_20261028-195926_2147483648.log", "chisel_20261028-195926_5805.log", "devin_20261028-195926_5805.log.gz", "../devin_20261028-195926_5805.log"} {
		_, err := windRuntimePID(windRuntimeLogs+"/"+name, start, end)
		if (err == nil) != (name == "devin_20261028-195926_5805.log") {
			t.Fatal("native literal filename/calendar/PID/window differs", name, err)
		}
	}
	w := windComponentInert(t)
	before := w.profile(t)
	if w.runtimeClosure(before, before, start, end) == nil {
		t.Fatal("missing log/maintenance pair admitted")
	}
	windRuntimeAuthorLog(t, w, "5805")
	path := filepath.Join(w.f.base.Root, "xdg-data", "devin", "cli", "logs", ".log-maintenance.lock")
	if err := os.WriteFile(path, []byte("5806\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if w.runtimeClosure(before, w.profile(t), start, end) == nil {
		t.Fatal("different maintenance self-PID admitted")
	}
}
