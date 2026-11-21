package prompt

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSelectionContract(t *testing.T) {
	r := TargetSelectionRequest{Choices: []TargetChoice{{ID: "cursor"}, {ID: "codex"}}, DefaultIDs: []domain.ClientID{"cursor", "codex"}}
	for _, tc := range []struct {
		name string
		ids  []domain.ClientID
		bad  bool
	}{
		{"empty", nil, true}, {"duplicate", []domain.ClientID{"cursor", "cursor"}, true}, {"unknown", []domain.ClientID{"claude"}, true}, {"subset", []domain.ClientID{"codex"}, false}, {"reordered", []domain.ClientID{"codex", "cursor"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateSelection(r, tc.ids)
			if (err != nil) != tc.bad {
				t.Fatal(got, err)
			}
			if err != nil && got.IDs != nil {
				t.Fatal("nonzero error result")
			}
			if tc.name == "reordered" && !reflect.DeepEqual(got.IDs, r.DefaultIDs) {
				t.Fatal(got)
			}
		})
	}
	r.Choices = append(r.Choices, r.Choices[0])
	if ValidateRequest(r) == nil {
		t.Fatal("duplicate request accepted")
	}
}
func TestSafeText(t *testing.T) {
	if got := SafeText("a\x1b\x00\u202eb\nc"); got != "abc" {
		t.Fatal(got)
	}
}

func TestSkippedClientsNoticeSanitizesDeduplicatesAndGroups(t *testing.T) {
	got := SkippedClientsNotice([]SkippedTarget{
		{ID: "chatgpt", Label: " ChatGPT\x1b\u202e ", Kind: SkipNotSaveable},
		{ID: "chatgpt", Label: "ChatGPT", Kind: SkipNotSaveable},
		{ID: "vscode", Label: "Visual Studio Code", Kind: SkipNotSaveable},
		{ID: "hermes", Label: "Hermes Agent", Kind: SkipNotDetected},
		{ID: "antigravity", Label: "Google Antigravity", Kind: SkipNotDetected},
		{ID: "pi", Label: "Pi", Target: "pi", Kind: SkipNoRelease},
		{ID: "kiro", Label: "Kiro", Kind: SkipManual, Reason: "connect the server in Kiro yourself\x1b"},
		{ID: "empty", Label: " ", Kind: SkipNotDetected},
	})
	// Specific problems first, missing clients last; one line per shared reason.
	want := "Not available for automatic install:\n" +
		"  Kiro: connect the server in Kiro yourself\n" +
		"  ChatGPT, Visual Studio Code: cannot be recorded in the dependency list; install it separately with --no-save\n" +
		"  Pi: no compatible release of this package; check one with --target pi\n" +
		"  Hermes Agent, Google Antigravity: not found on this computer; install or configure it first\n"
	if got != want {
		t.Fatalf("notice = %q, want %q", got, want)
	}
	if SkippedClientsNotice([]SkippedTarget{{ID: "x", Label: "X", Kind: SkipOther}}) != "" {
		t.Fatal("a skipped client without any reason must not render an empty explanation")
	}
}

func TestGroupSkippedUsesGenericTargetForSeveralClients(t *testing.T) {
	groups := GroupSkipped([]SkippedTarget{
		{ID: "pi", Label: "Pi", Kind: SkipNoRelease},
		{ID: "cline", Label: "Cline", Kind: SkipNoRelease},
	})
	if len(groups) != 1 || len(groups[0].Names) != 2 || groups[0].Detail != "no compatible release of this package; check one with --target <name>" {
		t.Fatalf("groups = %+v", groups)
	}
}
