package agentpluginscli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestRemoveJSONRejectsMissingTargetBeforeInput(t *testing.T) {
	for _, terminal := range []bool{true, false} {
		for _, dryRun := range []bool{false, true} {
			f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex)})
			if out, _, err := f.execute(false, "add", writeCLIPlugin(t), "--target=cursor,codex"); err != nil {
				t.Fatalf("fixture: %s %v", out, err)
			}
			before, err := f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			app := f.app
			app.Input, app.Output, app.ErrorOutput, app.Terminal = mustNotRead{t}, &out, &bytes.Buffer{}, terminal
			cmd := NewRoot(app)
			args := []string{"remove", before.Installations[0].InstallationID, "--format=json"}
			if dryRun {
				args = append(args, "--dry-run")
			}
			cmd.SetArgs(args)
			err = cmd.ExecuteContext(context.Background())
			if err == nil || !strings.Contains(err.Error(), "requires --target") || out.Len() != 0 {
				t.Fatalf("terminal=%v dryRun=%v stdout=%q err=%v", terminal, dryRun, out.String(), err)
			}
		}
	}
}

func TestProgressFinishOnlyMarksObservedStages(t *testing.T) {
	for _, tc := range []struct {
		name   string
		phases []usecase.GroupProgressPhase
		greens int
	}{
		{name: "untouched"}, {name: "preparation failed", phases: []usecase.GroupProgressPhase{usecase.GroupProgressPreparing}},
		{name: "configuration failed", phases: []usecase.GroupProgressPhase{usecase.GroupProgressPreparing, usecase.GroupProgressConfiguring}, greens: 1},
		{name: "activation failed", phases: []usecase.GroupProgressPhase{usecase.GroupProgressPreparing, usecase.GroupProgressConfiguring, usecase.GroupProgressConfigured}, greens: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			board := newGroupProgressBoard(&bytes.Buffer{}, []domain.DetectedClient{{ClientID: domain.ClientCursor}})
			for _, phase := range tc.phases {
				board.observe(usecase.GroupProgressEvent{ClientID: domain.ClientCursor, Phase: phase})
			}
			board.finish(nil)
			got := progressPipeline(terminaltheme.Theme{Enabled: true}, board.rows[0])
			if strings.Count(got, "✓") != tc.greens {
				t.Fatalf("unobserved success: %q", got)
			}
		})
	}
}

func TestProgressOutcomeAgreesWithSummary(t *testing.T) {
	for _, auth := range []domain.AuthenticationState{domain.AuthenticationPending, domain.AuthenticationNotChecked, domain.AuthenticationFailed, domain.AuthenticationComplete, domain.AuthenticationNotRequired} {
		result := usecase.AddResult{GroupPhase: usecase.GroupTargetExternalCompleted, Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Authentication: auth}}
		want := strings.ToLower(string(classifyBatchPresentation(addTargetResult{Output: addResultData{Result: result}})))
		if got := progressResultLabel(result); got != want {
			t.Errorf("auth=%s board=%q summary=%q", auth, got, want)
		}
	}
	for _, phase := range []usecase.GroupTargetPhase{"", usecase.GroupTargetPlanned, usecase.GroupTargetManagedCommitted, usecase.GroupTargetExternalNotAttempted, usecase.GroupTargetExternalPartial, usecase.GroupTargetManagedRolledBack} {
		result := usecase.AddResult{GroupPhase: phase}
		want := strings.ToLower(string(classifyBatchPresentation(addTargetResult{Output: addResultData{Result: result}})))
		if got := progressResultLabel(result); got != want {
			t.Errorf("phase=%s board=%q summary=%q", phase, got, want)
		}
	}
}

func TestAttestedProgressIsNotObservedInstallation(t *testing.T) {
	result := usecase.AddResult{GroupPhase: usecase.GroupTargetExternalCompleted, Activation: domain.ActivationOutcome{
		Activation: domain.ActivationActive, Authentication: domain.AuthenticationComplete, Verification: domain.VerificationInstalled, ActivationAttested: true,
	}}
	if got := progressResultLabel(result); got != "user-attested" {
		t.Fatalf("attestation labeled %q", got)
	}
	target := addTargetResult{Output: addResultData{Result: result}}
	if got := string(classifyBatchPresentation(target)); got != "User-attested" {
		t.Fatalf("summary labeled %q", got)
	}
}
