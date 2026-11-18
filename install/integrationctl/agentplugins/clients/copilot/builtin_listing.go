package copilot

import (
	"regexp"
	"strings"
)

// Copilot prints plugins bundled with the CLI in a separate section of
// `copilot plugin list`. Their names, versions and count change with Copilot
// releases: 1.0.103 listed only browser-use v0.0.4, while 1.0.112 lists
// computer-use v0.1.97 and browser-use v0.1.0. Pinning one observed tuple made
// every user-scope install fail closed after a routine Copilot update, so the
// strict section grammar is the contract instead.
//
// A builtin entry has no `@marketplace` identity, so it can neither prove nor
// disprove a managed `name@marketplace` plugin. Only its name matters: callers
// refuse a declared plugin name that equals a listed builtin.
const builtinListingHeader = "Built-in plugins:"

// maxBuiltinListingEntries bounds one accepted builtin section.
const maxBuiltinListingEntries = 32

var builtinListingEntry = regexp.MustCompile(`^  • ([A-Za-z0-9][A-Za-z0-9._-]*) \(v[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?\) \[(?:enabled|disabled)\]$`)

// builtinListing is a `plugin list` document split around its builtin section.
type builtinListing struct {
	// rest is the document without the builtin section, still LF-terminated.
	rest string
	// leading reports that the section preceded rest; otherwise it followed it.
	leading bool
	names   []string
}

// lists reports whether name equals a listed builtin, ignoring case.
func (listing builtinListing) lists(name string) bool {
	for _, builtin := range listing.names {
		if strings.EqualFold(builtin, name) {
			return true
		}
	}
	return false
}

// splitBuiltinListing removes exactly one well-formed builtin section that
// starts or ends an LF-terminated document. Any other shape (no or several
// headers, an empty, oversized or malformed section, duplicate builtin names,
// a section in the middle of the document, CR, ANSI or NUL bytes) is reported
// as not split so callers keep their fail-closed handling.
func splitBuiltinListing(document string) (builtinListing, bool) {
	if strings.Count(document, builtinListingHeader) != 1 || !strings.HasSuffix(document, "\n") {
		return builtinListing{}, false
	}
	lines := strings.Split(strings.TrimSuffix(document, "\n"), "\n")
	header := -1
	for index, line := range lines {
		if line == builtinListingHeader {
			header = index
			break
		}
	}
	if header < 0 {
		return builtinListing{}, false
	}
	end := header + 1
	names := []string{}
	for ; end < len(lines); end++ {
		match := builtinListingEntry.FindStringSubmatch(lines[end])
		if match == nil {
			break
		}
		if len(names) == maxBuiltinListingEntries || (builtinListing{names: names}).lists(match[1]) {
			return builtinListing{}, false
		}
		names = append(names, match[1])
	}
	if len(names) == 0 {
		return builtinListing{}, false
	}
	switch {
	case header == 0:
		return builtinListing{rest: joinListingLines(lines[end:]), leading: true, names: names}, true
	case end == len(lines):
		return builtinListing{rest: joinListingLines(lines[:header]), names: names}, true
	default:
		return builtinListing{}, false
	}
}

func joinListingLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
