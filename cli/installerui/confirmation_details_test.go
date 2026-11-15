package installerui

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConfirmationDetailsShareExistingAuthorityBudget(t *testing.T) {
	for _, req := range []confirmationReview{
		{ConfirmRequest: ConfirmRequest{Title: "Apply?", Summary: make([]string, 64)}, Details: make([]string, 65)},
		{ConfirmRequest: ConfirmRequest{Title: "Apply?"}, Details: []string{strings.Repeat("x", 4097)}},
		{ConfirmRequest: ConfirmRequest{Title: "Apply?"}, Details: []string{string([]byte{255})}},
		{ConfirmRequest: ConfirmRequest{Title: "Apply?", Summary: strings.Split(strings.Repeat(strings.Repeat("x", 4096)+"\n", 8), "\n")}, Details: strings.Split(strings.Repeat(strings.Repeat("x", 4096)+"\n", 8), "\n")},
	} {
		if _, err := normalizeConfirmationReview(req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("details bypassed authority budget: %v", err)
		}
	}
	path := strings.Repeat("x", 1000) + "/exact-destination"
	req := confirmationReview{ConfirmRequest: ConfirmRequest{Title: "Apply?", Summary: []string{"One Skill"}}, Details: []string{"\x1b" + path}}
	got, err := normalizeConfirmationReview(req)
	if err != nil || len(got.Details) != 1 || got.Details[0] != path {
		t.Fatalf("authority path was clipped or not cleaned: %+v %v", got, err)
	}
	got.Details[0] = "changed"
	if req.Details[0] != "\x1b"+path {
		t.Fatal("normalization mutated the caller's details")
	}
}

func TestPlainConfirmationDisplaysSummaryAndFullDetails(t *testing.T) {
	var output strings.Builder
	ui, err := New(Config{Input: strings.NewReader("n\n"), Output: &output})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ui.ConfirmWithDetails(context.Background(), ConfirmRequest{Title: "Apply?", Summary: []string{"One Skill for two agents"}, Default: true}, []string{"Claude: /exact/claude/path", "Codex: /exact/codex/path"})
	if err != nil || result.Accepted || !strings.Contains(output.String(), "One Skill for two agents\nClaude: /exact/claude/path\nCodex: /exact/codex/path\n") {
		t.Fatalf("plain authority details disappeared or changed consent: %+v %v %q", result, err, output.String())
	}
}
