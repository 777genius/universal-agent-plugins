// Package prompt is the CLI-owned, presentation-independent interaction port.
package prompt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const SkippedClientsHeading = "Not available for automatic install"

var (
	ErrPromptCanceled    = errors.New("prompt canceled")
	ErrPromptUnavailable = errors.New("prompt unavailable; use --target and explicit flags")
	ErrPromptInputClosed = errors.New("prompt input closed before a complete answer")
)

type Prompter interface {
	SelectTargets(context.Context, TargetSelectionRequest) (TargetSelectionResult, error)
	Confirm(context.Context, ConfirmationRequest) (ConfirmationResult, error)
}
type TargetChoice struct {
	ID    domain.ClientID
	Label string
}
type TargetSelectionRequest struct {
	Choices       []TargetChoice
	DefaultIDs    []domain.ClientID
	SkippedLabels []string
}
type TargetSelectionResult struct{ IDs []domain.ClientID }
type ConfirmationRequest struct {
	Title   string
	Summary []string
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

// SkippedClientsNotice renders one compact, sanitized block while preserving
// the installer-owned reason for every unavailable detected client.
func SkippedClientsNotice(labels []string) string {
	seen := make(map[string]bool, len(labels))
	clean := make([]string, 0, len(labels))
	for _, label := range labels {
		label = SafeText(strings.TrimSpace(label))
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		clean = append(clean, label)
	}
	if len(clean) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(SkippedClientsHeading)
	b.WriteString(":\n")
	for _, label := range clean {
		b.WriteString("  - ")
		b.WriteString(label)
		b.WriteString("\n")
	}
	return b.String()
}
