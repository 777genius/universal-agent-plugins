package prompt

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSkillNavigationLabelKeepsDistinctDeepPathSuffixes(t *testing.T) {
	prefix := "review · " + strings.Repeat("deep-folder/", 100)
	first := SkillNavigationLabel(prefix + "first/review")
	second := SkillNavigationLabel(prefix + "second/review")
	if first == second || !strings.HasSuffix(first, "first/review") || !strings.HasSuffix(second, "second/review") {
		t.Fatalf("deep paths lost identity: %q %q", first, second)
	}
	if utf8.RuneCountInString(first) != 128 || !strings.HasPrefix(first, "review · ") || !strings.Contains(first, "…") {
		t.Fatalf("navigation budget changed: %q", first)
	}
}
func TestSkillNavigationLabelSanitizesBeforeEliding(t *testing.T) {
	label := "\x1b" + strings.Repeat("x", 600) + "\u202e/private/final"
	got := SkillNavigationLabel(label)
	if strings.ContainsAny(got, "\x1b\u202e") || !strings.HasSuffix(got, "/private/final") {
		t.Fatalf("unsafe or shortened suffix: %q", got)
	}
	if got := SkillNavigationLabel("short · docs/review"); got != "short · docs/review" {
		t.Fatalf("ordinary label changed: %q", got)
	}
}
