//go:build !windows

package opencode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeTransitionPrivacyRejectsExposedModes(t *testing.T) {
	for _, directory := range []bool{true, false} {
		name, mode := "journal", os.FileMode(0640)
		if directory {
			name, mode = "root", 0750
		}
		t.Run(name, func(t *testing.T) {
			_, root := privateTransitionJournalFixture(t, false)
			path := root
			if !directory {
				path = filepath.Join(root, transitionRecordFile)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			if _, err := readTransitionRecord(root); err == nil {
				t.Fatal("exposed transition was accepted")
			}
		})
	}
}
