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
	lock := filepath.Join(w.f.base.Root, "xdg-data", "devin", "cli", "plugins", "lock.json")
	if info, err := os.Lstat(lock); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0644 || info.Size() != 57 || nativeSHA(t, lock) != "cbce3a5464482eb4cfb7b152ad15183bcb5db5c5557c625ee0cec1edb582aa88" || w.f.protected[lock] != nativeSHA(t, lock) {
		t.Fatal("source-defined empty plugin lock seed differs from captured bytes or authority")
	}
	mcp, err := windOwnedTree(w.f.ctx, filepath.Join(w.f.base.Root, "xdg-data", "devin", "mcp"))
	if err != nil || len(mcp) != 2 || mcp["."].Kind != "directory" || mcp["oauth"].Kind != "directory" || mcp["."].Mode != 040755 || mcp["oauth"].Mode != 040755 {
		t.Fatal("source-defined MCP storage must begin as only two empty ordinary0755 directories")
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
	for _, fault := range []string{"config", "cache", "uuid", "plugin-lock", "plugin-lock-timestamp", "plugin-lock-inode", "oauth-file", "oauth-mode", "oauth-inode", "old-log", "old-log-mode", "log-dir", "unknown", "missing-old", "second-log", "size", "owner", "links", "lock-pid", "lock-mode", "lock-inode"} {
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
		case "plugin-lock", "plugin-lock-timestamp", "plugin-lock-inode":
			key = "xdg-data\x00devin/cli/plugins/lock.json"
		case "oauth-file", "oauth-mode", "oauth-inode":
			key = "xdg-data\x00devin/mcp/oauth"
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
		case "oauth-file":
			bad[key+"/unexpected.json"] = bad[windRuntimeLogs+"/devin_20261028-195926_5806.log"]
		case "oauth-mode":
			r.Mode = 040700
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
		case "log-dir", "lock-inode", "oauth-inode", "plugin-lock-inode":
			r.Inode++
		case "plugin-lock-timestamp":
			r.Modified[0]++
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

func TestWindsurfComponentRuntimeTelemetryRewritePreservesExactAuthorityAndBytes(t *testing.T) {
	w := windComponentInert(t)
	start := time.Date(2026, 10, 28, 19, 59, 26, 0, time.UTC)
	before := w.profile(t)
	windRuntimeAuthorLog(t, w, "5805")
	path := filepath.Join(w.f.base.Root, "xdg-cache", "devin", "telemetry_state.json")
	if err := os.WriteFile(path, []byte(windRuntimeTelemetryBody), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, start, start); err != nil {
		t.Fatal(err)
	}
	after := w.profile(t)
	if before[windRuntimeTelemetry].Modified == after[windRuntimeTelemetry].Modified || w.runtimeClosure(before, after, start, start.Add(time.Second)) != nil {
		t.Fatal("same-inode fixed-byte telemetry timestamp rewrite refused")
	}
	w.freezeRuntime(after)
	w.check(t)
	if w.f.protected[path] != windsurfConfigDigest([]byte(windRuntimeTelemetryBody)) {
		t.Fatal("native cache timestamp variation replaced the protected bytes")
	}
	for _, fault := range []string{"content", "size", "kind", "mode", "device", "inode", "owner", "group", "links", "missing", "unknown"} {
		bad := make(map[string]windProfileRecord, len(after))
		for key, record := range after {
			bad[key] = record
		}
		record := bad[windRuntimeTelemetry]
		switch fault {
		case "content":
			record.SHA256 = strings.Repeat("0", 64)
		case "size":
			record.Size--
		case "kind":
			record.Kind = "directory"
		case "mode":
			record.Mode = 0100600
		case "device":
			record.Device++
		case "inode":
			record.Inode++
		case "owner":
			record.UID++
		case "group":
			record.GID++
		case "links":
			record.Links = 2
		case "missing":
			delete(bad, windRuntimeTelemetry)
		case "unknown":
			bad["xdg-cache\x00devin/unknown"] = record
		}
		if fault != "missing" {
			bad[windRuntimeTelemetry] = record
		}
		if w.runtimeClosure(before, bad, start, start.Add(time.Second)) == nil {
			t.Fatal("telemetry timestamp exception widened exact cache authority", fault)
		}
	}
}

func TestWindsurfComponentRuntimeEmptyRegistryReplacementIsCanonicalAndTimed(t *testing.T) {
	w := windComponentInert(t)
	path := filepath.Join(w.f.base.Root, "xdg-data", "devin", "cli", "plugins", "lock.json")
	raw, err := projectNativeRuntimeRead(path, 57)
	if err != nil {
		t.Fatal(err)
	}
	before := w.profile(t)
	start := time.Now()
	// Advance the filesystem timestamp clock inside this inert command window.
	time.Sleep(20 * time.Millisecond)
	logs := filepath.Join(w.f.base.Root, "xdg-data", "devin", "cli", "logs")
	projectNativeRuntimeWrite(t, filepath.Join(logs, "devin_"+start.UTC().Format("20060102-150405")+"_5805.log"), []byte("inert private log bytes\n"), 0644)
	if err := os.WriteFile(filepath.Join(logs, ".log-maintenance.lock"), []byte("5805\n"), 0644); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, path+".tmp", raw, 0644)
	if err := os.Rename(path+".tmp", path); err != nil {
		t.Fatal(err)
	}
	end := time.Now()
	after := w.profile(t)
	closureErr := w.runtimeClosure(before, after, start, end)
	if before[windRuntimePluginLock].Inode == after[windRuntimePluginLock].Inode || before[windRuntimePlugins] == after[windRuntimePlugins] || closureErr != nil {
		t.Fatal("actual canonical atomic registry replacement refused", closureErr)
	}
	w.freezeRuntime(after)
	w.check(t)
	if w.f.protected[path] != windRuntimePluginLockSHA {
		t.Fatal("native empty registry replacement changed protected bytes")
	}
	for _, fault := range []string{"content", "old-content", "size", "kind", "mode", "device", "inode", "owner", "group", "links", "mtime-before", "mtime-after", "mtime-nsec", "ctime-before", "ctime-after", "ctime-nsec", "parent-inode", "parent-device", "parent-kind", "parent-mode", "parent-owner", "parent-group", "parent-links", "parent-size", "parent-hash", "parent-mtime", "parent-ctime", "unchanged-lock", "tmp", "unknown"} {
		badBefore, bad := map[string]windProfileRecord{}, map[string]windProfileRecord{}
		for key, value := range before {
			badBefore[key] = value
		}
		for key, value := range after {
			bad[key] = value
		}
		key := windRuntimePluginLock
		if strings.HasPrefix(fault, "parent-") {
			key = windRuntimePlugins
		}
		r := bad[key]
		early, late := start.Add(-time.Nanosecond), end.Add(time.Nanosecond)
		switch fault {
		case "content", "parent-hash":
			r.SHA256 = strings.Repeat("0", 64)
		case "old-content":
			old := badBefore[key]
			old.SHA256 = strings.Repeat("0", 64)
			badBefore[key] = old
		case "size", "parent-size":
			r.Size--
		case "kind", "parent-kind":
			r.Kind = "other"
		case "mode", "parent-mode":
			r.Mode--
		case "device", "parent-device":
			r.Device++
		case "inode":
			r.Inode = 0
		case "parent-inode":
			r.Inode++
		case "owner", "parent-owner":
			r.UID++
		case "group", "parent-group":
			r.GID++
		case "links", "parent-links":
			r.Links++
		case "mtime-before", "parent-mtime":
			r.Modified = [2]int64{early.Unix(), int64(early.Nanosecond())}
		case "mtime-after":
			r.Modified = [2]int64{late.Unix(), int64(late.Nanosecond())}
		case "mtime-nsec":
			r.Modified[1] = int64(time.Second)
		case "ctime-before":
			r.Changed = [2]int64{early.Unix(), int64(early.Nanosecond())}
		case "ctime-after", "parent-ctime":
			r.Changed = [2]int64{late.Unix(), int64(late.Nanosecond())}
		case "ctime-nsec":
			r.Changed[1] = -1
		case "unchanged-lock":
			r = before[windRuntimePluginLock]
		case "tmp", "unknown":
			bad[windRuntimePluginLock+"."+fault] = r
		}
		bad[key] = r
		if w.runtimeClosure(badBefore, bad, start, end) == nil {
			t.Fatal("native empty registry rule widened content/authority/time/membership", fault)
		}
	}
}

func TestWindsurfComponentRuntimeDoesNotAdoptMutationAfterValidatedFrame(t *testing.T) {
	for _, fault := range []string{"log", "uuid", "cache", "cache-timestamp", "plugin-lock", "plugin-lock-timestamp", "plugin-lock-inode", "oauth-file", "unknown"} {
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
		case "cache", "cache-timestamp":
			path = filepath.Join(w.f.base.Root, "xdg-cache", "devin", "telemetry_state.json")
		case "plugin-lock", "plugin-lock-timestamp", "plugin-lock-inode":
			path = filepath.Join(root, "plugins", "lock.json")
		case "oauth-file":
			path = filepath.Join(w.f.base.Root, "xdg-data", "devin", "mcp", "oauth", "late.json")
		case "unknown":
			path = filepath.Join(root, "unknown")
		}
		var err error
		switch fault {
		case "plugin-lock-inode":
			raw, readErr := projectNativeRuntimeRead(path, 57)
			if readErr != nil {
				t.Fatal(readErr)
			}
			projectNativeRuntimeWrite(t, path+".tmp", raw, 0644)
			err = os.Rename(path+".tmp", path)
		case "cache-timestamp", "plugin-lock-timestamp":
			err = os.Chtimes(path, start, start)
		default:
			err = os.WriteFile(path, []byte("late unvalidated bytes"), 0644)
		}
		if err != nil {
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
