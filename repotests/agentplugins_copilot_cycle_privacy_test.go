//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

func copilotCyclePrivate(raw []byte) bool {
	marker := []byte("uap-copilot-private-")
	if len(raw) > 1<<20 || !utf8.Valid(raw) || bytes.Contains(raw, marker) {
		return true
	}
	budget := clineCyclePrivacyBudget{text: 4 * len(raw), nodes: 16384}
	return clineCycleCredentialJSON(raw, marker, 0, &budget)
}

func copilotCycleSafe(t *testing.T, raw []byte) {
	t.Helper()
	if copilotCyclePrivate(raw) {
		t.Fatal("Copilot cycle private or unbounded stream refused before capture")
	}
}
