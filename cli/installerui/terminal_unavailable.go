package installerui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

// Annotation decorates one option by ID without changing Option's public shape.
// Note is secondary text shown in parentheses; Info is a short informational
// addition shown after it. Both are display text and are sanitized.
type Annotation struct{ Note, Info string }

// Unavailable explains clients that cannot be selected. It is shown next to the
// list, never as a choice: no checkbox and no way to toggle it.
type Unavailable struct {
	Names  []string
	Detail string
}

const (
	maxUnavailable      = 32
	unavailableIndent   = "  "
	unavailableContinue = "    "
)

func normalizeAnnotations(annotations map[string]Annotation, options map[string]bool) (map[string]Annotation, error) {
	if len(annotations) == 0 {
		return nil, nil
	}
	result := make(map[string]Annotation, len(annotations))
	for id, annotation := range annotations {
		if !options[id] {
			return nil, fmt.Errorf("%w: annotation for unknown option", ErrInvalidRequest)
		}
		note, err := displayField(annotation.Note, false)
		if err != nil {
			return nil, err
		}
		info, err := displayField(annotation.Info, false)
		if err != nil {
			return nil, err
		}
		result[id] = Annotation{Note: strings.TrimSpace(note), Info: strings.TrimSpace(info)}
	}
	return result, nil
}

func normalizeUnavailable(heading string, items []Unavailable) (string, []Unavailable, error) {
	if len(items) == 0 {
		return "", nil, nil
	}
	if len(items) > maxUnavailable {
		return "", nil, fmt.Errorf("%w: too many unavailable entries", ErrInvalidRequest)
	}
	heading, err := displayField(heading, true)
	if err != nil {
		return "", nil, err
	}
	result := make([]Unavailable, 0, len(items))
	for _, item := range items {
		if len(item.Names) == 0 || len(item.Names) > maxUnavailable {
			return "", nil, fmt.Errorf("%w: unavailable entry names", ErrInvalidRequest)
		}
		detail, err := displayField(item.Detail, false)
		if err != nil || strings.TrimSpace(detail) == "" {
			return "", nil, fmt.Errorf("%w: unavailable entry detail", ErrInvalidRequest)
		}
		names := make([]string, len(item.Names))
		for i, name := range item.Names {
			if names[i], err = displayField(name, false); err != nil || strings.TrimSpace(names[i]) == "" {
				return "", nil, fmt.Errorf("%w: unavailable entry name", ErrInvalidRequest)
			}
		}
		result = append(result, Unavailable{Names: names, Detail: detail})
	}
	return strings.TrimSpace(heading), result, nil
}

// annotatedLabel appends the secondary note and informational text to a label.
func annotatedLabel(label string, annotation Annotation, theme terminaltheme.Theme) string {
	if annotation.Note != "" {
		label += " " + theme.Text(terminaltheme.Muted, "("+annotation.Note+")")
	}
	if annotation.Info != "" {
		label += " - " + theme.Text(terminaltheme.Info, annotation.Info)
	}
	return label
}

// AnnotatedLabel is the plain-text form used by line-oriented fallbacks.
func AnnotatedLabel(label string, annotation Annotation) string {
	return annotatedLabel(label, annotation, terminaltheme.Theme{})
}

type styledWord struct {
	text string
	role terminaltheme.Role
}

// unavailableLines renders the heading and one wrapped entry per explanation.
// A width <= 0 disables wrapping. Words are never split unless one alone is
// wider than the line.
func unavailableLines(heading string, items []Unavailable, width int, theme terminaltheme.Theme) []string {
	if len(items) == 0 {
		return nil
	}
	title := heading + ":"
	if width > 0 {
		title = ansi.Truncate(title, width, "…")
	}
	lines := []string{theme.Text(terminaltheme.Warning, title)}
	for _, item := range items {
		var words []styledWord
		for _, w := range strings.Fields(strings.Join(item.Names, ", ")) {
			words = append(words, styledWord{w, terminaltheme.Warning})
		}
		words[len(words)-1].text += ":"
		for _, w := range strings.Fields(item.Detail) {
			words = append(words, styledWord{w, terminaltheme.Muted})
		}
		lines = append(lines, wrapStyled(words, width, theme)...)
	}
	return lines
}

func wrapStyled(words []styledWord, width int, theme terminaltheme.Theme) []string {
	var lines []string
	line, used := unavailableIndent, len(unavailableIndent)
	empty := true
	flush := func() {
		lines = append(lines, line)
		line, used, empty = unavailableContinue, len(unavailableContinue), true
	}
	for _, word := range words {
		pieces := []string{word.text}
		if width > 0 {
			if avail := width - len(unavailableContinue); avail >= 1 && ansi.StringWidth(word.text) > avail {
				pieces = strings.Split(ansi.Hardwrap(word.text, avail, true), "\n")
			}
		}
		for _, piece := range pieces {
			w := ansi.StringWidth(piece)
			if !empty && width > 0 && used+1+w > width {
				flush()
			}
			if !empty {
				line += " "
				used++
			}
			line += theme.Text(word.role, piece)
			used += w
			empty = false
		}
	}
	return append(lines, line)
}

// unavailableCount is the number of clients behind the entries.
func unavailableCount(items []Unavailable) int {
	n := 0
	for _, item := range items {
		n += len(item.Names)
	}
	return n
}

// collapsedUnavailable is the one-line form used when the terminal is too short.
func collapsedUnavailable(items []Unavailable, theme terminaltheme.Theme) string {
	return theme.Text(terminaltheme.Warning, fmt.Sprintf("%d not available (terminal too short to show why)", unavailableCount(items)))
}

func plainUnavailable(heading string, items []Unavailable, theme terminaltheme.Theme) string {
	lines := unavailableLines(heading, items, 0, theme)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// annotatedRequest returns the line-oriented view of a request: option labels
// carry their plain annotations.
func annotatedRequest(req MultiSelectRequest, theme terminaltheme.Theme) SelectRequest {
	view := req.SelectRequest
	view.Options = append([]Option(nil), req.Options...)
	for i, option := range view.Options {
		if annotation, ok := req.Annotations[option.ID]; ok {
			view.Options[i].Label = annotatedLabel(option.Label, annotation, theme)
		}
	}
	return view
}
