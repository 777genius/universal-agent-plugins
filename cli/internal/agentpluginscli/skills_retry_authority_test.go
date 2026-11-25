package agentpluginscli

import (
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A fresh HTTPS retry can acquire changed bytes only after the existing second
// review. Retained installation identity continues to refuse a changed release.
func TestSkillsWebsiteRetryRetainsReviewAndRevisionAuthority(t *testing.T) {
	for _, retained := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh reviewed source", true: "retained installation"}[retained], func(t *testing.T) {
			_, journey, source, changeContent := managementMutableWebsite(t)
			app := journey.cli.app
			request := SkillsBatchRequest{Selections: []SkillsSelection{{Website: source, Name: "pdf"}}, TargetIDs: []string{"codex"}}
			if retained {
				seed, err := app.PrepareSkills(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := seed.Apply(t.Context()); err != nil {
					seed.Close()
					t.Fatal(err)
				}
				seed.Close()
				journey.run(t, "remove", "pdf", "--target", "codex", "--external-uninstalled", "--scope", "user", "--format", "json")
				if len(journey.state(t).EnvironmentReceipts) != 0 {
					t.Fatal("last-binding removal did not complete")
				}
			}
			plan, err := app.PrepareSkills(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			original, ok := plan.request.lock.Package("pdf")
			if !ok {
				t.Fatal("original pin missing")
			}
			changeContent("---\nname: pdf\ndescription: Read PDFs\n---\nChanged after the reviewed attempt failed.\n")
			confirmations := 0
			app.Terminal = true
			app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				confirmations++
				return prompt.ConfirmationResult{Accepted: true}, nil
			}}
			cmd, _, _ := retryTestCommand(t)
			next, err := prepareSkillsRetry(cmd, app, &options{format: "human", scope: "user"}, plan, plan.Actions(), reportByAgent([]agentItem{failedItem("pdf", "codex", errors.New("prepublication failure"))}))
			if retained {
				if next != nil || !errors.Is(err, domain.ErrRetainedRevisionChange) || confirmations != 1 {
					t.Fatalf("retained revision change escaped refusal: next=%v confirmations=%d err=%v", next != nil, confirmations, err)
				}
				return
			}
			if err != nil || next == nil {
				t.Fatalf("fresh retry missing: %v", err)
			}
			defer next.Close()
			retried, ok := next.request.lock.Package("pdf")
			if !ok || original.TreeDigest == retried.TreeDigest || confirmations != 2 {
				t.Fatalf("changed fresh retry did not get additional review: old=%s new=%s confirmations=%d", original.TreeDigest, retried.TreeDigest, confirmations)
			}
		})
	}
}
