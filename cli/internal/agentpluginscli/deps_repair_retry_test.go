package agentpluginscli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestRepairRetryDeclinedReviewKeepsOriginalFailure(t *testing.T) {
	for _, retry := range []bool{false, true} {
		name := "first attempt"
		if retry {
			name = "retry"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newCLIFixture(t, nil)
			app := fixture.app
			app.Terminal = true
			asked := 0
			app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				asked++
				return prompt.ConfirmationResult{Accepted: true}, nil
			}}
			cmd, out, _ := retryTestCommand(t)
			cmd.SetIn(strings.NewReader("n\n"))
			original := errors.New("native repair failed")
			opts := &options{format: "human"}
			reviewed := 0
			run := func() error {
				reviewed++
				opts.installResult = nil
				return applyDependencyRepairs(cmd, app, opts, nil, nil)
			}
			var err error
			if retry {
				opts.installResult = &installOutcome{Agents: reportByAgent([]agentItem{failedItem("demo", "codex", original)})}
				err = retryConverge(cmd, app, opts, original, run)
				if !errors.Is(err, original) || !errors.Is(err, installer.ErrCancelled) || asked != 1 || opts.retryAttempt != 1 {
					t.Fatalf("declined repair retry lost its failure: err=%v asked=%d attempt=%d", err, asked, opts.retryAttempt)
				}
			} else if err = run(); err != nil || asked != 0 || opts.retryAttempt != 0 {
				t.Fatalf("initial repair decline changed: err=%v asked=%d attempt=%d", err, asked, opts.retryAttempt)
			}
			if reviewed != 1 || !strings.Contains(out.String(), "Repair the managed package") || opts.installResult != nil {
				t.Fatalf("decline did not stop before apply bookkeeping: reviewed=%d result=%+v output=%q", reviewed, opts.installResult, out.String())
			}
			if _, err := os.Stat(fixture.store.Path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("declined repair wrote state: %v", err)
			}
		})
	}
}
