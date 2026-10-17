package platformmeta

import (
	"reflect"
	"testing"
)

func TestIDsReturnStableProfileOrder(t *testing.T) {
	t.Parallel()

	want := []string{
		"claude",
		"codex-package",
		"codex-runtime",
		"gemini",
		"cursor",
		"cursor-workspace",
		"opencode",
		"grok",
		"kimi",
	}
	if got := IDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs() = %#v, want %#v", got, want)
	}
}

func TestLookupNormalizesPlatformName(t *testing.T) {
	t.Parallel()

	profile, ok := Lookup("  GeMiNi  ")
	if !ok {
		t.Fatal("Lookup returned !ok for normalized name")
	}
	if profile.ID != "gemini" {
		t.Fatalf("Lookup().ID = %q, want gemini", profile.ID)
	}
}
