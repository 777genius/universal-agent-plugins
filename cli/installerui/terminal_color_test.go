package installerui

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRichNoColor(t *testing.T) {
	var out strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := (testRich{Input: strings.NewReader("\r"), Output: &out, NoColor: true}).Confirm(ctx, ConfirmRequest{Title: "Apply fixture?", Summary: []string{"Fixture summary", "", "Selected units", "  unit=alpha"}})
	if err != nil || result.Accepted {
		t.Fatal(result, err)
	}
	if regexp.MustCompile(`\x1b\[[0-9;]*m`).MatchString(out.String()) {
		t.Fatalf("SGR in color-disabled rich prompt: %q", out.String())
	}
}
