package pluginkitairepo_test

import (
	"strings"
	"testing"
)

// Normalize only Windows line endings; retain the exact historical heading and
// terminating newline so current and preserved-v1 assertions stay separated.
func cutHistoricalReadme(readme string) (current, historical string, found bool) {
	return strings.Cut(strings.ReplaceAll(readme, "\r\n", "\n"), "### Historical authoring and development\n")
}

func TestHistoricalReadmeBoundaryLineEndings(t *testing.T) {
	const heading = "### Historical authoring and development"
	for _, newline := range []struct{ name, value string }{{"LF", "\n"}, {"CRLF", "\r\n"}} {
		for _, tc := range []struct {
			name, boundary string
			found          bool
		}{
			{"exact", heading + newline.value, true},
			{"missing", "", false},
			{"wrong_title", "### Historical authoring" + newline.value, false},
			{"wrong_level", "## Historical authoring and development" + newline.value, false},
			{"trailing_space", heading + " " + newline.value, false},
			{"suffix", heading + " extra" + newline.value, false},
			{"no_newline", heading, false},
			{"bare_CR", heading + "\r", false},
		} {
			t.Run(newline.name+"/"+tc.name, func(t *testing.T) {
				input := "current" + newline.value + tc.boundary + "historical" + newline.value
				current, historical, found := cutHistoricalReadme(input)
				if found != tc.found {
					t.Fatalf("boundary found = %v, want %v", found, tc.found)
				}
				if found {
					if current != "current\n" || historical != "historical\n" {
						t.Fatalf("incorrect sections: current=%q historical=%q", current, historical)
					}
				} else if current != strings.ReplaceAll(input, "\r\n", "\n") || historical != "" {
					t.Fatalf("missing boundary changed sections: current=%q historical=%q", current, historical)
				}
			})
		}
	}
}
