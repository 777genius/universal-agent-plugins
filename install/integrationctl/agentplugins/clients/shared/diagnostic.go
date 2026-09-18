package shared

import "strings"

// nativeDiagnosticLimit caps how much of a failed native CLI invocation's own
// output is carried into a diagnostic. It exists purely for operator diagnosis
// of a discovery failure and is never treated as proof of package absence.
const nativeDiagnosticLimit = 2048

// BoundedNativeDiagnostic returns a short, sanitized excerpt of a failed native
// CLI invocation's own stdout/stderr. It never includes argv, environment, or
// configuration; only the bounded child-process output, with control characters
// stripped and length capped.
func BoundedNativeDiagnostic(stdout, stderr []byte) string {
	parts := make([]string, 0, 2)
	if text := sanitizeNativeDiagnosticText(stderr); text != "" {
		parts = append(parts, text)
	}
	if text := sanitizeNativeDiagnosticText(stdout); text != "" {
		parts = append(parts, text)
	}
	text := strings.Join(parts, " | ")
	if len(text) > nativeDiagnosticLimit {
		// Re-validate after the byte-index cut: it may have split a multibyte
		// rune, which strings.ToValidUTF8 would otherwise leave as U+FFFD.
		text = strings.ToValidUTF8(text[:nativeDiagnosticLimit], "") + "...(truncated)"
	}
	return text
}

func sanitizeNativeDiagnosticText(output []byte) string {
	text := strings.ToValidUTF8(string(output), "")
	text = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return ' '
		case r < 0x20 || r == 0x7f:
			return -1
		default:
			return r
		}
	}, text)
	return strings.Join(strings.Fields(text), " ")
}
