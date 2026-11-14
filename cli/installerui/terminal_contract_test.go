package installerui

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type forbiddenTerminalInput struct{ t *testing.T }

func (r forbiddenTerminalInput) Read([]byte) (int, error) {
	r.t.Fatal("invalid request read input")
	return 0, io.EOF
}

func TestTerminalConfirmationSummaryBudget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		req     ConfirmRequest
		invalid bool
	}{
		{"long-scope", ConfirmRequest{Title: "Apply?", Summary: []string{strings.Repeat("a", 1000) + "/chosen-suffix"}}, false},
		{"invalid-utf8", ConfirmRequest{Title: "Apply?", Summary: []string{string([]byte{255})}}, true},
		{"row-size", ConfirmRequest{Title: "Apply?", Summary: []string{strings.Repeat("a", 4097)}}, true},
		{"row-count", ConfirmRequest{Title: "Apply?", Summary: make([]string, 129)}, true},
		{"total-budget", ConfirmRequest{Title: "Apply?", Summary: strings.Split(strings.Repeat(strings.Repeat("a", 4096)+"\n", 16), "\n")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			var input io.Reader = strings.NewReader("n\n")
			if tc.invalid {
				input = forbiddenTerminalInput{t}
			}
			p := testPlain{Input: input, Output: &out}
			got, err := p.Confirm(context.Background(), tc.req)
			if got.Accepted || errors.Is(err, ErrInvalidRequest) != tc.invalid {
				t.Fatalf("%+v %v", got, err)
			}
			if tc.invalid && out.Len() != 0 {
				t.Fatal("invalid request rendered")
			}
			if !tc.invalid && !strings.Contains(out.String(), "/chosen-suffix") {
				t.Fatal("authority suffix clipped")
			}
		})
	}
}

func TestConsentSnapshotOverflow(t *testing.T) {
	input := strings.NewReader("yes\nnext-owner\n")
	_, _, err := readQueuedInput(context.Background(), input, 4097)
	if err == nil || !strings.Contains(err.Error(), "consent input budget exceeded") || input.Len() != len("yes\nnext-owner\n") {
		t.Fatalf("overflow consumed suffix: %v remaining=%d", err, input.Len())
	}
	// Exact cap remains valid and stops at the first submission.
	input = strings.NewReader(strings.Repeat("a", 4095) + "\nnext-owner\n")
	got, submitted, err := readQueuedInput(context.Background(), input, 4096)
	if err != nil || len(got) != 4096 || !submitted || input.Len() != len("next-owner\n") {
		t.Fatalf("cap boundary %d %v %v", len(got), submitted, err)
	}
}

func TestTerminalRequestValidationBeforeIO(t *testing.T) {
	base := MultiSelectRequest{SelectRequest: SelectRequest{Title: "Choose", Options: []Option{{"alpha", "Alpha"}, {"beta", "Beta"}}}}
	for _, tc := range []struct {
		name   string
		change func(*MultiSelectRequest)
	}{
		{"reserved-case", func(r *MultiSelectRequest) { r.Options[0].ID = "ALL" }},
		{"duplicate-default", func(r *MultiSelectRequest) { r.Defaults = []string{"alpha", "alpha"} }},
		{"unknown-default", func(r *MultiSelectRequest) { r.Defaults = []string{"absent"} }},
		{"invalid-min", func(r *MultiSelectRequest) { r.MinSelected = 3 }},
		{"hostile-empty-label", func(r *MultiSelectRequest) { r.Options[0].Label = "\x1b\u202e" }},
		{"invalid-utf8", func(r *MultiSelectRequest) { r.Options[0].Label = string([]byte{255}) }},
		{"long-title", func(r *MultiSelectRequest) { r.Title = strings.Repeat("a", 513) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.Options = append([]Option(nil), base.Options...)
			tc.change(&req)
			var out strings.Builder
			p := testPlain{Input: forbiddenTerminalInput{t}, Output: &out}
			got, err := p.SelectMany(context.Background(), req)
			if !errors.Is(err, ErrInvalidRequest) || got.Accepted || out.Len() != 0 {
				t.Fatalf("%+v %v output=%q", got, err, out.String())
			}
		})
	}
}
