package agentpluginscli

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func FuzzParseTargetOption(f *testing.F) {
	for _, seed := range []string{
		"", "cursor", "cursor,claude", " CURSOR , claude ",
		"all", "openai", "cursor,cursor", ",cursor", "legacy-all",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 1<<16 {
			t.Skip()
		}
		targets, err := parseTargetOption(value)
		if err != nil || len(targets) == 0 {
			return
		}
		seen := make(map[domain.ClientID]struct{}, len(targets))
		for _, target := range targets {
			if target != "legacy-all" && !domain.IsSupportedClient(target) {
				t.Fatalf("parser returned unsupported target %q", target)
			}
			if _, duplicate := seen[target]; duplicate {
				t.Fatalf("parser returned duplicate target %q", target)
			}
			seen[target] = struct{}{}
		}
		joined := make([]string, len(targets))
		for index, target := range targets {
			joined[index] = string(target)
		}
		roundTrip, roundTripErr := parseTargetOption(strings.Join(joined, ","))
		if roundTripErr != nil || !reflect.DeepEqual(roundTrip, targets) {
			t.Fatalf("canonical targets did not round-trip: %v, %v", roundTrip, roundTripErr)
		}
	})
}

func FuzzSafeDiagnosticText(f *testing.F) {
	for _, seed := range []string{
		"plain error", "line one\nline two", "\x1b[31mred\x1b[0m",
		"unterminated \x1b[31", "control\x00 bidi\u202e",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 1<<20 {
			t.Skip()
		}
		got := safeDiagnosticText(value)
		if again := safeDiagnosticText(got); again != got {
			t.Fatalf("sanitizer is not idempotent: %q != %q", again, got)
		}
		if strings.ContainsRune(got, '\x1b') {
			t.Fatalf("sanitizer retained ESC: %q", got)
		}
		for _, r := range got {
			if r != '\n' && (unicode.IsControl(r) || unicode.In(r, unicode.Cf)) {
				t.Fatalf("sanitizer retained control rune %U", r)
			}
		}
	})
}

func FuzzSourceClassification(f *testing.F) {
	for _, seed := range []string{
		"playwright", "owner/plugin", "./plugin", "../plugin", "/tmp/plugin",
		`C:\\plugin`, `C:plugin`, `\\\\server\\share\\plugin`, "https://example.com/plugin",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 1<<16 {
			t.Skip()
		}
		if isDirectorySelector(value) && explicitLocalPath(value) {
			t.Fatalf("source is both Directory selector and explicit local path: %q", value)
		}
	})
}
