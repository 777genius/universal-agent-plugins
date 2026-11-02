//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"strings"
	"testing"
)

func TestGeminiCycleAssessmentSelectionIsFiniteAndIndependent(t *testing.T) {
	selected := map[string]string{"add-user": "user", "add-a": "a", "add-b": "b", "repeat-add-a": "a", "unchanged-update-a": "a", "intact-repair-a": "a", "update-a": "a", "missing-repair-a": "a"}
	for _, label := range append(append([]string{}, geminiCyclePublic...), "foreign-add-a", "add", "add-a-extra", "repair-b", "version-user") {
		scope, source := selected[label]
		if geminiCycleSourceOperation(label) != source || source && geminiCycleScope(label) != scope {
			t.Fatal("source-cache authority escaped exact public label selection", label)
		}
	}
	_, recipe, expected := geminiCycleAssessmentFixture(t)
	for _, fault := range []string{"nil", "scope", "revision", "digest", "manifest"} {
		t.Run(fault, func(t *testing.T) {
			r := *recipe
			candidate := &r
			switch fault {
			case "nil":
				candidate = nil
			case "scope":
				r.Scope = "foreign"
			case "revision":
				r.Revision = "A-two"
			case "digest":
				r.Digest = "sha256:" + strings.Repeat("A", 64)
			case "manifest":
				r.Authored = nil
			}
			if _, err := geminiCycleAssessmentFor(candidate); err == nil {
				t.Fatal("unheld assessment recipe accepted")
			}
		})
	}
	ledger := map[string]geminiCycleAssessment{expected.Slot: expected}
	next, _, err := geminiCycleAssessmentLeaves(ledger, &expected)
	if err != nil {
		t.Fatal(err)
	}
	copyOfExpected := next[expected.Slot]
	copyOfExpected.Body[0] ^= 1
	if !bytes.Equal(ledger[expected.Slot].Body, expected.Body) || bytes.Equal(copyOfExpected.Body, expected.Body) {
		t.Fatal("candidate cache ledger aliases accepted bytes")
	}
	for _, fault := range []string{"slot", "key", "body", "rebound", "duplicate", "bound"} {
		t.Run(fault, func(t *testing.T) {
			held := map[string]geminiCycleAssessment{expected.Slot: expected}
			current := expected
			switch fault {
			case "slot":
				current.Slot = "foreign/U-one"
			case "key":
				current.Key = strings.Repeat("0", 63)
			case "body":
				current.Body = bytes.Repeat([]byte{'x'}, 4097)
			case "rebound":
				current.Key = strings.Repeat("0", 64)
			case "duplicate":
				current.Slot = "a/A-one"
			case "bound":
				for _, slot := range []string{"a/A-one", "b/B-one", "a/A-two", "foreign"} {
					entry := expected
					entry.Slot = slot
					held[slot] = entry
				}
			}
			if _, _, err := geminiCycleAssessmentLeaves(held, &current); err == nil {
				t.Fatal("foreign or rebound assessment ledger accepted")
			}
		})
	}
}
