package platformexec

import (
	"strings"
	"testing"
)

func TestDecodeOpenCodeDefaultAgentFieldRejectsBlankString(t *testing.T) {
	t.Parallel()

	err := decodeOpenCodeDefaultAgentField(map[string]any{"default_agent": "   "}, &importedOpenCodeConfig{})
	if err == nil || !strings.Contains(err.Error(), "must be a non-empty string") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeOpenCodeInstructionsFieldRejectsBlankEntry(t *testing.T) {
	t.Parallel()

	err := decodeOpenCodeInstructionsField(map[string]any{"instructions": []any{"ok", "  "}}, &importedOpenCodeConfig{})
	if err == nil || !strings.Contains(err.Error(), "invalid entry at index 1") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractOpenCodeExtraRemovesManagedKeys(t *testing.T) {
	t.Parallel()

	extra := extractOpenCodeExtra(map[string]any{
		"plugin": []any{"demo"},
		"x":      true,
	})
	if len(extra) != 1 || extra["x"] != true {
		t.Fatalf("extra = %#v", extra)
	}
}
