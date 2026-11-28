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
		{"commit-shaped explicit ref", "acme/demo", sha, false},
		{"native commit-shaped explicit ref", "git+https://git.example/team/repo.git", sha, false},
		{"browser commit with explicit ref", "https://github.com/acme/demo/tree/" + sha + "/skills/review", sha, false},
		{"browser commit with explicit branch", "https://github.com/acme/demo/tree/" + sha + "/skills/review", "refs/heads/" + sha, false},
		{"raw commit", "https://raw.githubusercontent.com/acme/demo/" + sha + "/skills/review/SKILL.md", "", true},
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
