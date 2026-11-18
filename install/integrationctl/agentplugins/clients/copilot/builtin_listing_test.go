package copilot

import (
	"reflect"
	"strings"
	"testing"
)

// Builtin sections captured from official Copilot CLI binaries in isolated
// empty profiles. 1.0.112 is the darwin-arm64 npm release; its builtin names,
// versions and count differ from 1.0.103.
const (
	copilot103BuiltinSection = "Built-in plugins:\n  • browser-use (v0.0.4) [disabled]\n"
	copilot112BuiltinSection = "Built-in plugins:\n  • computer-use (v0.1.97) [disabled]\n  • browser-use (v0.1.0) [disabled]\n"
	copilotEmptyListingBody  = "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n"
)

func TestSplitBuiltinListingAcceptsReleaseVariants(t *testing.T) {
	live := "Live Plugins (loaded from a local marketplace directory, never copied):\n  • demo@agentplugins-0123456789ab (v1.0.0) (enabled)\n      from /managed/demo\n"
	for _, test := range []struct {
		name, document, rest string
		leading              bool
		names                []string
	}{
		{"1.0.103 empty", copilotEmptyListingBody + copilot103BuiltinSection, copilotEmptyListingBody, false, []string{"browser-use"}},
		{"1.0.112 empty", copilotEmptyListingBody + copilot112BuiltinSection, copilotEmptyListingBody, false, []string{"computer-use", "browser-use"}},
		{"1.0.103 live", copilot103BuiltinSection + live, live, true, []string{"browser-use"}},
		{"1.0.112 live", copilot112BuiltinSection + live, live, true, []string{"computer-use", "browser-use"}},
		{"enabled builtin", strings.Replace(copilot103BuiltinSection, "[disabled]", "[enabled]", 1) + live, live, true, []string{"browser-use"}},
		{"prerelease builtin", "Built-in plugins:\n  • next (v2.0.0-rc.1) [enabled]\n" + live, live, true, []string{"next"}},
		{"only builtins", copilot112BuiltinSection, "", true, []string{"computer-use", "browser-use"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			listing, ok := splitBuiltinListing(test.document)
			if !ok || listing.rest != test.rest || listing.leading != test.leading || !reflect.DeepEqual(listing.names, test.names) {
				t.Fatalf("split=%+v ok=%v", listing, ok)
			}
		})
	}
}

func TestSplitBuiltinListingRefusesMalformedSections(t *testing.T) {
	live := "Live Plugins (loaded from a local marketplace directory, never copied):\n  • demo@agentplugins-0123456789ab (v1.0.0) (enabled)\n      from /managed/demo\n"
	entries := make([]string, maxBuiltinListingEntries+1)
	for index := range entries {
		entries[index] = "  • builtin-" + strings.Repeat("x", index+1) + " (v1.0.0) [disabled]\n"
	}
	for name, document := range map[string]string{
		"no section":          copilotEmptyListingBody,
		"empty section":       copilotEmptyListingBody + "Built-in plugins:\n",
		"two headers":         copilot103BuiltinSection + copilot112BuiltinSection + live,
		"middle section":      live + copilot112BuiltinSection + live,
		"missing state":       strings.Replace(copilot112BuiltinSection, " [disabled]", "", 1) + live,
		"unknown state":       strings.Replace(copilot112BuiltinSection, "[disabled]", "[pending]", 1) + live,
		"missing version":     strings.Replace(copilot112BuiltinSection, " (v0.1.97)", "", 1) + live,
		"marketplace builtin": strings.Replace(copilot112BuiltinSection, "computer-use", "computer-use@market", 1) + live,
		"duplicate builtin":   copilot112BuiltinSection + "  • Browser-Use (v0.1.1) [disabled]\n" + live,
		"indented header":     " " + copilot112BuiltinSection + live,
		"crlf":                strings.ReplaceAll(copilot112BuiltinSection+live, "\n", "\r\n"),
		"ansi":                "\x1b[32m" + copilot112BuiltinSection + live,
		"missing lf":          strings.TrimSuffix(copilotEmptyListingBody+copilot112BuiltinSection, "\n"),
		"trailing blank":      copilotEmptyListingBody + copilot112BuiltinSection + "\n",
		"oversized":           "Built-in plugins:\n" + strings.Join(entries, "") + live,
	} {
		t.Run(name, func(t *testing.T) {
			if listing, ok := splitBuiltinListing(document); ok {
				t.Fatalf("malformed builtin section accepted: %+v", listing)
			}
		})
	}
}
