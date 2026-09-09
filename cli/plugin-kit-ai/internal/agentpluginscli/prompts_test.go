package agentpluginscli

import (
	"context"
	"errors"
	"github.com/spf13/cobra"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type fakePrompter struct {
	selectFn  func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error)
	confirmFn func() (prompt.ConfirmationResult, error)
}

func (f fakePrompter) SelectTargets(_ context.Context, r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
	return f.selectFn(r)
}
func (f fakePrompter) Confirm(_ context.Context, r prompt.ConfirmationRequest) (prompt.ConfirmationResult, error) {
	return f.confirmFn()
}
func TestNoTargetConsentBoundary(t *testing.T) {
	for _, input := range []string{"\n", "\nn\n", "\ny", "\ny\n"} {
		t.Run(strings.ReplaceAll(input, "\n", "ENTER"), func(t *testing.T) {
			f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex)})
			out, _, err := f.executeInput(true, input, "add", writeCLIPlugin(t))
			yes := input == "\ny\n"
			state, _ := f.store.Load()
			if (len(state.Installations) > 0) != yes {
				t.Fatalf("mutation before consent: %v %v", state, err)
			}
			if !strings.Contains(out, "Scope: user") || strings.Index(out, "Result:") > strings.Index(out, "Apply this plan?") {
				t.Fatal(out)
			}
			if (input == "\n" || input == "\ny") && !errors.Is(err, prompt.ErrPromptInputClosed) {
				t.Fatal(err)
			}
		})
	}
}
func TestJSONNoTargetNeverPrompts(t *testing.T) {
	f := newCLIFixture(t, nil)
	f.app.PromptFactory = nil
	f.app.Prompter = fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		t.Fatal("JSON prompt")
		return prompt.TargetSelectionResult{}, nil
	}}
	out, _, err := f.executeInput(true, "y\n", "add", writeCLIPlugin(t), "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "requires --target") || out != "" {
		t.Fatal(out, err)
	}
}
func TestInvalidInjectedSelectionCannotApply(t *testing.T) {
	f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex)})
	f.app.Prompter = fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		return prompt.TargetSelectionResult{IDs: []domain.ClientID{"unknown"}}, nil
	}, confirmFn: func() (prompt.ConfirmationResult, error) {
		t.Fatal("confirmed invalid selection")
		return prompt.ConfirmationResult{}, nil
	}}
	_, _, err := f.execute(true, "add", writeCLIPlugin(t))
	if err == nil {
		t.Fatal("invalid selection accepted")
	}
	state, _ := f.store.Load()
	if len(state.Installations) != 0 {
		t.Fatal(state)
	}
}

func TestSingleDetectedTargetStillRequiresInstallConsent(t *testing.T) {
	f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	confirms := 0
	f.app.Prompter = fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		t.Fatal("single target should auto-select")
		return prompt.TargetSelectionResult{}, nil
	}, confirmFn: func() (prompt.ConfirmationResult, error) {
		confirms++
		state, _ := f.store.Load()
		if len(state.Installations) != 0 {
			t.Fatal("applied before confirm")
		}
		return prompt.ConfirmationResult{Accepted: false}, nil
	}}
	out, _, err := f.execute(true, "add", writeCLIPlugin(t))
	if err != nil || confirms != 1 || !strings.Contains(out, "Target: cursor") || !strings.Contains(out, "Installation not applied") {
		t.Fatal(out, err, confirms)
	}
}
func TestCancelAfterInjectedConsentCannotApply(t *testing.T) {
	f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		cancel()
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}
	f.app.Terminal = true
	command := NewRoot(f.app)
	command.SetArgs([]string{"add", writeCLIPlugin(t)})
	if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	state, _ := f.store.Load()
	if len(state.Installations) != 0 {
		t.Fatal("applied canceled consent")
	}
}

type mustNotRead struct{ t *testing.T }

func (r mustNotRead) Read([]byte) (int, error) {
	r.t.Fatal("read before visible output succeeded")
	return 0, io.EOF
}

type shortPromptWriter struct{}

func (shortPromptWriter) Write([]byte) (int, error) { return 0, nil }
func TestLegacyQuestionWriteFailureAbortsBeforeRead(t *testing.T) {
	accepted, err := promptYesNo(context.Background(), mustNotRead{t}, shortPromptWriter{}, io.Discard, "Activation? [y/N]")
	if accepted || !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(accepted, err)
	}
}
func TestSecurityFindingWriteFailureAbortsBeforeRead(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetIn(mustNotRead{t})
	cmd.SetOut(shortPromptWriter{})
	cmd.SetErr(io.Discard)
	loaded := &loadedPackage{security: &domain.SecurityAssessment{Counts: domain.SecurityCounts{Blocking: 1}}}
	err := authorizeSecurityAssessment(cmd, App{Terminal: true}, &options{format: "human"}, loaded)
	if !errors.Is(err, io.ErrShortWrite) || loaded.securityAuthorized {
		t.Fatal(err, loaded.securityAuthorized)
	}
}

func TestDynamicValuesCannotForgePlanOrSecurityRows(t *testing.T) {
	payload := "server\nTarget: forged\n" + strings.Repeat("x\n", 1000)
	var out strings.Builder
	result := usecase.AddResult{Plan: domain.DeliveryPlan{ClientID: domain.ClientCursor, Components: []domain.ComponentDecision{{Kind: "mcp", Name: payload}}}}
	if err := renderHumanPlan(&out, domain.PackageEnvelope{}, result); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "\nTarget:") != 1 || strings.Count(out.String(), "\n") != 7 || len(out.String()) > 800 {
		t.Fatal(out.String())
	}
	out.Reset()
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	renderSecurityAssessment(cmd, domain.SecurityAssessment{Counts: domain.SecurityCounts{Blocking: 1}, Findings: []domain.SecurityFinding{{Code: payload, Path: payload, Message: payload, Disposition: "blocking"}}}, true)
	if strings.Contains(out.String(), "\nTarget:") || strings.Count(out.String(), "\n") != 3 || len(out.String()) > 1800 {
		t.Fatal(out.String())
	}
}
