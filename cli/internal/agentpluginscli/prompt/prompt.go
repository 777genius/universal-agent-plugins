// Package prompt is the CLI-owned, presentation-independent interaction port.
package prompt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	ErrPromptCanceled    = errors.New("prompt canceled")
	ErrPromptUnavailable = errors.New("an interactive terminal is required for this prompt; run the command in a terminal or pass the explicit flags it documents (for example --target <agent>)")
	ErrPromptInputClosed = errors.New("prompt input closed before a complete answer")
	ErrReviewTooLarge    = errors.New("confirmation review exceeds the terminal display budget")
)

type Prompter interface {
	SelectTargets(context.Context, TargetSelectionRequest) (TargetSelectionResult, error)
	Confirm(context.Context, ConfirmationRequest) (ConfirmationResult, error)
}
type TargetChoice struct {
	ID    domain.ClientID
	Label string
	// Note is optional secondary text shown after the label in parentheses
	// (for example "prepare only"). Info is an optional short informational
	// addition. Both are untrusted display text; adapters sanitize them.
	Note string
	Info string
}
type TargetSelectionRequest struct {
	// Title overrides the adapter's default heading, for example a retry step.
	Title      string
	Choices    []TargetChoice
	DefaultIDs []domain.ClientID
	// Skipped lists detected or known clients that cannot be selected. Adapters
	// show them as grouped explanations next to the list, never as choices.
	Skipped []SkippedTarget
}
type TargetSelectionResult struct{ IDs []domain.ClientID }
type ConfirmationRequest struct {
	Title   string
	Summary []string
	Details []string
	Default bool
}
type ConfirmationResult struct{ Accepted bool }

// ValidateSelection returns a fresh subset in the request's canonical order.
// Commands build choices in domain order; adapters never maintain an ID registry.
func ValidateSelection(r TargetSelectionRequest, ids []domain.ClientID) (TargetSelectionResult, error) {
	allowed := make(map[domain.ClientID]bool, len(r.Choices))
	for _, c := range r.Choices {
		if c.ID == "" || allowed[c.ID] {
			return TargetSelectionResult{}, fmt.Errorf("invalid target choices")
		}
		allowed[c.ID] = true
	}
	if len(ids) == 0 {
		return TargetSelectionResult{}, fmt.Errorf("select at least one target")
	}
	selected := make(map[domain.ClientID]bool, len(ids))
	for _, id := range ids {
		if !allowed[id] || selected[id] {
			return TargetSelectionResult{}, fmt.Errorf("invalid or duplicate target selection: %s", SafeText(string(id)))
		}
		selected[id] = true
	}
	result := TargetSelectionResult{}
	for _, c := range r.Choices {
		if selected[c.ID] {
			result.IDs = append(result.IDs, c.ID)
		}
	}
	return result, nil
}
func ValidateRequest(r TargetSelectionRequest) error {
	_, err := ValidateSelection(r, r.DefaultIDs)
	return err
}

// SafeText bounds untrusted display text and removes terminal/bidi controls.
func SafeText(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			continue
		}
		if n == 512 {
			b.WriteString("…")
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// NormalizeIOError preserves the CLI diagnostic sentinels at the host boundary.
func NormalizeIOError(err error) error {
	switch {
	case errors.Is(err, io.EOF):
		return normalizedIOError{sentinel: ErrPromptInputClosed, cause: err}
	case errors.Is(err, promptio.ErrUnavailable):
		return normalizedIOError{sentinel: ErrPromptUnavailable, cause: err}
	default:
		return err
	}
}

type normalizedIOError struct{ sentinel, cause error }

func (e normalizedIOError) Error() string   { return e.sentinel.Error() }
func (e normalizedIOError) Unwrap() []error { return []error{e.sentinel, e.cause} }
