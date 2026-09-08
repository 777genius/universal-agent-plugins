package terminalprompts

import (
	"context"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Real forms consume keyboard bytes through public adapter methods. Only
// presentation-specific input differs; expectations are shared literal values,
// never computed by the production validator or a substitute form runner.
func TestAdapterSuccessContract(t *testing.T) {
	for _, adapter := range []string{"plain", "huh"} {
		t.Run(adapter, func(t *testing.T) {
			for _, tc := range []struct {
				name, plain, huh  string
				confirm, accepted bool
				defaults, want    []domain.ClientID
			}{
				{name: "default-no", plain: "\n", huh: "\r", confirm: true},
				{name: "explicit-no", plain: "n\n", huh: "  \r", confirm: true},
				{name: "explicit-yes", plain: "yes\n", huh: " \r", confirm: true, accepted: true},
				{name: "all-defaults", plain: "\n", huh: "\r", defaults: []domain.ClientID{"cursor", "claude", "codex"}, want: []domain.ClientID{"cursor", "claude", "codex"}},
				{name: "subset-defaults-canonical-order", plain: "\n", huh: "\r", defaults: []domain.ClientID{"codex", "cursor"}, want: []domain.ClientID{"cursor", "codex"}},
				{name: "changed-subset", plain: "3,1\n", huh: "\x1b[B \r", defaults: []domain.ClientID{"cursor", "claude", "codex"}, want: []domain.ClientID{"cursor", "codex"}},
				{name: "single-selection", plain: "2\n", huh: " \x1b[B\x1b[B \r", defaults: []domain.ClientID{"cursor", "claude", "codex"}, want: []domain.ClientID{"claude"}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					keys := tc.plain
					if adapter == "huh" {
						keys = tc.huh
					}
					input := strings.NewReader(keys + "next-answer\n")
					var output strings.Builder
					var p prompt.Prompter = PlainPrompter{Input: input, Output: &output}
					if adapter == "huh" {
						p = HuhPrompter{Input: input, Output: &output, NoColor: true}
					}
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					if tc.confirm {
						req := prompt.ConfirmationRequest{Title: "Apply fixture?", Summary: []string{"Disposable fixture only"}}
						got, err := p.Confirm(ctx, req)
						if err != nil || got.Accepted != tc.accepted {
							t.Fatalf("confirmation = %+v, %v; want accepted=%v", got, err, tc.accepted)
						}
						if !reflect.DeepEqual(req.Summary, []string{"Disposable fixture only"}) {
							t.Fatal("confirmation request mutated")
						}
					} else {
						req := prompt.TargetSelectionRequest{Choices: []prompt.TargetChoice{{ID: "cursor", Label: "Cursor"}, {ID: "claude", Label: "Claude"}, {ID: "codex", Label: "Codex"}}, DefaultIDs: append([]domain.ClientID(nil), tc.defaults...), SkippedLabels: []string{"Unsupported fixture"}}
						before := prompt.TargetSelectionRequest{Choices: append([]prompt.TargetChoice(nil), req.Choices...), DefaultIDs: append([]domain.ClientID(nil), req.DefaultIDs...), SkippedLabels: append([]string(nil), req.SkippedLabels...)}
						got, err := p.SelectTargets(ctx, req)
						if err != nil || !reflect.DeepEqual(got.IDs, tc.want) {
							t.Fatalf("selection = %+v, %v; want %v", got, err, tc.want)
						}
						if !reflect.DeepEqual(req, before) {
							t.Fatal("selection request mutated")
						}
						got.IDs[0] = "changed-result"
						if !reflect.DeepEqual(req, before) {
							t.Fatal("result aliases request")
						}
					}
					if output.Len() == 0 {
						t.Fatal("success without visible question")
					}
					rest, err := io.ReadAll(input)
					if err != nil || string(rest) != "next-answer\n" {
						t.Fatalf("queued answer consumed: %q, %v", rest, err)
					}
				})
			}
		})
	}
}
