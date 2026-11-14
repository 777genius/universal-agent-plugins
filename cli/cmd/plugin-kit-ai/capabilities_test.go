package main

import (
	"strings"
	"testing"
)

func TestCapabilitiesHelpMentionsGeminiRuntimeView(t *testing.T) {
	if !strings.Contains(capabilitiesCmd.Long, "Claude, Codex, and Gemini") {
		t.Fatalf("capabilities help text missing Gemini runtime support:\n%s", capabilitiesCmd.Long)
	}
}
