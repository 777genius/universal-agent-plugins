//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestGeminiCycleAssessmentRejectsUnknownsBeforeUnchangedShortcutAndGlobalDrift(t *testing.T) {
	for _, fault := range []string{"unknown", "unchanged-unknown", "temp-residue", "parent-mode", "unchanged-parent-mode", "leaf-mode", "retained-replacement", "retained-deletion", "parent-replacement", "alias", "hardlink", "fifo", "caller", "scanner", "expired", "foreign-selection", "no-source", "no-source-parent-touch"} {
		t.Run(fault, func(t *testing.T) {
			root, _, expected := geminiCycleAssessmentFixture(t)
			parent := filepath.Join(root, "security", "assessments")
			path := filepath.Join(parent, expected.Key+".json")
			ledger := map[string]geminiCycleAssessment{}
			if fault == "unchanged-unknown" {
				projectNativeRuntimeWrite(t, filepath.Join(parent, "unknown.json"), []byte("caller"), 0600)
			}
			if fault == "unchanged-parent-mode" {
				if err := os.Mkdir(parent, 0755); err != nil {
					t.Fatal(err)
				}
			}
			retained := fault == "retained-replacement" || fault == "retained-deletion" || fault == "parent-replacement" || fault == "no-source-parent-touch"
			if retained {
				geminiCycleAssessmentStore(t, root, expected)
				ledger[expected.Slot] = expected
			}
			before := geminiCycleTempTree(t, root)
			if !retained {
				geminiCycleAssessmentStore(t, root, expected)
			}
			ctx := context.Background()
			current := &expected
			switch fault {
			case "unknown", "temp-residue":
				name := "unknown.json"
				if fault == "temp-residue" {
					name = ".assessment-unknown.tmp"
				}
				projectNativeRuntimeWrite(t, filepath.Join(parent, name), []byte("unknown"), 0600)
			case "parent-mode", "leaf-mode", "no-source-parent-touch":
				target, mode := parent, os.FileMode(0755)
				switch fault {
				case "leaf-mode":
					target, mode = path, 0644
				case "no-source-parent-touch":
					mode, current = 0700, nil
				}
				if err := os.Chmod(target, mode); err != nil {
					t.Fatal(err)
				}
			case "retained-replacement":
				if err := os.Rename(path, filepath.Join(filepath.Dir(root), "retired-entry")); err != nil {
					t.Fatal(err)
				}
				projectNativeRuntimeWrite(t, path, expected.Body, 0600)
			case "retained-deletion", "alias", "fifo":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				switch fault {
				case "alias":
					if err := os.Symlink(filepath.Join(root, "caller", "keep"), path); err != nil {
						t.Fatal(err)
					}
				case "fifo":
					if err := unix.Mkfifo(path, 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "parent-replacement":
				if err := os.Rename(parent, filepath.Join(filepath.Dir(root), "retired-parent")); err != nil {
					t.Fatal(err)
				}
				projectNativeRuntimeWrite(t, path, expected.Body, 0600)
			case "hardlink":
				if err := os.Link(path, filepath.Join(root, "linked")); err != nil {
					t.Fatal(err)
				}
			case "caller", "scanner":
				target := filepath.Join(root, "caller", "keep")
				if fault == "scanner" {
					target = filepath.Join(root, "security", "lintai", "held")
				}
				if err := os.WriteFile(target, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "expired":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "foreign-selection":
				foreign := expected
				foreign.Key = "f" + expected.Key[1:]
				current = &foreign
			case "no-source":
				current = nil
			}
			after, err := geminiCycleTree(context.Background(), root)
			if fault == "alias" || fault == "hardlink" || fault == "fifo" {
				if err == nil || after != nil {
					t.Fatal("unsafe cache entry produced an accepted tree")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			beforeBytes, _ := json.Marshal(before)
			ledgerBytes, _ := json.Marshal(ledger)
			frame, err := geminiCycleAssessmentClosure(ctx, before, after, ledger, current)
			if fault == "caller" || fault == "scanner" {
				if err != nil || frame.globalUnchanged(true) == nil {
					t.Fatal("cache allowance hid unrelated global mutation", err)
				}
			} else if err == nil {
				t.Fatal("unproved cache frame accepted")
			}
			retainedBefore, _ := json.Marshal(before)
			retainedLedger, _ := json.Marshal(ledger)
			if !bytes.Equal(beforeBytes, retainedBefore) || !bytes.Equal(ledgerBytes, retainedLedger) {
				t.Fatal("rejected frame adopted a new baseline")
			}
		})
	}
}
func TestGeminiCycleAssessmentExactBytesRejectForeignSchemaAndSecurityFacts(t *testing.T) {
	for _, mutation := range []string{"source", "policy", "scanner", "report", "counts", "evidence", "type", "unknown", "duplicate", "trailing"} {
		t.Run(mutation, func(t *testing.T) {
			root, _, expected := geminiCycleAssessmentFixture(t)
			before := geminiCycleTempTree(t, root)
			geminiCycleAssessmentStore(t, root, expected)
			var value map[string]any
			if err := json.Unmarshal(expected.Body, &value); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "source":
				value["subject"].(map[string]any)["tree_digest"] = "sha256:foreign"
			case "policy":
				value["policy"].(map[string]any)["version"] = 1
			case "scanner":
				value["scanner"].(map[string]any)["version"] = "other"
			case "report":
				value["report_digest"] = "sha256:foreign"
			case "counts":
				value["counts"].(map[string]any)["blocking"] = 1
			case "evidence":
				value["evidence_source"] = "local_scan"
			case "type":
				value["scanned_files"] = true
			case "unknown":
				value["unknown"] = "private-value"
			}
			body, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "duplicate":
				body = append([]byte(`{"schema_version":1,`), expected.Body[1:]...)
			case "trailing":
				body = append(bytes.Clone(expected.Body), []byte(" {}")...)
			}
			if err := os.WriteFile(filepath.Join(root, "security", "assessments", expected.Key+".json"), body, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := geminiCycleAssessmentClosure(context.Background(), before, geminiCycleTempTree(t, root), nil, &expected); err == nil {
				t.Fatal("foreign serialized assessment accepted")
			}
		})
	}
}
