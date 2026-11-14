package codex

import (
	"slices"
	"testing"
)

func TestCodexAuthorDoc_TrimsEmptyFields(t *testing.T) {
	t.Parallel()

	got := codexAuthorDoc(&author{
		Name:  " Example ",
		Email: " ",
		URL:   " https://example.com ",
	})
	if got["name"] != "Example" {
		t.Fatalf("name = %v", got["name"])
	}
	if got["url"] != "https://example.com" {
		t.Fatalf("url = %v", got["url"])
	}
	if _, ok := got["email"]; ok {
		t.Fatalf("email should be omitted: %+v", got)
	}
}

func TestCodexKeywords_DropsBlankEntries(t *testing.T) {
	t.Parallel()

	got := codexKeywords([]string{" codex ", "", " ", "plugin"})
	if !slices.Equal(got, []string{"codex", "plugin"}) {
		t.Fatalf("keywords = %v", got)
	}
}
