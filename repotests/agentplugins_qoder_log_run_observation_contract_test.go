//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const qoderInertRunID = "2026-10-28T15-30-45-123Z-abc123-p1234"

func qoderRunFixture(t *testing.T, mutate func(map[string]any)) (string, string, []string, map[string]string, map[string]string) {
	t.Helper()
	root, link := qoderLogFixture(t, "run-private-target")
	if err := os.Mkdir(filepath.Join(root, "logs", "runs"), 0755); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(root, "settings.json"), []byte("ORIGINAL-PRIVATE-SETTINGS"), 0600)
	before := qoderDeltaSnapshot(t, root)
	cwd := filepath.Join(filepath.Dir(root), "owned-project")
	argv := []string{filepath.Join(filepath.Dir(root), "owned-client"), "plugins", "list", "--json", "--plugin-dir", filepath.Join(cwd, "owned-view")}
	manifest := map[string]any{"run_id": qoderInertRunID, "started_at": "2026-10-28T15:30:45.123Z", "pid": 1234, "ppid": 5678,
		"process_role": "main", "cli_version": "1.1.78", "node_version": "v24.0.0", "platform": "linux", "arch": "x64", "cwd": cwd,
		"project_id": "PRIVATE-PROJECT-ID", "argv": argv, "env_flags": map[string]any{"debug": false, "remote_worker": false, "ci": false},
		"log_files": map[string]any{"process": "qodercli.log"}}
	if mutate != nil {
		mutate(manifest)
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "logs", "runs", qoderInertRunID)
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(directory, "manifest.json"), body, 0644)
	projectNativeRuntimeWrite(t, filepath.Join(directory, "qodercli.log"), []byte("PRIVATE-LOG-BODY"), 0644)
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("runs/"+qoderInertRunID, link); err != nil {
		t.Fatal(err)
	}
	return root, cwd, argv, before, qoderDeltaSnapshot(t, root)
}

func qoderRunObservation(t *testing.T, root, cwd string, argv []string, before, after map[string]string) qoderLogRunObservation {
	t.Helper()
	old, _ := json.Marshal(before)
	current, _ := json.Marshal(after)
	observation := qoderObserveLogRun(context.Background(), root, cwd, argv, before, after)
	raw, err := json.Marshal(observation)
	if err != nil || len(raw) > 2048 {
		t.Fatal("run observation serialization bound")
	}
	for _, canary := range []string{"PRIVATE", root, cwd, argv[0], "env_flags", "project_id", "node_version", "argv\""} {
		if bytes.Contains(raw, []byte(canary)) {
			t.Fatal("run observation exposed private or unapproved data")
		}
	}
	now, _ := json.Marshal(before)
	stable, _ := json.Marshal(after)
	if !bytes.Equal(old, now) || !bytes.Equal(current, stable) || !reflect.DeepEqual(qoderDeltaSnapshot(t, root), after) {
		t.Fatal("run observation changed its maps or original profile")
	}
	return observation
}

func TestQoderLogRunObservationJoinsHeldManifestWithoutAdoptingProfile(t *testing.T) {
	root, cwd, argv, before, after := qoderRunFixture(t, nil)
	observation := qoderRunObservation(t, root, cwd, argv, before, after)
	body, err := os.ReadFile(filepath.Join(root, "logs", "runs", qoderInertRunID, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	run := observation.Run
	if observation.Status != "captured" || run == nil || run.RunID != qoderInertRunID || run.LatestTarget != "runs/"+qoderInertRunID ||
		run.ManifestBytes != len(body) || run.ManifestSHA256 != hex.EncodeToString(digest[:]) || run.PID != 1234 || run.PPID != 5678 ||
		run.PIDProvenance != "manifest-only" || !run.RunIDMatchesManifest || !run.CWDMatches || !run.ArgvMatches || !run.ExactAddedRunClosure || !run.PreexistingRecordsStable || reflect.DeepEqual(before, after) {
		t.Fatal("source-defined projection lost a join or adopted the changed profile")
	}
}

func TestQoderLogRunObservationRetainsOnlyFinitePrivateMismatchFacts(t *testing.T) {
	root, cwd, argv, before, after := qoderRunFixture(t, func(m map[string]any) {
		m["cwd"], m["argv"], m["pid"] = "PRIVATE-CWD", []string{"PRIVATE-ARGV"}, 1235
	})
	run := qoderRunObservation(t, root, cwd, argv, before, after).Run
	if run == nil || run.CWDMatches || run.ArgvMatches || run.RunIDMatchesManifest {
		t.Fatal("private mismatch was concealed")
	}
	projectNativeRuntimeWrite(t, filepath.Join(root, "logs", "runs", qoderInertRunID, "PRIVATE-extra"), nil, 0600)
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte("CHANGED-PRIVATE-SETTINGS"), 0600); err != nil {
		t.Fatal(err)
	}
	after = qoderDeltaSnapshot(t, root)
	run = qoderRunObservation(t, root, cwd, argv, before, after).Run
	if run == nil || run.ExactAddedRunClosure || run.PreexistingRecordsStable {
		t.Fatal("new or changed foreign records were exempted")
	}
}

func TestQoderLogRunObservationRefusesUnknownAndUnsafeManifestShapes(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"unknown":   func(m map[string]any) { m["PRIVATE-KEY"] = "PRIVATE-BODY" },
		"pid-type":  func(m map[string]any) { m["pid"] = "1234" },
		"pid-range": func(m map[string]any) { m["pid"] = int64(1) << 32 },
		"start":     func(m map[string]any) { m["started_at"] = "PRIVATE-TIME" },
		"run":       func(m map[string]any) { m["run_id"] = "PRIVATE-RUN" },
		"role":      func(m map[string]any) { m["process_role"] = "PRIVATE-ROLE" },
		"version":   func(m map[string]any) { m["cli_version"] = "PRIVATE-VERSION" },
		"flags":     func(m map[string]any) { m["env_flags"] = map[string]any{"PRIVATE-KEY": true} },
		"flag-type": func(m map[string]any) { m["env_flags"].(map[string]any)["debug"] = "PRIVATE-FLAG" },
		"logs":      func(m map[string]any) { m["log_files"] = map[string]any{"process": "PRIVATE-LOG"} },
		"argv-type": func(m map[string]any) { m["argv"] = []any{true} },
		"oversized": func(m map[string]any) { m["project_id"] = strings.Repeat("PRIVATE", 1000) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			root, cwd, argv, before, after := qoderRunFixture(t, mutate)
			observation := qoderRunObservation(t, root, cwd, argv, before, after)
			if observation.Status != "manifest-refused" || observation.Run != nil {
				t.Fatal("unknown/private manifest shape gained projection")
			}
		})
	}
}

func TestQoderLogRunObservationSourceGrammarAndDuplicateRefusals(t *testing.T) {
	for _, id := range []string{qoderInertRunID, "2026-10-28T15-30-45-123Z--p1", "2026-10-28T15-30-45-123+05-30-abc-p1234"} {
		if _, _, ok := qoderLogRunParts(id); !ok {
			t.Fatal("source run ID refused", id)
		}
	}
	for _, id := range []string{strings.ReplaceAll(qoderInertRunID, "abc123", "abcdefg"), strings.ToUpper(qoderInertRunID),
		strings.ReplaceAll(qoderInertRunID, "-28T", "-99T"), strings.ReplaceAll(qoderInertRunID, "-p1234", "-p01234"),
		strings.ReplaceAll(qoderInertRunID, "30-45", "30-60"), strings.ReplaceAll(qoderInertRunID, "30-45", "30-61"),
		strings.ReplaceAll(qoderInertRunID, "123Z", "123+24-00"), strings.ReplaceAll(qoderInertRunID, "123Z", "123+05-60"), "../PRIVATE", "PRIVATE"} {
		if _, _, ok := qoderLogRunParts(id); ok {
			t.Fatal("arbitrary source target admitted")
		}
	}
	for _, raw := range []string{`{"pid":1,"pid":2}`, `[]`, `null`, `{"PRIVATE":"` + strings.Repeat("x", 4096) + `"}`} {
		if _, ok := qoderLogRunManifest([]byte(raw), qoderInertRunID, "/owned", nil); ok {
			t.Fatal("duplicate/malformed/unbounded manifest admitted")
		}
	}
}

func TestQoderLogRunObservationRefusesAliasLinkAndHeldInventoryDrift(t *testing.T) {
	for _, kind := range []string{"latest-record", "manifest-hash", "preexisting-run", "hardlink", "symlink", "run-alias", "held-directory"} {
		t.Run(kind, func(t *testing.T) {
			root, cwd, argv, before, after := qoderRunFixture(t, nil)
			rel := filepath.Join("logs", "runs", qoderInertRunID, "manifest.json")
			path := filepath.Join(root, rel)
			want := "manifest-refused"
			switch kind {
			case "latest-record":
				after["logs/latest"], want = "PRIVATE", "profile-refused"
			case "manifest-hash":
				after[rel] = "file:644:1:" + strings.Repeat("a", 64)
			case "preexisting-run":
				before[filepath.Dir(rel)], want = "directory:20000000755", "layout-refused"
			case "hardlink":
				if err := os.Link(path, filepath.Join(filepath.Dir(path), "PRIVATE-hardlink")); err != nil {
					t.Fatal(err)
				}
				want = "profile-refused"
			case "symlink":
				if err := os.Rename(path, path+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+"-original", path); err != nil {
					t.Fatal(err)
				}
				want = "profile-refused"
			default:
				leaf, err := os.Lstat(filepath.Join(root, "logs", "latest"))
				if err != nil {
					t.Fatal(err)
				}
				link, err := qoderReadOriginalLogLink(root, leaf)
				if err != nil {
					t.Fatal(err)
				}
				dir := filepath.Dir(path)
				if err := os.Rename(dir, dir+"-original"); err != nil {
					t.Fatal(err)
				}
				if kind == "run-alias" {
					if err := os.Symlink(dir+"-original", dir); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Mkdir(dir, 0755); err != nil {
						t.Fatal(err)
					}
					for _, name := range []string{"manifest.json", "qodercli.log"} {
						if err := os.Rename(filepath.Join(dir+"-original", name), filepath.Join(dir, name)); err != nil {
							t.Fatal(err)
						}
					}
				}
				if _, err := qoderLogRunRead(root, link, after[rel]); err == nil {
					t.Fatal("held directory replacement accepted")
				}
				want = "profile-refused"
			}
			if kind == "hardlink" || kind == "symlink" {
				leaf, err := os.Lstat(filepath.Join(root, "logs", "latest"))
				if err != nil {
					t.Fatal(err)
				}
				link, err := qoderReadOriginalLogLink(root, leaf)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := qoderLogRunRead(root, link, after[rel]); err == nil {
					t.Fatal("unsafe manifest leaf accepted")
				}
			}
			observation := qoderObserveLogRun(context.Background(), root, cwd, argv, before, after)
			if observation.Status != want || observation.Run != nil {
				t.Fatal("unsafe observation gained projection or wrong refusal stage", kind, observation.Status)
			}
		})
	}
}
