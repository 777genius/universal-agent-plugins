package skillsource

import (
	"strings"
	"testing"
)

func TestRequestedGitIntentPreservesOriginalPinBeforeResolution(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, input, ref string
		pinned           bool
	}{
		{"shorthand commit", "acme/demo@" + sha, "", true},
		{"native Git commit", "git+https://git.example/team/repo.git#" + sha + "//skills/review", "", true},
		{"GitHub browser commit", "https://github.com/acme/demo/tree/" + sha + "/skills/review", "", true},
		{"explicit commit", "acme/demo", sha, true},
		{"default branch", "acme/demo", "", false},
		{"branch selector", "acme/demo@main", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git, err := RequestedGitIntent(tc.input, tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			pinned := git.Commit != ""
			if !pinned && git.Ref == "" && git.Browser != nil {
				segment, _, _ := strings.Cut(git.Browser.Suffix, "/")
				pinned = segment == sha
			}
			if pinned != tc.pinned {
				t.Fatalf("original pin misclassified: %+v", git)
			}
		})
	}
}
