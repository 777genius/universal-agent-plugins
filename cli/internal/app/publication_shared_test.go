package app

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func TestOrderedPublicationChannelsPreservePreferredFamilies(t *testing.T) {
	t.Parallel()

	model := publicationmodel.Model{
		Channels: []publicationmodel.Channel{
			{Family: "gemini-gallery"},
			{Family: "custom-z"},
			{Family: "claude-marketplace"},
			{Family: "codex-marketplace"},
		},
	}

	got := orderedPublicationChannels(model)
	want := []string{"codex-marketplace", "claude-marketplace", "gemini-gallery", "custom-z"}
	if len(got) != len(want) {
		t.Fatalf("channels = %d want %d", len(got), len(want))
	}
	for i, family := range want {
		if got[i].Family != family {
			t.Fatalf("channels[%d] = %q want %q", i, got[i].Family, family)
		}
	}
}

func TestNormalizePackageRootDefaultsAndRejectsEscape(t *testing.T) {
	t.Parallel()

	got, err := normalizePackageRoot("", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got != "plugins/demo" {
		t.Fatalf("default root = %q", got)
	}
	if _, err := normalizePackageRoot("../escape", "demo"); err == nil {
		t.Fatal("expected escape error")
	}
}

func TestAppendUniquePublishStepsTrimsAndDedupes(t *testing.T) {
	t.Parallel()

	got := appendUniquePublishSteps([]string{"  first  ", "", "second", "first", " second "})
	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %#v want %#v", got, want)
	}
}
