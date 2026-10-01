package installerui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func displayField(s string, title bool) (string, error) {
	if len(s) > 4096 || !utf8.ValidString(s) {
		return "", fmt.Errorf("%w: invalid display text", ErrInvalidRequest)
	}
	s = clean(s)
	if title && utf8.RuneCountInString(s) > 512 {
		return "", fmt.Errorf("%w: title exceeds 512 runes", ErrInvalidRequest)
	}
	if !title {
		s = clipLabel(s)
	}
	return s, nil
}

func clipLabel(s string) string {
	if utf8.RuneCountInString(s) <= 512 {
		return s
	}
	return string([]rune(s)[:512]) + "…"
}

func validTerminalID(id string) bool {
	if id == "" || len(id) > 128 || !asciiLetter(id[0]) {
		return false
	}
	switch strings.ToLower(id) {
	case "all", "none", "q", "quit", "cancel", "c":
		return false
	}
	for _, c := range []byte(id) {
		if !asciiLetter(c) && (c < '0' || c > '9') && !strings.ContainsRune("_.:-", rune(c)) {
			return false
		}
	}
	return true
}

func asciiLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func normalizeMulti(req MultiSelectRequest) (MultiSelectRequest, error) {
	if len(req.Defaults) > len(req.Options) {
		return MultiSelectRequest{}, fmt.Errorf("%w: too many defaults", ErrInvalidRequest)
	}
	if len(req.Options) < 1 || len(req.Options) > 64 || req.MinSelected < 0 || req.MinSelected > len(req.Options) {
		return MultiSelectRequest{}, fmt.Errorf("%w: options/minimum out of range", ErrInvalidRequest)
	}
	total := len(req.Title)
	var err error
	req.Title, err = displayField(req.Title, true)
	if err != nil || strings.TrimSpace(req.Title) == "" {
		return MultiSelectRequest{}, fmt.Errorf("%w: selection title", ErrInvalidRequest)
	}
	options := make([]Option, len(req.Options))
	seen := make(map[string]bool, len(options))
	for i, option := range req.Options {
		total += len(option.Label)
		if !validTerminalID(option.ID) || seen[option.ID] {
			return MultiSelectRequest{}, fmt.Errorf("%w: invalid/duplicate option id", ErrInvalidRequest)
		}
		seen[option.ID] = true
		option.Label, err = displayField(option.Label, false)
		if err != nil || strings.TrimSpace(option.Label) == "" {
			return MultiSelectRequest{}, fmt.Errorf("%w: option label", ErrInvalidRequest)
		}
		options[i] = option
	}
	if total > 64*1024 {
		return MultiSelectRequest{}, fmt.Errorf("%w: display exceeds 64 KiB", ErrInvalidRequest)
	}
	defaults := make(map[string]bool, len(req.Defaults))
	for _, id := range req.Defaults {
		if !seen[id] || defaults[id] {
			return MultiSelectRequest{}, fmt.Errorf("%w: invalid/duplicate default", ErrInvalidRequest)
		}
		defaults[id] = true
	}
	req.Options = options
	req.Defaults = canonicalIDs(options, defaults)
	return req, nil
}

func normalizeConfirm(req ConfirmRequest) (ConfirmRequest, error) {
	total := len(req.Title)
	title, err := displayField(req.Title, true)
	if err != nil || strings.TrimSpace(title) == "" {
		return ConfirmRequest{}, fmt.Errorf("%w: confirmation title", ErrInvalidRequest)
	}
	if len(req.Summary) > 128 {
		return ConfirmRequest{}, fmt.Errorf("%w: summary exceeds 128 rows", ErrInvalidRequest)
	}
	summary := make([]string, len(req.Summary))
	for i, row := range req.Summary {
		total += len(row)
		if len(row) > 4096 || !utf8.ValidString(row) {
			return ConfirmRequest{}, fmt.Errorf("%w: invalid summary row", ErrInvalidRequest)
		}
		summary[i] = clean(row) // Authority rows are never clipped.
	}
	if total > 64*1024 {
		return ConfirmRequest{}, fmt.Errorf("%w: display exceeds 64 KiB", ErrInvalidRequest)
	}
	req.Title, req.Summary = title, summary
	return req, nil
}

func canonicalIDs(options []Option, selected map[string]bool) []string {
	ids := make([]string, 0, len(selected))
	for _, option := range options {
		if selected[option.ID] {
			ids = append(ids, option.ID)
		}
	}
	return ids
}

func terminalSelection(req MultiSelectRequest, ids []string) (Selection, error) {
	if len(ids) < req.MinSelected {
		return Selection{}, fmt.Errorf("%w: select at least %d", ErrInvalidSelection, req.MinSelected)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		found := false
		for _, option := range req.Options {
			if option.ID == id {
				found = true
				break
			}
		}
		if !found || seen[id] {
			return Selection{}, fmt.Errorf("%w: unknown/duplicate selection", ErrInvalidSelection)
		}
		seen[id] = true
	}
	return Selection{IDs: canonicalIDs(req.Options, seen), Accepted: true}, nil
}
