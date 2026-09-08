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
